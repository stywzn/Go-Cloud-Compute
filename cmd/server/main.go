package main

import (
	"log"
	"net"

	pb "github.com/stywzn/Go-Sentinel-Platform/api/proto"
	"github.com/stywzn/Go-Sentinel-Platform/internal/server"
	"google.golang.org/grpc"
)

func main() {
	port := ":9090"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf(" 端口被占用或无法监听: %v", err)
	}

	grpcServer := grpc.NewServer()

	handler := &server.SentinelServer{}
	pb.RegisterSentinelServiceServer(grpcServer, handler)

	log.Printf(" Sentinel Control Plane 已启动 | 监听端口 %s", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf(" 服务器崩溃: %v", err)
	}

}
