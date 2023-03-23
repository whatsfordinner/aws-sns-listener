package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

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

	queueArn, err := getQueueArn(ctx, sqsClient, queueUrl)

	if err != nil {
		return err
	}

	subscriptionArn, err := subscribeToTopic(ctx, snsClient, topicArn, queueArn)

	if err != nil {
		return err
	}

	listenCtx, cancel := context.WithCancel(ctx)

	go func() {
		listenToQueue(
			listenCtx,
			sqsClient,
			queueUrl,
			func(m types.Message) { fmt.Printf("Message Body: %s\n", *m.Body) },
			func(err error) { fmt.Println(err.Error()) },
			1000,
		)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	cancel()

	err = unsubscribeFromTopic(ctx, snsClient, subscriptionArn)

	if err != nil {
		return err
	}

	err = deleteQueue(ctx, sqsClient, queueUrl)

	if err != nil {
		return err
	}

	return nil
}
