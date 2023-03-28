package listener

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)
	os.Exit(m.Run())
}

type ConsumerImpl struct {
	cancel func()
}

func (c ConsumerImpl) OnMessage(ctx context.Context, msg MessageContent) {
	c.cancel()
}

func (c ConsumerImpl) OnError(ctx context.Context, err error) {}

func TestListenToTopic(t *testing.T) {
	tests := map[string]struct {
		shouldErrOnStartup  bool
		shouldErrOnTeardown bool
		config              ListenerConfiguration
	}{
		"successul setup and teardown": {
			false,
			false,
			ListenerConfiguration{
				QueueName:       "valid-queue",
				TopicArn:        "valid-topic",
				PollingInterval: 10 * time.Millisecond,
			},
		},
		"successful queue setup but unsuccessful subscription": {
			true,
			false,
			ListenerConfiguration{
				QueueName:       "valid-queue",
				TopicArn:        "invalid-topic",
				PollingInterval: 10 * time.Millisecond,
			},
		},
		"successful setup but unsuccessful queue teardown": {
			false,
			true,
			ListenerConfiguration{
				QueueName:       "breaks-on-teardown",
				TopicArn:        "valid-topic",
				PollingInterval: 10 * time.Millisecond,
			},
		},
		"successful setup but unsuccessful unsubscription": {
			false,
			true,
			ListenerConfiguration{
				QueueName:       "valid-queue",
				TopicArn:        "breaks-on-teardown",
				PollingInterval: 10 * time.Millisecond,
			},
		},
		"successful setup but unsuccessful queue deletion and unsubscription": {
			false,
			true,
			ListenerConfiguration{
				QueueName:       "breaks-on-teardown",
				TopicArn:        "breaks-on-teardown",
				PollingInterval: 10 * time.Millisecond,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			ssmClient := &SSMAPIImpl{}
			snsClient := &SNSAPIImpl{}
			sqsClient := &SQSAPIImpl{
				messages: []types.Message{
					{
						Body:          aws.String("foo"),
						MessageId:     aws.String("foo"),
						ReceiptHandle: aws.String("foo-handle"),
					},
				},
			}
			errCh := make(chan error)

			go func() {
				ListenToTopic(
					ctx,
					sqsClient,
					snsClient,
					ssmClient,
					ConsumerImpl{
						cancel: cancel,
					},
					test.config,
					errCh,
				)
			}()

			select {
			case <-ctx.Done():

				err := <-errCh
				for errs := range errCh {
					err = errors.Join(err, errs)
				}

				if err != nil && !test.shouldErrOnTeardown {
					t.Fatalf(
						"Expected no error on teardown but got %s",
						err.Error(),
					)
				}

				if err == nil && test.shouldErrOnTeardown {
					t.Fatal("Expected error on teardown but got no error")
				}

			case err := <-errCh:
				for errs := range errCh {
					err = errors.Join(err, errs)
				}

				if err != nil && !test.shouldErrOnStartup {
					t.Fatalf(
						"Expected no error on startup but got %s",
						err.Error(),
					)
				}

				if err == nil && test.shouldErrOnStartup {
					t.Fatal("Expected error on startup but got no error")
				}

				cancel()
			}
		})
	}
}
