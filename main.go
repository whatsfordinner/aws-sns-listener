package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type app struct {
	snsClient SNSAPI
	sqsClient SQSAPI
	topicARN  *string
	queueURL  *string
}

func (a *app) toString() string {
	return fmt.Sprintf("SQS queue URL: %s\nSNS topic ARN: %s", *a.queueURL, *a.topicARN)
}

func (a *app) run(ctx context.Context) error {
	defer a.teardown(ctx)
	err := a.init(ctx)

	if err != nil {
		return err
	}

	a.listen()
	return nil
}

func (a *app) init(ctx context.Context) error {
	queueName := "making-a-queue"
	createQueueResult, err := a.sqsClient.CreateQueue(
		ctx,
		&sqs.CreateQueueInput{
			QueueName: &queueName,
			Attributes: map[string]string{
				"DelaySeconds":           "60",
				"MessageRetentionPeriod": "86400",
			},
		},
	)

	if err != nil {
		return err
	}

	a.queueURL = createQueueResult.QueueUrl

	return nil
}

func (a *app) listen() {
	fmt.Println(a.toString())
}

func (a *app) teardown(ctx context.Context) error {
	_, err := a.sqsClient.DeleteQueue(
		ctx,
		&sqs.DeleteQueueInput{
			QueueUrl: a.queueURL,
		},
	)

	if err != nil {
		return err
	}

	a.queueURL = new(string)

	return nil
}

type SNSAPI interface {
	Subscribe(ctx context.Context,
		params *sns.SubscribeInput,
		optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)

	Unsubscribe(ctx context.Context,
		params *sns.UnsubscribeInput,
		optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
}

type SQSAPI interface {
	CreateQueue(ctx context.Context,
		params *sqs.CreateQueueInput,
		optFns ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error)

	DeleteQueue(ctx context.Context,
		params *sqs.DeleteQueueInput,
		optFns ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error)

	ReceiveMessage(ctx context.Context,
		params *sqs.ReceiveMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
}

func main() {
	topicARN := flag.String("t", "", "The ARN of the topic to listen to")

	flag.Parse()

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	a := new(app)
	a.sqsClient = sqs.NewFromConfig(cfg)
	a.snsClient = sns.NewFromConfig(cfg)
	a.topicARN = topicARN
	a.queueURL = new(string)

	err = a.run(context.TODO())

	if err != nil {
		panic("runtime error, " + err.Error())
	}

	fmt.Println(a.toString())
}
