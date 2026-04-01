package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/milan604/core-lab/pkg/jobs"
	"github.com/milan604/core-lab/pkg/server"
)

type emailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
}

func main() {
	manager, err := jobs.NewManager(jobs.Config{
		Name:        "jobs-example",
		Workers:     2,
		QueueBuffer: 32,
	}, jobs.NewMemoryStore())
	if err != nil {
		panic(err)
	}

	if err := manager.RegisterHandler("email.send", func(ctx context.Context, job jobs.Job) (any, error) {
		var payload emailPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return nil, err
		}

		time.Sleep(250 * time.Millisecond)
		return map[string]any{
			"delivered": true,
			"to":        payload.To,
			"subject":   payload.Subject,
		}, nil
	}, jobs.WithHandlerDescription("Simulate sending an email")); err != nil {
		panic(err)
	}

	if err := manager.Start(context.Background()); err != nil {
		panic(err)
	}
	defer manager.Stop(context.Background())

	if _, err := manager.Enqueue(context.Background(), jobs.EnqueueRequest{
		Type:    "email.send",
		Payload: json.RawMessage(`{"to":"demo@example.com","subject":"Background jobs are live"}`),
	}); err != nil {
		panic(err)
	}

	engine := jobs.NewAdminEngine(manager)
	if err := server.Start(engine, server.StartWithAddr(":8091")); err != nil {
		panic(err)
	}
}
