package listener

import (
	"context"
)

// A Consumer is used by ListenToTopic to process messages and errors during the course of oeprations. Both of its methods are provided
// with the existing context used by the package so that any implementing type is able to be propagate traces or be made aware of
// context cancellations.
type Consumer interface {
	// OnMessage is called when a message is successfully processed from the SQS queue. If no messages are processed then OnMessage won't
	// be called.
	OnMessage(ctx context.Context, msg MessageContent)
}

// A MessageContent maps the message body and message ID of a SQS message to
// a much more straightforward struct. For the purpose of listening to an SNS
// topic, the Body contains the full message that was published
type MessageContent struct {
	Body *string
	Id   *string
}
