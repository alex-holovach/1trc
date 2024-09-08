package model

type Measurement struct {
	Min, Max, Sum, Count int64
}

type TrcPubSubMessage struct {
	ProcessUuid string
	Filename    string
}

type Response struct {
	Result         map[string]*Measurement
	ProcessUuid    string
	Status         string
	ProcessedCount int
}
