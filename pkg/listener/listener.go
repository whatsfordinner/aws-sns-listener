// Package listener provides a mechanism for listening to events published to an
// Amazon Simple Notification Service topic.
//
// The listener package supports regular topics and first-in first-out (FIFO) topics
// as well as deriving the topic ARN from a Systems Manager (SSM) parameter. It works
// by creating an Amazon Simple Queue Service (SQS) queue, authorizing the SNS topic to
// publish messages to that queue, registering the queue as a subscriber to the topic
// and then receiving messages from the queue on a loop.
//
// The package does not instantiate any AWS clients itself and doesn't have any opinions
// on where the SQS queue is created relative to the SNS topic.
package listener

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

const name string = "github.com/whatsfordinner/aws-sns-listener/pkg/listener"
const traceNamespace string = "aws-sns-listener"

var logger *log.Logger = log.New(
	io.Discard,
	log.Default().Prefix(),
	log.Default().Flags(),
)

// A Consumer is used by ListenToTopic to process messages and errors during the course of oeprations. Both of its methods are provided
// with the existing context used by the package so that any implementing type is able to be propagate traces or be made aware of
// context cancellations.
type Consumer interface {
	// OnMessage is called when a message is successfully processed from the SQS queue. If no messages are processed then OnMessage won't
	// be called.
	OnMessage(ctx context.Context, msg MessageContent)

	// OnError is called when there is an error attempting to receive messages from the SQS queue or processing a given message from the
	// queue.
	OnError(ctx context.Context, err error)
}

// A MessageContent maps the message body and message ID of a SQS message to
// a much more straightforward struct. For the purpose of listening to an SNS
// topic, the Body contains the full message that was published
type MessageContent struct {
	Body *string
	Id   *string
}

// A ListenerConfiguration is used to set up the listener package when ListenToTopic
// is called.
type ListenerConfiguration struct {
	// Polling Interval is the time between attempts to receive messages from the SQS queue. Defaults to 1 second.
	PollingInterval time.Duration
	// QueueName is the name of the SQS queue to create. If the SNS topic is FIFO then the listener package will automatically
	// add the ".fifo" suffix. Defaults to a v4 UUID prefixed with "sns-listener-".
	QueueName string
	// ParameterPath is the location of the SSM parameter that contains the topic ARN. If this is unset then the listener package
	// will use the value of TopicArn.
	ParameterPath string
	// TopicArn is the full ARN of the topic to subscribe to. If this is unset and ParameterPath is unset then startup will fail.
	TopicArn string
	// Verbose controls whether logs will be output to stderr or silenced. Defaults to false.
	Verbose bool
}

// ListenToTopic is the entrypoint to the package. In the course of normal operations, ListenToTopic will create a new SQS queue, subscribe it to
// the SNS topic and receive messages from the queue until the provided context has been cancelled. When the context has been cancelled it will
// destroy the subscription and queue, returning any errors it encountered while doing so. Therefore it is recommended to use a channel to wait
// for the function to finish inside of the goroutine that is spawned.
func ListenToTopic(ctx context.Context, sqsClient SQSAPI, snsClient SNSAPI, ssmClient SSMAPI, consumer Consumer, cfg ListenerConfiguration) error {
	ctx, span := otel.Tracer(name).Start(ctx, "Startup")

	if cfg.Verbose {
		logger.SetOutput(os.Stderr)
	}

	if cfg.PollingInterval == 0 {
		cfg.PollingInterval = time.Second
	}

	if cfg.ParameterPath != "" {
		topicArn, err := getParameter(ctx, ssmClient, cfg.ParameterPath)

		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return err
		}

		cfg.TopicArn = topicArn
	}

	queueUrl, err := createQueue(ctx, sqsClient, cfg.QueueName, cfg.TopicArn)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return err
	}

	defer deleteQueue(context.Background(), sqsClient, queueUrl)

	queueArn, err := getQueueArn(ctx, sqsClient, queueUrl)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return err
	}

	subscriptionArn, err := subscribeToTopic(ctx, snsClient, cfg.TopicArn, queueArn)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return err
	}

	defer unsubscribeFromTopic(context.Background(), snsClient, subscriptionArn)

	span.SetStatus(codes.Ok, "Initialised successfully")
	span.End()

	listenToQueue(ctx, sqsClient, queueUrl, consumer, cfg.PollingInterval)

	return nil
}
