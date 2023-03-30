// Package listener provides a way of listening to messages published to an AWS SNS Topic.
package listener

import (
	"context"
	"errors"
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

// A Listener manages the resources for listening to a queue.
// It should not be instantiated directly, instead the New() function should be used.
type Listener struct {
	// PollingInterval is the time between attempts to receive messages from the SQS queue
	PollingInterval time.Duration
	// QueueName is the desired name for the SQS queue. If blank a v4 UUID prefixed with "sns-listener" will be used
	QueueName string
	// TopicArn is the ARN of the SNS topic to be listened to.
	TopicArn string
	// Verbose will enable logging to stderr when true, otherwise logs are discarded
	Verbose bool
	// SnsClient is a user-provided client used to interact with the SNS API
	SnsClient SNSAPI
	// SqsClient is a user-provided client used to interact with the SQS API
	SqsClient SQSAPI

	queueUrl        string
	subscriptionArn string
}

// An Option allows for the passing of optional parameters when creating a new Listener.
type Option func(l *Listener)

// WithPollingInterval will set PollingInterval to the provided time.
// Defaults to 1 second if set to 0.
func WithPollingInterval(pollingInterval time.Duration) Option {
	return func(l *Listener) {
		if pollingInterval <= 0 {
			log.Printf("Provided polling interval invalid: %s. Defaulting to 1 second", pollingInterval)
			l.PollingInterval = time.Second
		} else {
			l.PollingInterval = pollingInterval
		}
	}
}

// WithQueueName will control the name of the SQS queue created by the Listener.
// When listening to a FIFO topic, the Listener will add ".fifo" to the queue itself.
func WithQueueName(queueName string) Option {
	return func(l *Listener) {
		l.QueueName = queueName
	}
}

// WithVerbose controls whether or not logs will be printed to stderr.
func WithVerbose(verbose bool) Option {
	return func(l *Listener) {
		l.Verbose = verbose
	}
}

// New creates a new Listener and returns a pointer to it.
func New(topicArn string, snsClient SNSAPI, sqsClient SQSAPI, opts ...Option) *Listener {
	l := new(Listener)

	l.TopicArn = topicArn
	l.SnsClient = snsClient
	l.SqsClient = sqsClient

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Setup will create an SQS queue and subscribe it to that queue.
// The queue is given a policy that allows the SNS topic to subscribe to it.
// Once the queue is subscribed to the SNS topic it will start receiving published messages.
func (l *Listener) Setup(ctx context.Context) error {
	ctx, span := otel.Tracer(name).Start(ctx, "Listener.Setup")
	defer span.End()

	if l.Verbose {
		logger.SetOutput(os.Stderr)
	}

	queueUrl, err := createQueue(ctx, l.SqsClient, l.QueueName, l.TopicArn)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	l.queueUrl = queueUrl

	queueArn, err := getQueueArn(ctx, l.SqsClient, queueUrl)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	subscriptionArn, err := subscribeToTopic(ctx, l.SnsClient, l.TopicArn, queueArn)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	l.subscriptionArn = subscriptionArn

	return nil
}

// Listen is a blocking function that processes messages from the SQS queue as they arrive.
// Listen will block until the context provided to it is cancelled.
// Messages will be passed to the provided Consumer's OnMessage method then deleted from the queue.
// Do not pass the same context as provided to Teardown otherwise resources will not be destroyed.
func (l *Listener) Listen(ctx context.Context, c Consumer) error {
	// This function deliberately doesn't create a span because it's a shim around listenToQueue.
	// listenToQueue is a blocking function so any span created here will last the life of the method call.

	err := listenToQueue(
		ctx,
		l.SqsClient,
		l.queueUrl,
		c,
		l.PollingInterval,
	)

	if err != nil {
		return err
	}

	return nil
}

// Teardown unsubscribes the queue from the topic and then deletes the queue.
// It will attempt to do both regardless of the existing state of the infrastructure.
func (l *Listener) Teardown(ctx context.Context) error {
	ctx, span := otel.Tracer(name).Start(ctx, "Listener.Teardown")
	defer span.End()

	err := errors.Join(
		unsubscribeFromTopic(ctx, l.SnsClient, l.subscriptionArn),
		deleteQueue(ctx, l.SqsClient, l.queueUrl),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
