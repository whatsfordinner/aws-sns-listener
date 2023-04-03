package listener

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// SQSAPI is a shim over v2 of the AWS SDK's sqs client. The sqs client provided by
// github.com/aws/aws-sdk-go-v2/service/sqs automatically satisfies this.
type SQSAPI interface {
	CreateQueue(ctx context.Context,
		params *sqs.CreateQueueInput,
		optFns ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error)

	DeleteQueue(ctx context.Context,
		params *sqs.DeleteQueueInput,
		optFns ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error)

	GetQueueAttributes(ctx context.Context,
		params *sqs.GetQueueAttributesInput,
		optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)

	ReceiveMessage(ctx context.Context,
		params *sqs.ReceiveMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)

	DeleteMessage(ctx context.Context,
		params *sqs.DeleteMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

func createQueue(ctx context.Context, client SQSAPI, queueName string, topicArn string) (string, error) {
	ctx, span := otel.Tracer(name).Start(ctx, "createQueue")
	defer span.End()

	isFIFO, err := isTopicFIFO(ctx, topicArn)

	if err != nil {
		return "", err
	}

	if queueName == "" {
		queueName = "sns-listener-" + uuid.NewString()
	}

	queuePolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {
				"Service": "sns.amazonaws.com"
			},
			"Action": "sqs:SendMessage",
			"Resource": "*",
			"Condition": {
				"ArnEquals": {
					"aws:SourceArn": "%s"
				}
			}
		}]
	}`, topicArn)

	queueAttributes := map[string]string{
		"Policy": queuePolicy,
	}

	if isFIFO {
		queueName += ".fifo"
		queueAttributes["FifoQueue"] = "true"
		queueAttributes["ContentBasedDeduplication"] = "true"
	}

	span.SetAttributes(
		attribute.String(traceNamespace+".queueName", queueName),
		attribute.String(traceNamespace+".topicArn", topicArn),
		attribute.Bool(traceNamespace+".isFIFO", isFIFO),
	)

	logger.Printf("Creating new queue...\n\tName: %s\n\tAllowing messages from topic: %s\n\tFIFO: %t", queueName, topicArn, isFIFO)

	result, err := client.CreateQueue(
		ctx,
		&sqs.CreateQueueInput{
			QueueName:  aws.String(queueName),
			Attributes: queueAttributes,
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String(traceNamespace+".queueUrl", *result.QueueUrl))

	logger.Printf("Queue created with URL %s", *result.QueueUrl)

	span.SetStatus(codes.Ok, "")
	return *result.QueueUrl, nil
}

func getQueueArn(ctx context.Context, client SQSAPI, queueUrl string) (string, error) {
	ctx, span := otel.Tracer(name).Start(ctx, "getQueueArn")
	defer span.End()

	span.SetAttributes(attribute.String(traceNamespace+".queueUrl", queueUrl))

	result, err := client.GetQueueAttributes(
		ctx,
		&sqs.GetQueueAttributesInput{
			QueueUrl: &queueUrl,
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameQueueArn,
			},
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String(traceNamespace+".queueArn", result.Attributes[string(types.QueueAttributeNameQueueArn)]))
	span.SetStatus(codes.Ok, "")

	return result.Attributes[string(types.QueueAttributeNameQueueArn)], nil
}

func listenToQueue(ctx context.Context, client SQSAPI, queueUrl string, consumer Consumer, pollingInterval time.Duration) error {
	logger.Printf("Starting to listen to queue. Fetching messages every %s...", pollingInterval.String())
	for {
		select {
		case <-time.After(pollingInterval):
			ctx, span := otel.Tracer(name).Start(ctx, "listenToQueue")
			defer span.End()

			span.SetAttributes(
				attribute.String(traceNamespace+".queueUrl", queueUrl),
				attribute.String(traceNamespace+".pollingInterval", pollingInterval.String()),
			)
			span.AddEvent("Receiving messages from queue")

			receiveResult, err := client.ReceiveMessage(
				ctx,
				&sqs.ReceiveMessageInput{
					MessageAttributeNames: []string{
						string(types.QueueAttributeNameAll),
					},
					QueueUrl:            &queueUrl,
					MaxNumberOfMessages: 1,
					VisibilityTimeout:   int32(60),
				},
			)

			if err != nil {
				var cancelErr *smithy.CanceledError

				if errors.As(err, &cancelErr) {
					logger.Print("Leaving receive loop early due to cancelled context")

					span.AddEvent("Leaving receive loop early due to cancelled context")
					span.SetStatus(codes.Ok, "")

					return nil
				}

				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return err
			}

			span.SetAttributes(attribute.Int(traceNamespace+".messagesReceived", len(receiveResult.Messages)))

			for _, message := range receiveResult.Messages {
				msgCtx, msgSpan := otel.Tracer(name).Start(ctx, "processMessage")

				msgSpan.SetAttributes(
					attribute.String(traceNamespace+".queueUrl", queueUrl),
					attribute.String(traceNamespace+".messageId", *message.MessageId),
					attribute.String(traceNamespace+".receiptHandle", *message.ReceiptHandle),
				)

				_, err := client.DeleteMessage(
					msgCtx,
					&sqs.DeleteMessageInput{
						QueueUrl:      &queueUrl,
						ReceiptHandle: message.ReceiptHandle,
					},
				)

				if err != nil {
					var cancelErr *smithy.CanceledError

					if errors.As(err, &cancelErr) {
						logger.Print("Leaving receive loop early due to cancelled context")

						span.AddEvent("Leaving receive loop early due to cancelled context")
						span.SetStatus(codes.Ok, "")

						return nil
					}

					msgSpan.RecordError(err)
					msgSpan.SetStatus(codes.Error, err.Error())
					msgSpan.End()
					return err
				}

				consumer.OnMessage(
					msgCtx,
					MessageContent{
						Body: message.Body,
						Id:   message.MessageId,
					})

				msgSpan.SetStatus(codes.Ok, "")
				msgSpan.End()
			}

			span.SetStatus(codes.Ok, "")
			span.End()

		case <-ctx.Done():
			logger.Printf("Context cancelled, no longer listening to queue")
			return nil
		}
	}
}

func deleteQueue(ctx context.Context, client SQSAPI, queueUrl string) error {
	ctx, span := otel.Tracer(name).Start(ctx, "deleteQueue")
	defer span.End()

	span.SetAttributes(attribute.String(traceNamespace+".queueUrl", queueUrl))

	logger.Printf("Deleting queue with URL %s...", queueUrl)

	_, err := client.DeleteQueue(
		ctx,
		&sqs.DeleteQueueInput{
			QueueUrl: &queueUrl,
		},
	)

	if err != nil {
		logger.Printf("Unable to delete queue: %s", err.Error())

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	logger.Printf("Deleted queue")

	span.SetStatus(codes.Ok, "")
	return nil
}
