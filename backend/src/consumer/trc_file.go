package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"runtime"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/alex-holovach/1trc/backend/src/config"
	"github.com/alex-holovach/1trc/backend/src/model"
	"github.com/go-redis/redis"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis"
	"go.uber.org/fx"
)

type TrcFileConsunmer struct {
	fx.In

	PubSubClient *pubsub.Client
	RedisClient  *redis.Client
	GcsClient    *storage.Client
	Config       config.AppConfig
}

func (c *TrcFileConsunmer) ProcessTrcFile(ctx context.Context, message *pubsub.Message) {
	log.Println("Received pub/sub message")
	var trcMessage model.TrcPubSubMessage
	err := json.Unmarshal(message.Data, &trcMessage)
	if err != nil {
		log.Fatal(err)
	}
	bucket := c.GcsClient.Bucket(c.Config.BucketName)
	obj := bucket.Object(trcMessage.Filename)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		log.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to read content: %v", err)
	}

	result := process(content)

	pool := goredis.NewPool(c.RedisClient)
	rs := redsync.New(pool)

	mutexname := "trc-mutex"
	mutex := rs.NewMutex(mutexname)

	if err := mutex.Lock(); err != nil {
		log.Fatal(err)
	}

	processStr, err := c.RedisClient.Get(trcMessage.ProcessUuid).Result()
	var process model.Response
	err = json.Unmarshal([]byte(processStr), &process)
	if err != nil {
		log.Fatal(err)
	}

	mergedMeasurements := mergeMeasurements(process.Result, result)
	process.Result = mergedMeasurements
	process.ProcessedCount++
	processBytes, err := json.Marshal(process)
	if err != nil {
		log.Fatal(err)
	}
	c.RedisClient.Set(trcMessage.ProcessUuid, processBytes, time.Hour)

	fmt.Printf("Successfully processed file - %s, processId - %s \n", trcMessage.Filename, trcMessage.ProcessUuid)
	message.Ack()

	if ok, err := mutex.Unlock(); !ok || err != nil {
		log.Fatal(err)
	}
}

func process(data []byte) map[string]*model.Measurement {
	nChunks := runtime.NumCPU()

	chunkSize := len(data) / nChunks
	if chunkSize == 0 {
		chunkSize = len(data)
	}

	chunks := make([]int, 0, nChunks)
	offset := 0
	for offset < len(data) {
		offset += chunkSize
		if offset >= len(data) {
			chunks = append(chunks, len(data))
			break
		}

		nlPos := bytes.IndexByte(data[offset:], '\n')
		if nlPos == -1 {
			chunks = append(chunks, len(data))
			break
		} else {
			offset += nlPos + 1
			chunks = append(chunks, offset)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(chunks))

	results := make([]map[string]*model.Measurement, len(chunks))
	start := 0
	for i, chunk := range chunks {
		go func(data []byte, i int) {
			results[i] = processChunk(data)
			wg.Done()
		}(data[start:chunk], i)
		start = chunk
	}
	wg.Wait()

	measurements := make(map[string]*model.Measurement)
	for _, r := range results {
		for id, rm := range r {
			m := measurements[id]
			if m == nil {
				measurements[id] = rm
			} else {
				m.Min = min(m.Min, rm.Min)
				m.Max = max(m.Max, rm.Max)
				m.Sum += rm.Sum
				m.Count += rm.Count
			}
		}
	}
	return measurements
}

func processChunk(data []byte) map[string]*model.Measurement {
	const (
		nBuckets = 1 << 12
		maxIds   = 10_000

		fnv1aOffset64 = 14695981039346656037
		fnv1aPrime64  = 1099511628211
	)

	type entry struct {
		key uint64
		mid int
	}
	buckets := make([][]entry, nBuckets)
	measurements := make([]model.Measurement, 0, maxIds)
	ids := make(map[uint64][]byte)

	getMeasurement := func(key uint64) *model.Measurement {
		i := key & uint64(nBuckets-1)
		for j := 0; j < len(buckets[i]); j++ {
			e := &buckets[i][j]
			if e.key == key {
				return &measurements[e.mid]
			}
		}
		return nil
	}

	putMeasurement := func(key uint64, m model.Measurement) {
		i := key & uint64(nBuckets-1)
		buckets[i] = append(buckets[i], entry{key: key, mid: len(measurements)})
		measurements = append(measurements, m)
	}

	for len(data) > 0 {

		idHash := uint64(fnv1aOffset64)
		semiPos := 0
		for i, b := range data {
			if b == ';' {
				semiPos = i
				break
			}

			idHash ^= uint64(b)
			idHash *= fnv1aPrime64
		}

		idData := data[:semiPos]

		data = data[semiPos+1:]

		var temp int64
		{
			negative := data[0] == '-'
			if negative {
				data = data[1:]
			}

			_ = data[3]
			if data[1] == '.' {
				temp = int64(data[0])*10 + int64(data[2]) - '0'*(10+1)
				data = data[4:]
			} else {
				_ = data[4]
				temp = int64(data[0])*100 + int64(data[1])*10 + int64(data[3]) - '0'*(100+10+1)
				data = data[5:]
			}

			if negative {
				temp = -temp
			}
		}

		m := getMeasurement(idHash)
		if m == nil {
			putMeasurement(idHash, model.Measurement{
				Min:   temp,
				Max:   temp,
				Sum:   temp,
				Count: 1,
			})
			ids[idHash] = idData
		} else {
			m.Min = min(m.Min, temp)
			m.Max = max(m.Max, temp)
			m.Sum += temp
			m.Count++
		}
	}

	result := make(map[string]*model.Measurement, len(measurements))
	for _, bucket := range buckets {
		for _, entry := range bucket {
			result[string(ids[entry.key])] = &measurements[entry.mid]
		}
	}
	return result
}

func round(x float64) float64 {
	t := math.Trunc(x)
	if x < 0.0 && t-x == 0.5 {
	} else if math.Abs(x-t) >= 0.5 {
		t += math.Copysign(1, x)
	}

	if t == 0 {
		return 0.0
	}
	return t
}

func mergeMeasurements(map1, map2 map[string]*model.Measurement) map[string]*model.Measurement {
	result := make(map[string]*model.Measurement, len(map1))

	for k, v := range map1 {
		result[k] = v
	}

	for k, v := range map2 {
		if existing, ok := result[k]; ok {
			existing.Min = int64(math.Min(float64(existing.Min), float64(v.Min)))
			existing.Max = int64(math.Max(float64(existing.Max), float64(v.Max)))
			existing.Sum += v.Sum
			existing.Count += v.Count
		} else {
			result[k] = v
		}
	}

	return result
}
