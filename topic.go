package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func subscribeToTopic(ctx context.Context, client SNSAPI, topicArn *string, queueArn *string) (*string, error) {
	result, err := client.Subscribe(
		ctx,
		&sns.SubscribeInput{
			Endpoint:              queueArn,
			Protocol:              aws.String("sqs"),
			ReturnSubscriptionArn: true,
			TopicArn:              topicArn,
		},
	)

	if err != nil {
		return nil, err
	}

	return result.SubscriptionArn, nil
}

func unsubscribeFromTopic(ctx context.Context, client SNSAPI, subscriptionArn *string) error {
	_, err := client.Unsubscribe(
		ctx,
		&sns.UnsubscribeInput{
			SubscriptionArn: subscriptionArn,
		},
	)

	if err != nil {
		return err
	}

	return nil
}
