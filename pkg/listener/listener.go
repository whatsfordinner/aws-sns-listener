package listener

import (
	"context"
	"log"
	"time"
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
}

func ListenToTopic(ctx context.Context, sqsClient SQSAPI, snsClient SNSAPI, ssmClient SSMAPI, consumer Consumer, cfg ListenerConfiguration) error {
	if cfg.PollingInterval == 0 {
		log.Printf("No polling interval set, defaulting to 1s")
		cfg.PollingInterval = time.Second
	}

	if cfg.ParameterPath != "" {
		topicArn, err := getParameter(ctx, ssmClient, cfg.ParameterPath)

		if err != nil {
			return err
		}

		cfg.TopicArn = *topicArn
	}

	queueUrl, err := createQueue(ctx, sqsClient, cfg.QueueName, cfg.TopicArn)

	if err != nil {
		return err
	}

	defer deleteQueue(context.Background(), sqsClient, queueUrl)

	queueArn, err := getQueueArn(ctx, sqsClient, queueUrl)

	if err != nil {
		return err
	}

	subscriptionArn, err := subscribeToTopic(ctx, snsClient, &cfg.TopicArn, queueArn)

	if err != nil {
		return err
	}

	defer unsubscribeFromTopic(context.Background(), snsClient, subscriptionArn)

	listenToQueue(ctx, sqsClient, queueUrl, consumer, cfg.PollingInterval)

	return nil
}
