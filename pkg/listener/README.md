# AWS SNS listener package

The `listener` package can be used for listening to an Amazon SNS topic. It supports both regular topics and FIFO topics. The original motivation for this project was to be able to subscribe to a SNS topic and identify the shape and content of messages being published, making it easier to build other software. A CLI that wraps this package is available in the root of this repository.

## Using the package

### Importing

`listener` uses semantic versioning and conventional commits for releases. The latest published version can be fetched with:

```
go get github.com/whatsfordinner/aws-sns-listener/pkg/listener
```

A specific version can be fetched with:

```
go get github.com/whatsfordinner/aws-sns-listener/pkg/listener@v0.5.0
```

### Tracing

The package uses the OpenTelemetry library for distributed tracing and if an exporter is defined in the calling service it will emit spans using that exporter.

### Setup

The package uses three AWS APIs for operation:  
* Simple Notification Service for subscribing and unsubscribing to the topic  
* Simple Queue Service for creating a queue, receiving messages from it and then destroying it  
* Systems Manager for (optionally) fetching the SNS Topic ARN from a Parameter Store parameter

The package defines three interfaces for this purpose: `SNSAPI`, `SQSAPI` and `SSMAPI` which are all shims over the the clients provided by v2 of the AWS SDK. The easiest way to use this package would be to create those 3 clients and provide them to `listener.ListenToTopic`.

A configuration object `listener.ListenerConfiguration` is passed in that controls the behaviour of the package when starting up. It is used by the package to identify the topic to subscribe to and some other configuration details.

The simplest possible implementation would be:

```go
listenerCfg := listener.ListenerConfiguration {
    TopicArn: "arn:aws:sns:us-east-1:123456789012:my-sns-topic"
}
```

A more involved implementation that uses a parameter to derive the topic ARN, overrides the queue name, sets a custom polling interval and asks the package to log to `stderr` would be:

```go
listenerCfg := listener.ListenerConfiguration {
    ParameterPath:   "/path/to/topic-arn"
    PollingInterval: 30 * time.Second 
    QueueName:       "my-listener-queue"
}
```

The final component is the `listener.Consumer` interface which is used by the package to notify the calling service of messages. It requires two methods `OnMessage(context.Context, listener.MessageContent)` and `OnError(context.Context, error)`. Context is supplied for use with tracing and to notify the consumer in the event of cancellation. An example implementation would be:

```go
type c struct{}

func (c listener.Consumer) OnMessage(ctx context.Context, msg listener.MessageContent) {
    fmt.Printf("Message content:\n\tID: %s\n\tBody: %s\n", msg.Id, msg.Body)
}

func (c listener.Consumer) OnError(ctx context.Context, err error) {
    fmt.Print(err.Error())
}
```

### Runtime

The package can start being used once the AWS clients, configuration object and consumer have been set up. The entrypoint is `listener.ListenToTopic`. Once running, the function will block until the context provided to it is cancelled, after which it will teardown any infrastructure it has created and return any errors.

```go
ctx, cancel := context.WithCancel(context.Background())

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt) // stop on Ctrl+C

errCh := make(chan error, 1)

go func() {
    errCh <- listener.ListenToTopic(
        ctx,
        sqsClient,
        snsClient,
        ssmClient,
        c{}, // our consumer from earlier
        listenerCfg, // one of the configuration object from before
    )
}()

select {
case err := <- errCh: // catch any errors on startup
    panic(err)
case <- sigCh: // catch SIGINT
    cancel() // notify the goroutine it's time to shutdown
    err := <- errCh // wait for the result of teardown

    if err != nil {
        panic(err)
    }
}
```

### Teardown

When context has been cancelled or in the event of an error, the package will make a best effort attempt to destroy the SNS subcription and SQS queue to ensure it's left things as it's found them. This is not guaranteed to succeed (credential expiry, network failures, might have been authorised to create a resource but not delete it, etc.) and so sometimes there will be resources that are leftover. In that case, the package will return an error containing the errors that occurred while trying to shutdown which can be used to identify and delete those resources manually.
