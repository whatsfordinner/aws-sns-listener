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

type Listener struct {
	PollingInterval time.Duration
	QueueName       string
	TopicArn        string
	Verbose         bool
	SnsClient       SNSAPI
	SqsClient       SQSAPI

	queueUrl        string
	subscriptionArn string
}

type Option func(l *Listener)

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

func WithQueueName(queueName string) Option {
	return func(l *Listener) {
		l.QueueName = queueName
	}
}

func WithVerbose(verbose bool) Option {
	return func(l *Listener) {
		l.Verbose = verbose
	}
}

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

func (l *Listener) Listen(ctx context.Context, c Consumer) error {
	ctx, span := otel.Tracer(name).Start(ctx, "Listener.Listen")
	defer span.End()

	err := listenToQueue(
		ctx,
		l.SqsClient,
		l.queueUrl,
		c,
		l.PollingInterval,
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

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
