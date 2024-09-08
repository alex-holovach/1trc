package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alex-holovach/1trc/backend/src/config"
	"github.com/alex-holovach/1trc/backend/src/model"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/fx"
)

type TrcCntrl struct {
	fx.In

	PubSubClient *pubsub.Client
	RedisClient  *redis.Client
	GcsClient    *storage.Client
	Config       config.AppConfig
}

func (c *TrcCntrl) SetRoute(mux *http.ServeMux) {
	mux.Handle("/trc", otelhttp.NewHandler(http.HandlerFunc(c.TrillionRowChallenge), "Run TRC"))
}

func (c *TrcCntrl) TrillionRowChallenge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := c.GcsClient.Bucket(c.Config.BucketName)
	topic := c.PubSubClient.Topic(c.Config.TopicName)
	processId := uuid.New().String()

	fmt.Printf("Created process %s \n", processId)

	result := model.Response{
		ProcessUuid: processId,
		Status:      "Processing",
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}
	status := c.RedisClient.Set(processId, resultBytes, time.Hour)
	if status.Err() != nil {
		log.Fatal(status.Err())
	}

	files := bucket.Objects(ctx, nil)
	filesCount := 0

	for {
		attrs, err := files.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error iterating through objects: %v", err)
		}

		message := model.TrcPubSubMessage{
			ProcessUuid: processId,
			Filename:    attrs.Name,
		}
		messageBytes, err := json.Marshal(message)
		if err != nil {
			log.Fatal(err)
		}

		res := topic.Publish(ctx, &pubsub.Message{
			Data: messageBytes,
		})
		filesCount++

		_, err = res.Get(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	startTime := time.Now()
	for {
		resultStr, _ := c.RedisClient.Get(processId).Result()
		var responseObj model.Response
		err = json.Unmarshal([]byte(resultStr), &responseObj)
		if err != nil {
			log.Fatal(err)
		}

		if responseObj.ProcessedCount == filesCount {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(responseObj)
			break
		}

		time.Sleep(10 * time.Millisecond)

		if time.Since(startTime) >= 5*time.Second {
			break
		}
	}
}
