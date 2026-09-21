package grpc

import (
	"context"
	pb "github.com/fortuna/api/proto/agent"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SendCombinedFinding is retired. Only Core may derive CVE matches from a
// workload-owned snapshot received through SendSBOMFinding.
func (s *SBOMServiceServer) SendCombinedFinding(context.Context, *pb.CombinedFinding) (*pb.CombinedFindingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SendCombinedFinding is retired; use SendSBOMFinding")
}
