package main

import (
	"context"
	"fmt"
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

	ReceiveMessage(ctx context.Context,
		params *sqs.ReceiveMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)

	DeleteMessage(ctx context.Context,
		params *sqs.DeleteMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

func createQueue(ctx context.Context, client SQSAPI, queueName string) (*string, error) {
	if queueName == "" {
		queueName = "sns-listener-" + uuid.NewString()
	}

	result, err := client.CreateQueue(
		ctx,
		&sqs.CreateQueueInput{
			QueueName: aws.String(queueName),
			Attributes: map[string]string{
				"DelaySeconds":           "60",
				"MessageRetentionPeriod": "86400",
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return result.QueueUrl, nil
}

func listenToQueue(ctx context.Context, client SQSAPI, queueUrl *string) error {
	for {
		select {
		case <-time.After(5 * time.Second):
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
				return err
			}

			for _, message := range receiveResult.Messages {
				fmt.Printf(
					"MessageId: %s\nMessage Body: %s\n",
					*message.MessageId,
					*message.Body,
				)

				_, err := client.DeleteMessage(
					ctx,
					&sqs.DeleteMessageInput{
						QueueUrl:      queueUrl,
						ReceiptHandle: message.ReceiptHandle,
					},
				)

				if err != nil {
					return err
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func deleteQueue(ctx context.Context, client SQSAPI, queueUrl *string) error {
	_, err := client.DeleteQueue(
		ctx,
		&sqs.DeleteQueueInput{
			QueueUrl: queueUrl,
		},
	)

	if err != nil {
		return err
	}

	return nil
}
