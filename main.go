package main

import (
	"context"
	"flag"
	"fmt"
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
		panic("configuration error, " + err.Error())
	}

	if *parameterPath != "" {
		ssmClient := ssm.NewFromConfig(cfg)
		parameterValue, err := getParameter(ctx, ssmClient, *parameterPath)

		if err != nil {
			panic("error fetching parameter, " + err.Error())
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
		panic("runtime error, " + err.Error())
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		listenToQueue(
			ctx,
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

	return nil
}
