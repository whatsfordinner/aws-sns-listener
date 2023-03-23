# AWS SNS Listener

This util will let you listen to a Simple Notification Service (SNS) topic by creating a Simple Queue Service (SQS) queue and subscribing that queue to the SNS topic. When it's done, it'll delete the queue and the subscription.

## Usage

```
Usage of aws-sns-listener:
  -q string
        Optional name for the queue to create
  -t string
        The ARN of the topic to listen to
```

| Flag | Mandatory | Use |
|------|-----------|-----|
| `-t` | YES | The ARN for the SNS topic that you want to listen to |
| `-q` | NO | Name for the queue you want to create. If not provided it will be a v4 UUID prefixed with `sns-listener-` E.g. `sns-listener-67ea4ab1-fafa-4a1c-ad76-2db314ec17e3` |


Example output:
```
❯ ./aws-sns-listener -t 'arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener'
2023/03/23 15:03:39 Creating new queue...
        Name: sns-listener-1c4b6814-71f1-45ac-bc94-ce4584d2459d
        Allowing messages from topic: arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener
2023/03/23 15:03:39 Queue created with URL: https://sqs.ap-southeast-2.amazonaws.com/123456789012/sns-listener-1c4b6814-71f1-45ac-bc94-ce4584d2459d
2023/03/23 15:03:39 Creating a new SNS subscription...
        SNS topic ARN: arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener
        SQS queue ARN: arn:aws:sqs:ap-southeast-2:123456789012:sns-listener-1c4b6814-71f1-45ac-bc94-ce4584d2459d
2023/03/23 15:03:39 Subscription created with ARN arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener:e1439336-6b92-45ad-8e82-05735ac52e0d
2023/03/23 15:03:39 Starting to listen to queue. Fetching messages every 1.000000 seconds...
Message Body: {
  "Type" : "Notification",
  "MessageId" : "2de326a5-53f1-57bf-b197-7a3082043c87",
  "TopicArn" : "arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener",
  "Message" : "hello there!",
  "Timestamp" : "2023-03-23T04:04:05.116Z",
  "SignatureVersion" : "1",
  "Signature" : "F5cRxaP0gNgKl/ZcBC01obED2cNIKvS0HrLt0ktKIO8iHamkTOYns6srXkSxhVGWkYjhfoUiybu9ewwxOI1Bj5uGyfIUBG10W9Rbl4vN+mZXz4Rh5LME3OwasmnsiUQnOpJqa4GWR2T/wMhspVAv5P8QYdyMtrxjgKjHhHNMCPEcuPqUjJszkyHPTzLeFcqOqbVbSinjotc1PddlS3pKj0AxgmYdV70I+hIJS3EN+MbZt/M7AQw1fjEfMmyN4YLclmA2Kde2wtKqjXmxZAL5/+Oi+RvQCnSY5LeAApf8HUqviVIt3TNfOWuug9V2LzfVzdjf7G79aSjzbctlURFuYw==",
  "SigningCertURL" : "https://sns.ap-southeast-2.amazonaws.com/SimpleNotificationService-56e67fcb41f6fec09b0196692625d385.pem",
  "UnsubscribeURL" : "https://sns.ap-southeast-2.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener:e1439336-6b92-45ad-8e82-05735ac52e0d"
}
^C2023/03/23 15:04:12 Removing subscription with ARN arn:aws:sns:ap-southeast-2:123456789012:aws-sns-topic-listener:e1439336-6b92-45ad-8e82-05735ac52e0d...
2023/03/23 15:04:12 Context cancelled, no longer listening to queue
2023/03/23 15:04:12 Subscription removed
2023/03/23 15:04:12 Deleting queue with URL https://sqs.ap-southeast-2.amazonaws.com/123456789012/sns-listener-1c4b6814-71f1-45ac-bc94-ce4584d2459d...
2023/03/23 15:04:12 Deleted queue
```

## Building

```
go build
```

## Testing

The tests currently mock out the SQS and SNS API so no valid AWS credentials are required