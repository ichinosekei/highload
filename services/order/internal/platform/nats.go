package platform

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsClient struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

func NewNatsClient(url string) (*NatsClient, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connection: %w", err)
	}

	js, errJS := jetstream.New(nc)
	if errJS != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream initialization: %w", errJS)
	}

	return &NatsClient{
		Conn: nc,
		JS:   js,
	}, nil
}

func (n *NatsClient) Close() {
	n.Conn.Close()
}

// EnsureStream ensures that the stream exists.
func (n *NatsClient) EnsureStream(ctx context.Context, name string, subjects []string) error {
	_, err := n.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
	})
	if err != nil {
		return fmt.Errorf("ensure stream %s: %w", name, err)
	}
	return nil
}
