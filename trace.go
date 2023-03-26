package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func newOtlpExporter() (trace.SpanExporter, error) {
	return otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithInsecure(),
	)
}

func newResource() *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("aws-sns-listener"),
		),
	)

	return r
}

func initTracing() (func(), error) {
	var exp trace.SpanExporter
	var err error

	exp, err = newOtlpExporter()

	if err != nil {
		return func() {}, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(newResource()),
	)

	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}, nil
}
