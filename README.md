# AWS SNS Listener

This util will let you listen to a Simple Notification Service (SNS) topic by creating a Simple Queue Service (SQS) queue and subscribing that queue to the SNS topic. When it's done, it'll delete the queue and the subscription.

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
❯ aws-sns-listener -o -v -p /sns-listener/topic-arn
2023/03/26 18:53:45 Initialising GRPC OTLP exporter...
2023/03/26 18:53:45 Fetching topic ARN from SSM parameter at path /sns-listener/topic-arn...
2023/03/26 18:53:46 Successfully fetched paramater value: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
2023/03/26 18:53:46 Creating new queue...
        Name: sns-listener-a9bc952e-642c-4ed8-90c5-26f637fb9dc9
        Allowing messages from topic: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
        FIFO: false
2023/03/26 18:53:46 Queue created with URL https://sqs.us-east-1.amazonaws.com/123456789012/sns-listener-a9bc952e-642c-4ed8-90c5-26f637fb9dc9
2023/03/26 18:53:46 Creating a new SNS subscription...
        SNS topic ARN: arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener
        SQS queue ARN: arn:aws:sqs:us-east-1:123456789012:sns-listener-a9bc952e-642c-4ed8-90c5-26f637fb9dc9
2023/03/26 18:53:46 Subscription created with ARN arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener:72181f9e-6d06-4a1c-95d8-b65c72dce84f
2023/03/26 18:53:46 Starting to listen to queue. Fetching messages every 1s...
^C2023/03/26 18:53:50 Received interrupt instruction, cancelling context
2023/03/26 18:53:50 Context cancelled, no longer listening to queue
2023/03/26 18:53:50 Removing subscription with ARN arn:aws:sns:us-east-1:123456789012:aws-sns-topic-listener:72181f9e-6d06-4a1c-95d8-b65c72dce84f...
2023/03/26 18:53:50 Subscription removed
2023/03/26 18:53:50 Deleting queue with URL https://sqs.us-east-1.amazonaws.com/123456789012/sns-listener-a9bc952e-642c-4ed8-90c5-26f637fb9dc9...
2023/03/26 18:53:50 Deleted queue
2023/03/26 18:53:50 Teardown complete
```

## Building

```
go build
```

## Testing

The tests currently mock out the SQS and SNS API so no valid AWS credentials are required

```
go test ./...
```