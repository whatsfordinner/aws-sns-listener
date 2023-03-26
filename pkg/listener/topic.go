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

type SNSAPI interface {
	Subscribe(ctx context.Context,
		params *sns.SubscribeInput,
		optFns ...func(*sns.Options)) (*sns.SubscribeOutput, error)

	Unsubscribe(ctx context.Context,
		params *sns.UnsubscribeInput,
		optFns ...func(*sns.Options)) (*sns.UnsubscribeOutput, error)
}

func subscribeToTopic(ctx context.Context, client SNSAPI, topicArn *string, queueArn *string) (*string, error) {
	ctx, span := otel.Tracer(name).Start(ctx, "subscribeToTopic")
	defer span.End()

	span.SetAttributes(
		attribute.String(traceNamespace+".topicArn", *topicArn),
		attribute.String(traceNamespace+".queueArn", *queueArn),
	)

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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String(traceNamespace+".subscriptionArn", *result.SubscriptionArn))
	span.SetStatus(codes.Ok, "")

	return result.SubscriptionArn, nil
}

func unsubscribeFromTopic(ctx context.Context, client SNSAPI, subscriptionArn *string) {
	ctx, span := otel.Tracer(name).Start(ctx, "unsubscribeFromTopic")
	defer span.End()

	span.SetAttributes(attribute.String(traceNamespace+".subscriptionArn", *subscriptionArn))

	_, err := client.Unsubscribe(
		ctx,
		&sns.UnsubscribeInput{
			SubscriptionArn: subscriptionArn,
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

}

func isTopicFIFO(ctx context.Context, topicArn *string) (bool, error) {
	_, span := otel.Tracer(name).Start(ctx, "isTopicFIFO")
	defer span.End()

	return regexp.MatchString(`\.fifo$`, *topicArn)
}
