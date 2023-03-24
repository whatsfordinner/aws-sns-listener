package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type consumer struct{}

func (c consumer) OnMessage(ctx context.Context, m MessageContent) {
	fmt.Printf("Message body: %s\n", *m.Body)
}

func (c consumer) OnError(ctx context.Context, err error) {
	fmt.Println(err.Error())
}

func (c consumer) GetPollingInterval(ctx context.Context) time.Duration {
	return time.Second
}

func main() {
	ctx := context.Background()
	topicArn := flag.String("t", "", "The ARN of the topic to listen to, cannot be set along with parameter path")
	parameterPath := flag.String("p", "", "The path of the SSM parameter to get the topic ARN from, cannot be set along with topic ARN")
	queueName := flag.String("q", "", "Optional name for the queue to create")

	flag.Parse()

	if *topicArn == "" && *parameterPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		log.Fatalf(
			"Error loading AWS configuration: %s",
			err.Error(),
		)
	}

	if *parameterPath != "" {
		ssmClient := ssm.NewFromConfig(cfg)
		parameterValue, err := getParameter(ctx, ssmClient, *parameterPath)

		if err != nil {
			log.Fatalf(
				"Error fetching Topic ARN from Parameter Store: %s",
				err.Error(),
			)
		}

		*topicArn = *parameterValue
	}

	err = run(
		ctx,
		sns.NewFromConfig(cfg),
		sqs.NewFromConfig(cfg),
		topicArn,
		queueName,
	)

	if err != nil {
		log.Fatalf(
			"Error at runtime: %s",
			err.Error(),
		)
	}
}

func run(ctx context.Context, snsClient SNSAPI, sqsClient SQSAPI, topicArn *string, queueName *string) error {
	queueUrl, err := createQueue(ctx, sqsClient, *queueName, *topicArn)

	if err != nil {
		return err
	}

	defer deleteQueue(ctx, sqsClient, queueUrl)

	queueArn, err := getQueueArn(ctx, sqsClient, queueUrl)

	if err != nil {
		return err
	}

	subscriptionArn, err := subscribeToTopic(ctx, snsClient, topicArn, queueArn)

	if err != nil {
		return err
	}

	defer unsubscribeFromTopic(ctx, snsClient, subscriptionArn)

	listenCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		listenToQueue(
			listenCtx,
			sqsClient,
			queueUrl,
			consumer{},
		)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	return nil
}
