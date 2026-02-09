package main

import (
	"context"
	"log"
	"os"
	"time"

	pb "github.com/stywzn/Go-Sentinel-Platform/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("无法连接 Server: %v", err)
	}
	defer conn.Close()

	client := pb.NewSentinelServiceClient(conn)

	hostname, _ := os.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Agent [%s] 正在向控制面注册...", hostname)

	regResp, err := client.Register(ctx, &pb.RegisterReq{
		Hostname: hostname,
		Ip:       "127.0.0.1",
		Tags:     []string{"dev", "test-node"},
	})
	if err != nil {
		log.Fatalf(" 注册失败: %v", err)
	}

	log.Println(" 注册成功! ID: %s", regResp.AgentId)

	stream, err := client.Heartbeat(context.Background())
	if err != nil {
		log.Fatal("heart beat failed: %v", err)
	}

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			err := stream.Send(&pb.HeartbeatReq{
				AgentId:   regResp.AgentId,
				Timestamp: time.Now().Unix(),
				CpuUsage:  15.5,
			})
			if err != nil {
				log.Printf(" 心跳发送失败： %v", err)
				return
			}
			log.Printf("send the heartbeat...")
		}
	}()

	for {
		resp, err := stream.Recv()
		if err != nil {
			log.Fatal("与 Server 断开连接: %v", err)
		}
		if resp.ConfigOutdated {
			log.Println(" 收到指令: 配置已过期，正在拉取新配置...")
		}
	}
}
