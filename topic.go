package main

import (
	"context"
	"log"
	"regexp"

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
	log.Printf("Creating a new SNS subscription...\n\tSNS topic ARN: %s\n\tSQS queue ARN: %s\n", *topicArn, *queueArn)
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

	log.Printf("Subscription created with ARN %s", *result.SubscriptionArn)

	return result.SubscriptionArn, nil
}

func unsubscribeFromTopic(ctx context.Context, client SNSAPI, subscriptionArn *string) {
	log.Printf("Removing subscription with ARN %s...", *subscriptionArn)
	_, err := client.Unsubscribe(
		ctx,
		&sns.UnsubscribeInput{
			SubscriptionArn: subscriptionArn,
		},
	)

	if err != nil {
		log.Printf("Unable to unsubscribe from topic: %s", err.Error())
	}

	log.Printf("Subscription removed")
}

func isTopicFIFO(ctx context.Context, topicArn *string) (bool, error) {
	return regexp.MatchString(`\.fifo$`, *topicArn)
}
