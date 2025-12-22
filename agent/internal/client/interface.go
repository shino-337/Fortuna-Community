package client

import (
	"context"

	fortuna "github.com/ksam/agent/proto/gen/proto"
)

// GRPCClient interface for gRPC communication with core
type GRPCClient interface {
	Register(ctx context.Context, req *fortuna.RegisterRequest) (*fortuna.RegisterResponse, error)
	StreamInventory(ctx context.Context, items []*fortuna.InventoryItem) error
	Heartbeat(ctx context.Context, req *fortuna.RegisterRequest) error
	Close() error
}

