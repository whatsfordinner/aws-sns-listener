package listener

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// SNSAPI is a shim over v2 of the AWS SDK's sns client. The sns client provided by
// github.com/aws/aws-sdk-go-v2/service/sns automatically satisfies this.
type SNSAPI interface {
	Subscribe(ctx context.Context,
		params *sns.SubscribeInput,
		optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)

	Unsubscribe(ctx context.Context,
		params *sns.UnsubscribeInput,
		optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
}

func subscribeToTopic(ctx context.Context, client SNSAPI, topicArn string, queueArn string) (string, error) {
	ctx, span := otel.Tracer(name).Start(ctx, "subscribeToTopic")
	defer span.End()

	span.SetAttributes(
		attribute.String(traceNamespace+".topicArn", topicArn),
		attribute.String(traceNamespace+".queueArn", queueArn),
	)

	logger.Printf("Creating a new SNS subscription...\n\tSNS topic ARN: %s\n\tSQS queue ARN: %s", topicArn, queueArn)

	result, err := client.Subscribe(
		ctx,
		&sns.SubscribeInput{
			Endpoint:              &queueArn,
			Protocol:              aws.String("sqs"),
			ReturnSubscriptionArn: true,
			TopicArn:              &topicArn,
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String(traceNamespace+".subscriptionArn", *result.SubscriptionArn))
	span.SetStatus(codes.Ok, "")

	logger.Printf("Subscription created with ARN %s", *result.SubscriptionArn)

	return *result.SubscriptionArn, nil
}

func unsubscribeFromTopic(ctx context.Context, client SNSAPI, subscriptionArn string) error {
	ctx, span := otel.Tracer(name).Start(ctx, "unsubscribeFromTopic")
	defer span.End()

	span.SetAttributes(attribute.String(traceNamespace+".subscriptionArn", subscriptionArn))

	logger.Printf("Removing subscription with ARN %s...", subscriptionArn)

	_, err := client.Unsubscribe(
		ctx,
		&sns.UnsubscribeInput{
			SubscriptionArn: &subscriptionArn,
		},
	)

	if err != nil {
		logger.Printf("Unable to subscribe from topic: %s", err.Error())

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	logger.Print("Subscription removed")

	span.SetStatus(codes.Ok, "")
	return nil
}

func isTopicFIFO(ctx context.Context, topicArn string) (bool, error) {
	_, span := otel.Tracer(name).Start(ctx, "isTopicFIFO")
	defer span.End()

	return regexp.MatchString(`\.fifo$`, topicArn)
}
