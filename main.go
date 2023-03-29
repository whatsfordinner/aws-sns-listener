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
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

const name string = "github.com/whatsfordinner/aws-sns-listener"
const traceNamespace string = "listener-util"

type consumer struct{}

func (c consumer) OnMessage(ctx context.Context, m listener.MessageContent) {
	fmt.Printf("Message body: %s\n", *m.Body)
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
		paramTopicArn, err := getParameter(
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
