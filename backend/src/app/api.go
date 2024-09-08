package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/alex-holovach/1trc/backend/src/config"
	"github.com/alex-holovach/1trc/backend/src/controller"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"google.golang.org/api/option"
)

type api struct {
	fx.In
	TrcCntrl controller.TrcCntrl
}

func ConfigureApiRoutes(a api, mux *http.ServeMux) {
	a.TrcCntrl.SetRoute(mux)
}

func HttpServerProvider() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func RedisClientProvider(config config.AppConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: config.RedisHost,
	})

	return client
}

func GcsClientProvider(config config.AppConfig, ctx context.Context) *storage.Client {
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(config.ServiceAccountFilePath))
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func PubSubClientProvider(config config.AppConfig, ctx context.Context) *pubsub.Client {
	client, err := pubsub.NewClient(ctx, config.ProjectID, option.WithCredentialsFile(config.ServiceAccountFilePath))
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func RunHttpServer(lc fx.Lifecycle, mux *http.ServeMux) {
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatal(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return server.Shutdown(ctx)
		},
	})
}
