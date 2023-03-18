package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type SNSAPI interface {
	Subscribe(ctx context.Context,
		params *sns.SubscribeInput,
		optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)

	Unsubscribe(ctx context.Context,
		params *sns.UnsubscribeInput,
		optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
}

func subscribeToTopic(ctx context.Context, topicArn *string) (*string, error) {
	return nil, nil
}

func unsubscribeFromTopic(ctx context.Context, subscriptionArn *string) error {
	return nil
}
