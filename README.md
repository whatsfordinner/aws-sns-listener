# AWS SNS Listener

This util will let you listen to a Simple Notification Service (SNS) topic by creating a Simple Queue Service (SQS) queue and subscribing that queue to the SNS topic. When it's done, it'll delete the queue and the subscription.

Message body is printed to stdout and logs are printed to stderr so messages can be piped or redirected without worrying about pollution from log messages.

You can also import the package yourself and use it for your own purposes. Check the documentation [here](./pkg/listener/README.md)!

Pre-compiled binaries are available on the [releases page](https://github.com/whatsfordinner/aws-sns-listener/releases).

## Usage

```
Usage of aws-sns-listener:
  -i int
        Optional duration for delay when polling the SQS queue
  -o    Enable the GRPC OTLP exporter
  -p string
        The path of the SSM parameter to get the topic ARN from, cannot be set along with topic ARN
  -q string
        Optional name for the queue to create
  -t string
        The ARN of the topic to listen to, cannot be set along with parameter path
  -v    Log listener package events
```

| Flag | Use |
|------|-----|
| `-t` | The ARN for the SNS topic that you want to listen to |
| `-q` | Name for the queue you want to create. If not provided it will be a v4 UUID prefixed with `sns-listener-` E.g. `sns-listener-67ea4ab1-fafa-4a1c-ad76-2db314ec17e3` |
| `-p` | The path to the SSM parameter you want to get the ARN for the SNS topic from. Overwrites `-t` |
| `-i` | The interval for polling the SQS queue in milliseconds |
| `-o` | Enable the GRPC OTLP exporter for distributed tracing |
| `-v` | Enable logging to stderr for the `listener` package |

Only one of `-t` or `-p` must be provided. All others are optional

Example output:
```
❯ aws-sns-listener -v -p /sns-listener/topic-arn
2023/03/30 21:49:37 Fetching topic ARN from SSM parameter at path /sns-listener/topic-arn...
2023/03/30 21:49:38 Successfully fetched paramater value: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
2023/03/30 21:49:38 Provided polling interval invalid: 0s. Defaulting to 1 second
2023/03/30 21:49:38 Creating new queue...
	Name: sns-listener-2ee83613-3e69-497e-8378-3ef7b9ba50a8
	Allowing messages from topic: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
	FIFO: false
2023/03/30 21:49:38 Queue created with URL https://sqs.us-east-1.amazonaws.com/123456789012/sns-listener-2ee83613-3e69-497e-8378-3ef7b9ba50a8
2023/03/30 21:49:38 Creating a new SNS subscription...
	SNS topic ARN: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
	SQS queue ARN: arn:aws:sqs:us-east-1:123456789012:sns-listener-2ee83613-3e69-497e-8378-3ef7b9ba50a8
2023/03/30 21:49:38 Subscription created with ARN arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener:a051f49b-75b3-4a77-91b2-0cf1c64d9bfb
2023/03/30 21:49:38 Starting to listen to queue. Fetching messages every 1s...
{
  "Type" : "Notification",
  "MessageId" : "834b4a6e-7412-5a71-ba02-16f11fbcd2bc",
  "TopicArn" : "arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener",
  "Message" : "Hello from SNS!",
  "Timestamp" : "2023-03-30T10:50:22.443Z",
  "SignatureVersion" : "1",
  "Signature" : "uPYnR89ZJHHx5VqNOtZ1+vXz2B1gVPoNAJAyDPG4REFmnWKMTf2jUkg/oGLRVtRotAJYDdJ+xlV7tg0nshiSQ3bTj6ryNbrSrmSs1pSTKRT99UAw0RlEhzWQrvhHB1xhTJ15x21gMsuzm5wgtP3BK1qBluV0KoUzpa8H8Uk9jWElycArQyhJS8MKV57EzyFY6AW+0GHsO7PTkhB8K+aM9GtmW52yqgMYXds8vPsh83b4REVGiMlS6s/XPa+3UcKsEqNRDxi/JQ3rwosFwsZgITaXD9R3UoEETuE0jxFtphsXn6mDOVJc6oSnODEbHu0/PdZCGn2pQqkDcUZIuroZsQ==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-56e67fcb41f6fec09b0196692625d385.pem",
  "UnsubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener:a051f49b-75b3-4a77-91b2-0cf1c64d9bfb"
}
^C2023/03/30 21:50:35 Received interrupt instruction, cancelling context
2023/03/30 21:50:35 Context cancelled, no longer listening to queue
2023/03/30 21:50:35 Removing subscription with ARN arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener:a051f49b-75b3-4a77-91b2-0cf1c64d9bfb...
2023/03/30 21:50:35 Subscription removed
2023/03/30 21:50:35 Deleting queue with URL https://sqs.us-east-1.amazonaws.com/123456789012/sns-listener-2ee83613-3e69-497e-8378-3ef7b9ba50a8...
2023/03/30 21:50:35 Deleted queue
```

The utility will make the best possible effort to clean up any infrastructure in the event of failure. However, it is possible you might wind up with a queue and a subscription lying around in your AWS account that you don't want.

## Building

```
go build
```

## Testing

The tests currently mock out the SQS and SNS API so no valid AWS credentials are required

```
go test ./...
```