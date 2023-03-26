package listener

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// SSMAPI is a shim over v2 of the AWS SDK's ssm client. The ssm client provided by
// github.com/aws/aws-sdk-go-v2/service/ssm automatically satisfies this.
type SSMAPI interface {
	GetParameter(ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

func getParameter(ctx context.Context, client SSMAPI, parameterPath string) (*string, error) {
	ctx, span := otel.Tracer(name).Start(ctx, "getParameter")
	defer span.End()

	span.SetAttributes(attribute.String(traceNamespace+".ssmParameter", parameterPath))

	logger.Printf("Fetching topic ARN from SSM parameter at path %s...", parameterPath)

	result, err := client.GetParameter(
		ctx,
		&ssm.GetParameterInput{
			Name:           aws.String(parameterPath),
			WithDecryption: aws.Bool(true),
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	logger.Printf("Successfully fetched paramater value: %s", *result.Parameter.Value)

	span.SetAttributes(attribute.String(traceNamespace+".ssmParameterValue", *result.Parameter.Value))
	span.SetStatus(codes.Ok, "")

	return result.Parameter.Value, nil
}
