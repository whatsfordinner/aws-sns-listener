package main

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSAPIImpl struct {
	messages []types.Message
}

func (c SQSAPIImpl) CreateQueue(ctx context.Context,
	params *sqs.CreateQueueInput,
	optFns ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error) {
	queueName := *params.QueueName

	valid, _ := regexp.MatchString("[A-Za-z0-9_\\-]{1,80}", queueName)

	if !valid {
		return nil, errors.New("Invalid queue name")
	}

	return &sqs.CreateQueueOutput{
		QueueUrl: aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/" + queueName),
	}, nil
}

func (c SQSAPIImpl) DeleteQueue(ctx context.Context,
	params *sqs.DeleteQueueInput,
	optFns ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error) {
	queueUrl := *params.QueueUrl

	if queueUrl == "https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue" {
		return &sqs.DeleteQueueOutput{}, nil
	}

	return nil, errors.New("Can't delete that queue!")
}

func (c SQSAPIImpl) GetQueueAttributes(ctx context.Context,
	params *sqs.GetQueueAttributesInput,
	optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	queueUrl := *params.QueueUrl

	if queueUrl == "https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue" {
		return &sqs.GetQueueAttributesOutput{
			Attributes: map[string]string{
				"QueueArn": "arn:aws:sqs:us-east-1:123456789012:valid-queue",
			},
		}, nil
	}

	return nil, errors.New("Couldn't get attributes for that queue!")
}

func (c SQSAPIImpl) ReceiveMessage(ctx context.Context,
	params *sqs.ReceiveMessageInput,
	optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	queueUrl := *params.QueueUrl

	if queueUrl == "https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue" {
		if len(c.messages) > 0 {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{c.messages[0]},
			}, nil
		}

		return &sqs.ReceiveMessageOutput{
			Messages: []types.Message{},
		}, nil
	}

	return nil, errors.New("Couldn't receive messages from that queue!")
}

func (c SQSAPIImpl) DeleteMessage(ctx context.Context,
	params *sqs.DeleteMessageInput,
	optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	queueUrl := *params.QueueUrl
	if queueUrl == "https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue" {
		for _, v := range c.messages {
			if *v.ReceiptHandle == *params.ReceiptHandle && *params.ReceiptHandle == *v.Body+"-handle" {
				return &sqs.DeleteMessageOutput{}, nil
			}
		}
	}

	return nil, errors.New("Couldn't delete messages from that queue!")
}

func TestCreateQueue(t *testing.T) {
	tests := map[string]struct {
		shouldErr      bool
		queueName      string
		topicArn       string
		queueUrlRegexp string
	}{
		"generated queue name": {
			false,
			"",
			"arn:aws:sns:us-east-1:123456789012:example-topic",
			"https://sqs.us-east-1.amazonaws.com/123456789012/sns-listener-[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}",
		},
		"overridden queue name": {
			false,
			"test-queue-name",
			"arn:aws:sns:us-east-1:123456789012:example-topic",
			"https://sqs.us-east-1.amazonaws.com/123456789012/test-queue-name",
		},
		"invalid queue name": {
			true,
			"?<>",
			"arn:aws:sns:us-east-1:123456789012:example-topic",
			"",
		},
	}

	client := &SQSAPIImpl{}
	ctx := context.TODO()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			queueUrl, err := createQueue(ctx, client, test.queueName, test.topicArn)

			if err != nil && !test.shouldErr {
				t.Fatalf(
					"Expected no error but got %s",
					err.Error(),
				)
			}

			if err == nil && test.shouldErr {
				t.Fatalf("Expected error but got no error")
			}

			if err == nil && !test.shouldErr {
				match, _ := regexp.Match(test.queueUrlRegexp, []byte(*queueUrl))

				if !match {
					t.Fatalf(
						"Queue URL %s did not match regex %s",
						*queueUrl,
						test.queueUrlRegexp,
					)
				}
			}
		})
	}
}

func TestGetQueueArn(t *testing.T) {
	tests := map[string]struct {
		shouldErr   bool
		queueUrl    *string
		expectedArn string
	}{
		"valid queue": {
			false,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue"),
			"arn:aws:sqs:us-east-1:123456789012:valid-queue",
		},
		"invalid queue": {
			true,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/invalid-queue"),
			"",
		},
	}

	client := &SQSAPIImpl{}
	ctx := context.TODO()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := getQueueArn(ctx, client, test.queueUrl)

			if err != nil && !test.shouldErr {
				t.Fatalf(
					"Expected no error but got %s",
					err.Error(),
				)
			}

			if err == nil && test.shouldErr {
				t.Fatalf("Expected error but got no error")
			}

			if err == nil && !test.shouldErr {
				if *result != test.expectedArn {
					t.Fatalf(
						"Queue ARN %s did not match expected ARN %s",
						*result,
						test.expectedArn,
					)
				}
			}
		})
	}
}

func TestListenToQueue(t *testing.T) {
	tests := map[string]struct {
		shouldErr bool
		queueUrl  *string
		messages  []types.Message
	}{
		"valid queue with valid receipts": {
			false,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue"),
			[]types.Message{
				{
					Body:          aws.String("foo"),
					ReceiptHandle: aws.String("foo-handle"),
				},
			},
		},
		"empty queue": {
			false,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue"),
			[]types.Message{},
		},
		"invalid queue": {
			true,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/invalid-queue"),
			[]types.Message{
				{
					Body:          aws.String("foo"),
					ReceiptHandle: aws.String("foo-handle"),
				},
			},
		},
		"valid queue with invalid receipts": {
			true,
			aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue"),
			[]types.Message{
				{
					Body:          aws.String("foo"),
					ReceiptHandle: aws.String("bar-handle"),
				},
			},
		},
	}

	client := SQSAPIImpl{}
	ctx := context.Background()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			client.messages = test.messages
			receivedMessages := []types.Message{}
			err := make(chan error, 1)

			go func() {
				listenToQueue(
					ctx,
					client,
					test.queueUrl,
					func(m types.Message) {
						receivedMessages = append(receivedMessages, m)
					},
					func(e error) {
						err <- e
					},
					10,
				)
			}()

			for len(err) == 0 && len(receivedMessages) < len(test.messages) {
			}

			cancel()

			if len(err) > 0 && !test.shouldErr {
				t.Fatalf("Expected no error but got %s",
					(<-err).Error(),
				)
			}

			if len(err) == 0 && test.shouldErr {
				t.Fatal("Expected error but got no error")
			}
		})
	}
}

func TestDeleteQueue(t *testing.T) {
	tests := map[string]struct {
		shouldErr bool
		queueUrl  *string
	}{
		"valid queue":   {false, aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/valid-queue")},
		"invalid queue": {true, aws.String("https://sqs.us-east-1.amazonaws.com/123456789012/invalid-queue")},
	}

	client := &SQSAPIImpl{}
	ctx := context.TODO()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := deleteQueue(ctx, client, test.queueUrl)

			if err != nil && !test.shouldErr {
				t.Fatalf(
					"Expected no error but got %s",
					err.Error(),
				)
			}

			if err == nil && test.shouldErr {
				t.Fatalf("Expected error but got no error")
			}
		})
	}
}
