package collector

import (
	"context"
	"testing"

	pb "github.com/fortuna/api/proto/agent"
)

type recordingGRPCClient struct {
	registerReq *pb.RegisterAgentRequest
}

func (c *recordingGRPCClient) Connect(context.Context) error   { return nil }
func (c *recordingGRPCClient) Close() error                    { return nil }
func (c *recordingGRPCClient) Reconnect(context.Context) error { return nil }
func (c *recordingGRPCClient) SendSBOMFinding(context.Context, *pb.SBOMFinding) (*pb.SBOMFindingResponse, error) {
	return &pb.SBOMFindingResponse{Success: true}, nil
}
func (c *recordingGRPCClient) SendCombinedFinding(context.Context, *pb.CombinedFinding) (*pb.CombinedFindingResponse, error) {
	return &pb.CombinedFindingResponse{Success: true}, nil
}
func (c *recordingGRPCClient) RegisterAgent(_ context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	c.registerReq = req
	return &pb.RegisterAgentResponse{Success: true, ClusterId: "cluster-a"}, nil
}
func (c *recordingGRPCClient) Ping(context.Context, *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Status: "healthy"}, nil
}
func (c *recordingGRPCClient) StreamInventory(context.Context, interface{}) error { return nil }
func (c *recordingGRPCClient) Heartbeat(context.Context, *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	return &pb.HeartbeatResponse{Success: true}, nil
}

func TestCollectorRegisterUsesConfiguredAgentID(t *testing.T) {
	grpcClient := &recordingGRPCClient{}
	collector, err := NewCollector(nil, grpcClient, "node-a-agent", "cluster-a", "cluster-a")
	if err != nil {
		t.Fatal(err)
	}
	defer collector.cancel()

	if err := collector.register(); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if grpcClient.registerReq == nil {
		t.Fatal("RegisterAgent was not called")
	}
	if got := grpcClient.registerReq.AgentId; got != "node-a-agent" {
		t.Fatalf("RegisterAgent agent_id=%q, want configured identity %q", got, "node-a-agent")
	}
	if collector.agentID != "node-a-agent" {
		t.Fatalf("collector agentID=%q", collector.agentID)
	}
}
