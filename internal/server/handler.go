package server

import (
	"context"
	"io"
	"log"

	"github.com/google/uuid"
	pb "github.com/stywzn/Go-Sentinel-Platform/api/proto"
)

type SentinelServer struct {
	pb.UnimplementedSentinelServiceServer
}

func (s *SentinelServer) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	agentID := uuid.New().String()

	log.Printf("[Register] 新节点加入 Host: %s, IP: %s, ID: %s", req.Hostname, req.Ip, agentID)

	return &pb.RegisterResp{
		AgentId: agentID,
		Success: true,
	}, nil
}

func (s *SentinelServer) Heartbeat(stream pb.SentinelService_HeartbeatServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Println(" Agent 断开连接")
			return nil
		}
		if err != nil {
			log.Printf(" 接收错误: %v", err)
			return err
		}

		log.Printf("[Heartbeat] ID: %s | CPU: %.1f%%", req.AgentId, req.CpuUsage)

		err = stream.Send(&pb.HeartbeatResp{
			ConfigOutdated: false,
		})
		if err != nil {
			log.Printf(" 发送失败: %v", err)
			return err
		}
	}
}
