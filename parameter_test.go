package main

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type SSMAPIImpl struct{}

func (c SSMAPIImpl) GetParameter(ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if *params.Name == "/valid/param/path" {
		return &ssm.GetParameterOutput{
			Parameter: &types.Parameter{
				Name:  aws.String("/valid/param/path"),
				Value: aws.String("some-value"),
			},
		}, nil
	}
	return nil, errors.New("Couldn't find param")
}

func TestGetParameter(t *testing.T) {
	tests := map[string]struct {
		shouldErr     bool
		parameterPath string
		expectedValue string
	}{
		"parameter exists":         {false, "/valid/param/path", "some-value"},
		"parameter does not exist": {true, "/invalid/param/path", ""},
	}

	client := &SSMAPIImpl{}
	ctx := context.TODO()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			param, err := getParameter(ctx, client, test.parameterPath)

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
				if test.expectedValue != param {
					t.Fatalf("Parameter value %s did not match expected value %s",
						param,
						test.expectedValue,
					)
				}
			}
		})
	}
}
