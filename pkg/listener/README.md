# AWS SNS listener package

[![Go Reference](https://pkg.go.dev/badge/github.com/whatsfordinner/aws-sns-listener/pkg/listener.svg)](https://pkg.go.dev/github.com/whatsfordinner/aws-sns-listener/pkg/listener)

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

The package uses two AWS APIs for operation:  
* Simple Notification Service for subscribing and unsubscribing to the topic  
* Simple Queue Service for creating a queue, receiving messages from it and then destroying it  

The package defines two interfaces for this purpose: `SNSAPI` and `SQSAPI` which are all shims over the the clients provided by v2 of the AWS SDK. The easiest way to use this package would be to create those 2 clients and provide them to `listener.New`.

Configuration and execution are managed by the `listener.Listener` struct which tracks the SNS topic as well as the SQS queue and its subcription to the topic. A new Listener is created with `listener.New` which takes in the topic ARN and the AWS clients. The simplest possible implementation would be:

```go
l := listener.New(
    "arn:aws:sns:us-east-1:123456789012:my-topic",
    sns.NewFromConfig(cfg),
    sqs.NewFromConfig(cfg),
)
```

A more complication configuration which overrides the queue name, enables verbose logging and sets a polling interval of 250 miliseconds would be:

```go
l := listener.New(
    "arn:aws:sns-us-east-1:123456789012:my-topic",
    sns.NewFromConfig(cfg),
    sqs.NewFromConfig(cfg),
    listener.WithQueueName("my-queue"),
    listener.WithVerbose(true),
    listener.WithPollingInterval(250 * time.Milisecond),
)
```

The final component is the `listener.Consumer` interface which is used by the package to notify the calling service of messages. It requires only a single method: `OnMessage(context.Context, listener.MessageContent)`. Context is supplied for use with tracing and to notify the consumer in the event of cancellation. An example implementation would be:

```go
type consumer struct{}

func (c consumer) OnMessage(ctx context.Context, msg listener.MessageContent) {
    fmt.Printf("Message content:\n\tID: %s\n\tBody: %s\n", msg.Id, msg.Body)
}
```

### Runtime

The package can start being used once the Listener struct has been created. The struct receives three methods:  
* `Setup` which creates the SQS queue and subscribes it to the SNS topic
* `Listen` which receives messages from the queue and passes them to a `listener.Consumer`'s `OnMessage` method  
* `Teardown` which destroys the subscription and queue

A possible implementation would be:

```go
ctx := context.Background()

err := l.Setup(ctx) // our listener from earlier

if err != nil {
    panic(err)
}

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt) // stop on Ctrl+C

errCh := make(chan error, 1)

listenCtx, cancel := context.WithCancel(ctx)

go func() {
    errCh <-l.Listen(
        listenCtx,
        consumer{}, // our consumer from earlier
    )
}()

select {
case err := <- errCh: // catch any errors while listening
    fmt.Println(err.Error()) // log them but we still want to cleanup
case <- sigCh: // catch SIGINT
    cancel() // notify the goroutine it's time to shutdown
    err := <- errCh // catch any errors while stopping

    if err != nil {
        fmt.Println(err.Error()) // log them but proceed to cleanup
    }
}

_ = l.Teardown(ctx) // be careful not to use cancelled context here
```

### Teardown

It is best not to panic even if an error occurs when setting up or listening. The teardown method will still be able to run because the struct has kept track of what needs destroying (if it's been created). If you don't care about catching teardown error specifically, it's fine to `defer` the method.