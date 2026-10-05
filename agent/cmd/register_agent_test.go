package main

import (
	"context"
	"testing"

	pb "github.com/fortuna/api/proto/agent"

	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/internal/config"
)

type recordingGRPCClient struct {
	client.GRPCClient
	registerReq *pb.RegisterAgentRequest
}

func (c *recordingGRPCClient) RegisterAgent(_ context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	c.registerReq = req
	return &pb.RegisterAgentResponse{Success: true, ClusterId: "cluster-a"}, nil
}

func TestRegisterAgentUsesConfiguredAgentID(t *testing.T) {
	grpcClient := &recordingGRPCClient{}
	cfg := &config.Config{AgentID: "node-a-agent", NodeID: "node-a", NodeName: "node-a"}
	if err := registerAgent(context.Background(), grpcClient, cfg); err != nil {
		t.Fatal(err)
	}
	if grpcClient.registerReq == nil {
		t.Fatal("RegisterAgent was not called")
	}
	if got := grpcClient.registerReq.AgentId; got != "node-a-agent" {
		t.Fatalf("AgentId = %q, want configured Agent ID", got)
	}
	if got := grpcClient.registerReq.NodeName; got != "node-a" {
		t.Fatalf("NodeName = %q, want node-a", got)
	}
}
