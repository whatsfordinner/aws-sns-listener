/*
AWS-SNS-Listener listens to SNS topics.
It prints the body of any messages published to an SNS topic to stdout.
It accomplishes this by creating an SQS queue, subscribing it to the topic and then periodically receiving messages.

Usage:

	aws-sns-listener [flags]

The flags are:

	-t
		The ARN of the SNS topic to subscribe to.
		Mutually exclusive with -p
	-p
		The Systems Manager Parameter Store parameter to use to resolve the topic ARN.
		Mutually exclusive with -t
	-q
		The desired name for the SQS queue.
		The queue name does not need to include ".fifo" for FIFO topics.
		If omitted the queue name wil be a v4 UUID prefixed with "sns-listener-".
	-p
		The interval between messages to receive from the queue in miliseconds.
		If omitted the value will be 1 second.
	-v
		Enable logging from the listener package used by this utility.
	-o
		Enable the OpenTelemetry gRPC exporter.
		Uses insecure transport.
		Destination can be controlled with standard environment variables.
		See: https://opentelemetry.io/docs/concepts/sdk-configuration/otlp-exporter-configuration/

AWS-SNS-Listener uses v2 of the AWS SDK for interacting with the SNS, SQS and SSM APIs.
The default credential provider is used and it does not accept named profiles.
See: https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials

Messages are written to stdout while logs are written to stderr.
This allows message content to be piped or redirected without pollution by logs.

Only one message at a time is received from the queue so high volume topics may result in a very full queue.
This utility is not meant for processing high volumes of messages but to help troubleshoot SNS without fussing with email or SMS.
*/
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
	"github.com/whatsfordinner/aws-sns-listener/internal/resolve"
	"github.com/whatsfordinner/aws-sns-listener/pkg/listener"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

type consumer struct{}

func (c consumer) OnMessage(ctx context.Context, m listener.MessageContent) {
	fmt.Println(*m.Body)
}

func main() {
	ctx := context.Background()

	topicArn := flag.String("t", "", "The ARN of the topic to listen to, cannot be set along with parameter path")
	parameterPath := flag.String("p", "", "The path of the SSM parameter to get the topic ARN from, cannot be set along with topic ARN")
	queueName := flag.String("q", "", "Optional name for the queue to create")
	pollingInterval := flag.Int("i", 0, "Optional duration for delay when polling the SQS queue")
	verbose := flag.Bool("v", false, "Log listener package events")
	enableOtlp := flag.Bool("o", false, "Enable the GRPC OTLP exporter")

	flag.Parse()

	if *topicArn == "" && *parameterPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *enableOtlp {
		log.Print("Initialising GRPC OTLP exporter...")
		shutdownTracing, err := initTracing()

		if err != nil {
			log.Fatalf(
				"Error initialising OpenTelemetry: %s",
				err.Error(),
			)
		}

		defer shutdownTracing()
	}

	cfg, err := config.LoadDefaultConfig(ctx)

	otelaws.AppendMiddlewares(&cfg.APIOptions)

	if err != nil {
		log.Fatalf(
			"Error loading AWS configuration: %s",
			err.Error(),
		)
	}

	if *parameterPath != "" {
		paramTopicArn, err := resolve.GetParameter(
			ctx,
			ssm.NewFromConfig(cfg),
			*parameterPath,
		)

		if err != nil {
			log.Fatalf(
				"Error reading parameter from path %s: %s",
				*parameterPath,
				err.Error(),
			)
		}

		*topicArn = paramTopicArn
	}

	topicListener := listener.New(
		*topicArn,
		sns.NewFromConfig(cfg),
		sqs.NewFromConfig(cfg),
		listener.WithQueueName(*queueName),
		listener.WithPollingInterval(time.Duration(*pollingInterval)*time.Millisecond),
		listener.WithVerbose(*verbose),
	)

	err = topicListener.Setup(ctx)

	if err != nil {
		log.Fatalf(err.Error())
	}

	errCh := make(chan error, 1)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	listenCtx, cancel := context.WithCancel(ctx)

	go func() {
		errCh <- topicListener.Listen(listenCtx, consumer{})
	}()

	select {
	case err := <-errCh:
		log.Printf("Runtime error: %s", err.Error())
	case <-sigCh:
		log.Print("Received interrupt instruction, cancelling context")

		cancel()
		err = <-errCh

		if err != nil {
			log.Printf("Error while cancelling context: %s", err.Error())
		}
	}

	err = topicListener.Teardown(ctx)

	if err != nil {
		log.Fatalf(err.Error())
	}
}
