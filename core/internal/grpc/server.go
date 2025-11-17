package grpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/ksam/core/internal/config"
	"gorm.io/gorm"
)

type Server struct {
	config *config.Config
	db     *gorm.DB
	server *grpc.Server
}

func NewServer(cfg *config.Config, db *gorm.DB) *Server {
	grpcServer := grpc.NewServer()
	
	// Register gRPC service
	// TODO: After generating proto files, uncomment:
	// serviceServer := NewKSAMServiceServer(db)
	// pb.RegisterKSAMServiceServer(grpcServer, serviceServer)
	_ = NewKSAMServiceServer(db) // Keep for future use
	
	return &Server{
		config: cfg,
		db:     db,
		server: grpcServer,
	}
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", ":"+s.config.GRPCPort)
	if err != nil {
		return err
	}

	fmt.Printf("Starting gRPC server on port %s\n", s.config.GRPCPort)
	return s.server.Serve(lis)
}

func (s *Server) Stop() {
	s.server.GracefulStop()
}

