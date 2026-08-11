package main

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/castai/logging/components"
)

type registrator struct {
	once    sync.Once
	blockCh chan struct{}
}

func newRegistrator() *registrator {
	return &registrator{blockCh: make(chan struct{})}
}

func (r *registrator) releaseWaiters() {
	r.once.Do(func() { close(r.blockCh) })
}

func (r *registrator) waitUntilRegistered() {
	<-r.blockCh
}

type exportHook struct {
	batchClient *components.BatchClient
	cancel      context.CancelFunc
	done        chan struct{}
}

func (h *exportHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *exportHook) Fire(entry *logrus.Entry) error {
	fields := make(map[string]string, len(entry.Data))
	for k, v := range entry.Data {
		fields[k] = toString(v)
	}
	return h.batchClient.IngestLogs(context.Background(), []components.Entry{{
		Level:   entry.Level.String(),
		Message: entry.Message,
		Time:    entry.Time,
		Fields:  fields,
	}})
}

func (h *exportHook) Wait() {
	h.cancel()
	<-h.done
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func main() {
	apiClient, err := components.NewAPIClient(components.Config{
		APIBaseURL: "http://localhost:8090",
		APIKey:     "test",
		ClusterID:  "exp",
		Component:  "logrus-exporter",
		Version:    "v0",
	})
	if err != nil {
		panic(err)
	}

	batchClient := components.NewBatchClient(apiClient,
		components.FlushInterval(30*time.Second),
		components.BatchSize(1000),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = batchClient.Run(ctx)
	}()

	reg := newRegistrator()
	reg.releaseWaiters()
	reg.waitUntilRegistered()

	hook := &exportHook{batchClient: batchClient, cancel: cancel, done: done}

	log := logrus.New()
	log.AddHook(hook)
	logrus.RegisterExitHandler(hook.Wait)

	for i := 1; i <= 5; i++ {
		log.Infof("logrus-exporter message %d", i)
	}

	log.Fatal("logrus-exporter fatal shutdown")
}
