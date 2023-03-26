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

type Consumer interface {
	OnMessage(ctx context.Context, msg MessageContent)

	OnError(ctx context.Context, err error)
}

type MessageContent struct {
	Body *string
	Id   *string
}

type ListenerConfiguration struct {
	PollingInterval time.Duration
	QueueName       string
	ParameterPath   string
	TopicArn        string
	Verbose         bool
}

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

		cfg.TopicArn = *topicArn
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

	subscriptionArn, err := subscribeToTopic(ctx, snsClient, &cfg.TopicArn, queueArn)

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
