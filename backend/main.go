package main

import (
	"context"
	"os"

	"github.com/alex-holovach/1trc/backend/src/app"
	"github.com/alex-holovach/1trc/backend/src/config"
	_ "github.com/joho/godotenv/autoload"
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
