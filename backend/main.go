package main

import (
	"context"
	"log"
	"os"

	"github.com/alex-holovach/1trc/backend/src/app"
	"github.com/alex-holovach/1trc/backend/src/config"
	_ "github.com/joho/godotenv/autoload"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
)

func main() {
	cfg := config.AppConfig{
		ProjectID:              os.Getenv("PROJECT_ID"),
		SubscriptionID:         os.Getenv("SUBSCRIPTION_ID"),
		TopicName:              os.Getenv("TOPIC_NAME"),
		ServiceAccountFilePath: os.Getenv("SERVICE_ACCOUNT_PATH"),
		RedisHost:              os.Getenv("REDIS_HOST"),
		BucketName:             os.Getenv("BUCKET_NAME"),
	}

	ctx := context.Background()
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(os.Getenv("OTEL_ENDPOINT")),
	)
	if err != nil {
		log.Fatal(err, "Failed to create exporter")
	}
	defer exporter.Shutdown(ctx)

	openTelemetryURL := attribute.KeyValue{
		Key:   attribute.Key("opentelemetry.io/schemas"),
		Value: attribute.StringValue("1.7.0"),
	}

	resource, err := resource.New(ctx,
		resource.WithAttributes(
			openTelemetryURL,
		),
	)
	if err != nil {
		log.Fatal(err, "Failed to create resource")
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)

	otel.SetTracerProvider(tracerProvider)

	fx.New(
		fx.Provide(
			func() config.AppConfig { return cfg },
			context.Background,
			app.RedisClientProvider,
			app.GcsClientProvider,
			app.PubSubClientProvider,
			app.HttpServerProvider,
			app.PubSubWorkerProvider,
		),
		fx.Invoke(
			app.ConfigureApiRoutes,
			app.RunHttpServer,
			app.RunWorker,
		),
	).Run()
}
