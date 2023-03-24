package listener

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type SSMAPI interface {
	GetParameter(ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

func getParameter(ctx context.Context, client SSMAPI, parameterPath string) (*string, error) {
	log.Printf("Fetching topic ARN from SSM parameter %s...", parameterPath)
	result, err := client.GetParameter(
		ctx,
		&ssm.GetParameterInput{
			Name:           aws.String(parameterPath),
			WithDecryption: aws.Bool(true),
		},
	)

	if err != nil {
		return nil, err
	}

	log.Printf("Successfully fetched parameter value %s", *result.Parameter.Value)

	return result.Parameter.Value, nil
}
