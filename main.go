package main

import (
	"context"
	"errors"
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

	listenerCfg := listener.ListenerConfiguration{
		ParameterPath:   *parameterPath,
		PollingInterval: time.Duration(*pollingInterval) * time.Millisecond,
		QueueName:       *queueName,
		TopicArn:        *topicArn,
		Verbose:         *verbose,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	errCh := make(chan error)

	go func() {
		listener.ListenToTopic(
			ctx,
			sqs.NewFromConfig(cfg),
			sns.NewFromConfig(cfg),
			ssm.NewFromConfig(cfg),
			consumer{},
			listenerCfg,
			errCh,
		)
	}()

	select {
	case err := <-errCh:
		for errs := range errCh {
			err = errors.Join(err, errs)
		}

		log.Fatalf("Runtime error: %s", err.Error())
	case <-sigCh:
		log.Print("Received interrupt instruction, cancelling context")

		cancel()
		err := <-errCh

		for errs := range errCh {
			err = errors.Join(err, errs)
		}

		if err != nil {
			log.Fatalf("Error during teardown: %s", err.Error())
		}

		log.Print("Teardown complete")
	}
}
