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
	"github.com/whatsfordinner/aws-sns-listener/pkg/listener"
)

type consumer struct{}

func (c consumer) OnMessage(ctx context.Context, m listener.MessageContent) {
	fmt.Printf("Message body: %s\n", *m.Body)
}

func (c consumer) OnError(ctx context.Context, err error) {
	fmt.Println(err.Error())
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	topicArn := flag.String("t", "", "The ARN of the topic to listen to, cannot be set along with parameter path")
	parameterPath := flag.String("p", "", "The path of the SSM parameter to get the topic ARN from, cannot be set along with topic ARN")
	queueName := flag.String("q", "", "Optional name for the queue to create")
	pollingInterval := flag.Int("i", 0, "Optional duration for delay when polling the SQS queue")

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

	listenerCfg := listener.ListenerConfiguration{
		ParameterPath:   *parameterPath,
		PollingInterval: time.Duration(*pollingInterval) * time.Millisecond,
		QueueName:       *queueName,
		TopicArn:        *topicArn,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	result := make(chan error, 1)

	go func() {
		result <- listener.ListenToTopic(
			ctx,
			sqs.NewFromConfig(cfg),
			sns.NewFromConfig(cfg),
			ssm.NewFromConfig(cfg),
			consumer{},
			listenerCfg,
		)
	}()

	<-c
	cancel()

	if (<-result) != nil {
		log.Fatalf(
			"Error at runtime: %s",
			err.Error(),
		)
	}
}
