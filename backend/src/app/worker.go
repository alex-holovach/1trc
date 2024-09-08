package app

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/pubsub"
	"github.com/alex-holovach/1trc/backend/src/config"
	"github.com/alex-holovach/1trc/backend/src/consumer"
	"go.uber.org/fx"
)

type PubSubWorker struct {
	client         *pubsub.Client
	subscription   *pubsub.Subscription
	messageHandler func(context.Context, *pubsub.Message)
}

func (w *PubSubWorker) Run(ctx context.Context) error {
	return w.subscription.Receive(ctx, w.messageHandler)
}

func PubSubWorkerProvider(config config.AppConfig, c consumer.TrcFileConsunmer, client *pubsub.Client) (*PubSubWorker, error) {
	ctx := context.Background()
	subscription := client.Subscription(config.SubscriptionID)
	exists, err := subscription.Exists(ctx)
	if err != nil {
		log.Fatalf("subscription.Exists: %v", err)
	}
	if !exists {
		return nil, fmt.Errorf("subscription %s does not exist", config.SubscriptionID)
	}

	return &PubSubWorker{
		client:         client,
		subscription:   subscription,
		messageHandler: c.ProcessTrcFile,
	}, nil
}

func RunWorker(lc fx.Lifecycle, worker *PubSubWorker) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Printf("Starting Pub/Sub worker")
				worker.Run(context.Background())
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}
