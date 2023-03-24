package main

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type SNSAPIImpl struct{}

func (c SNSAPIImpl) Subscribe(ctx context.Context,
	params *sns.SubscribeInput,
	optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error) {
	if *params.TopicArn == "valid-topic" {
		return &sns.SubscribeOutput{
			SubscriptionArn: aws.String("foo:bar:baz"),
		}, nil
	}
	return nil, errors.New("Couldn't subscribe to topic")
}

func (c SNSAPIImpl) Unsubscribe(ctx context.Context,
	params *sns.UnsubscribeInput,
	optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error) {
	if *params.SubscriptionArn == "valid:arn" {
		return &sns.UnsubscribeOutput{}, nil
	}

	return nil, errors.New("Could not unsubscribe using that ARN")
}

func TestSubscribe(t *testing.T) {
	tests := map[string]struct {
		shouldErr   bool
		topicArn    *string
		expectedArn string
	}{
		"valid input":   {false, aws.String("valid-topic"), "foo:bar:baz"},
		"invalid input": {true, aws.String("invalid-topic"), ""},
	}

	client := &SNSAPIImpl{}
	ctx := context.TODO()
	queueArn := aws.String("some-valid-queue")

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {})
		result, err := subscribeToTopic(ctx, client, test.topicArn, queueArn)

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
					"Subscription ARN %s did not match expected ARN %s",
					*result,
					test.expectedArn,
				)
			}
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	tests := map[string]struct {
		shouldErr       bool
		subscriptionArn *string
	}{
		"valid subscription ARN":   {false, aws.String("valid:arn")},
		"invalid subscription ARN": {true, aws.String("invalid:arn")},
	}

	ctx := context.TODO()
	client := &SNSAPIImpl{}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			unsubscribeFromTopic(ctx, client, test.subscriptionArn)
		})
	}
}

func TestIsTopicFIFO(t *testing.T) {
	tests := map[string]struct {
		shouldErr bool
		topicArn  *string
		expected  bool
	}{
		"FIFO topic":       {false, aws.String("arn:aws:sns:us-east-1:123456789012:my-topic.fifo"), true},
		"not a FIFO topic": {false, aws.String("arn:aws:sns:us-east-1:123456789012:my-topic"), false},
	}

	ctx := context.TODO()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := isTopicFIFO(ctx, test.topicArn)

			if err != nil && !test.shouldErr {
				t.Fatalf(
					"Expected no error but got %s",
					err.Error(),
				)
			}

			if err == nil && test.shouldErr {
				t.Fatal("Expected error but got no error")
			}

			if err == nil && !test.shouldErr {
				if result != test.expected {
					t.Fatalf(
						"Expected %t for topic with ARN %s but got %t",
						test.expected,
						*test.topicArn,
						result,
					)
				}
			}
		})
	}
}
