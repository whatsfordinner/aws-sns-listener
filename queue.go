package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
)

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

type Consumer interface {
	OnMessage(ctx context.Context, m MessageContent)

	OnError(ctx context.Context, err error)

	GetPollingInterval(ctx context.Context) time.Duration
}

type MessageContent struct {
	Body *string
	Id   *string
}

func createQueue(ctx context.Context, client SQSAPI, queueName string, topicArn string) (*string, error) {
	isFIFO, err := isTopicFIFO(ctx, &topicArn)

	if err != nil {
		return nil, err
	}

	if queueName == "" {
		queueName = "sns-listener-" + uuid.NewString()
	}

	if isFIFO {
		queueName += ".fifo"
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
		queueAttributes["FifoQueue"] = "true"
		queueAttributes["ContentBasedDeduplication"] = "true"
	}

	log.Printf("Creating new queue...\n\tName: %s\n\tAllowing messages from topic: %s\n\tFIFO: %t\n", queueName, topicArn, isFIFO)

	result, err := client.CreateQueue(
		ctx,
		&sqs.CreateQueueInput{
			QueueName:  aws.String(queueName),
			Attributes: queueAttributes,
		},
	)

	if err != nil {
		return nil, err
	}

	log.Printf("Queue created with URL: %s", *result.QueueUrl)

	return result.QueueUrl, nil
}

func getQueueArn(ctx context.Context, client SQSAPI, queueUrl *string) (*string, error) {
	result, err := client.GetQueueAttributes(
		ctx,
		&sqs.GetQueueAttributesInput{
			QueueUrl: queueUrl,
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameQueueArn,
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return aws.String(result.Attributes[string(types.QueueAttributeNameQueueArn)]), nil
}

func listenToQueue(ctx context.Context, client SQSAPI, queueUrl *string, consumer Consumer) {
	log.Printf("Starting to listen to queue. Fetching messages every %s...", consumer.GetPollingInterval(ctx))
	for {
		select {
		case <-time.After(consumer.GetPollingInterval(ctx)):
			receiveResult, err := client.ReceiveMessage(
				ctx,
				&sqs.ReceiveMessageInput{
					MessageAttributeNames: []string{
						string(types.QueueAttributeNameAll),
					},
					QueueUrl:            queueUrl,
					MaxNumberOfMessages: 1,
					VisibilityTimeout:   int32(60),
				},
			)

			if err != nil {
				consumer.OnError(ctx, err)
				continue
			}

			for _, message := range receiveResult.Messages {
				_, err := client.DeleteMessage(
					ctx,
					&sqs.DeleteMessageInput{
						QueueUrl:      queueUrl,
						ReceiptHandle: message.ReceiptHandle,
					},
				)

				if err != nil {
					consumer.OnError(ctx, err)
				}

				consumer.OnMessage(ctx, MessageContent{
					Body: message.Body,
					Id:   message.MessageId,
				})
			}

		case <-ctx.Done():
			log.Printf("Context cancelled, no longer listening to queue")
			return
		}
	}
}

func deleteQueue(ctx context.Context, client SQSAPI, queueUrl *string) {
	log.Printf("Deleting queue with URL %s...", *queueUrl)
	_, err := client.DeleteQueue(
		ctx,
		&sqs.DeleteQueueInput{
			QueueUrl: queueUrl,
		},
	)

	if err != nil {
		log.Printf("Unable to delete queue: %s", err.Error())
	}

	log.Printf("Deleted queue")
}
