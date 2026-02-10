package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RunLocalCommand(cmdStr string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("任务超时! (10s limit)\n输出: %s", string(output)), false
		}
		return fmt.Sprintf("执行出错: %v\n输出: %s", err, string(output)), false
	}
	return string(output), true
}

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
		if resp.Job != nil {
			log.Printf(" [接单] 收到任务! ID: %s | 类型: %s | 目标: %s",
				resp.Job.JobId,
				resp.Job.Type,
				resp.Job.Payload,
			)
			go func(j *pb.Job) {

				log.Printf(" [执行中] 正在执行任务 ID: %s", j.JobId)
				output, success := RunLocalCommand(j.Payload)
				log.Printf("[执行结果] \n%s", output)

				reportCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				status := "Success"
				if !success {
					status = "Failed"
				}

				_, err := client.ReportJobStatus(reportCtx, &pb.ReportJobReq{
					AgentId: regResp.AgentId,
					JobId:   j.JobId,
					Status:  status,
					Result:  output,
				})

				if err != nil {
					log.Printf(" 汇报失败: %v", err)
				} else {
					log.Printf(" [汇报成功] 任务结果已发送给 Server")
				}
			}(resp.Job)
		}

	}
}
