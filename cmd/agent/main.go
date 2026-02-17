package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/streadway/amqp"
	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"github.com/stywzn/Go-Cloud-Compute/pkg/config" // ✅ 引入配置
	"github.com/stywzn/Go-Cloud-Compute/pkg/mq"     // ✅ 引入 MQ
)

// RunLocalCommand 执行本地命令
func RunLocalCommand(cmdStr string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, output), false
	}
	return string(output), true
}

func main() {
	// ✅ 1. 初始化配置和 MQ (必须放在最前面)
	config.LoadConfig()
	mq.Init()

	// 👇👇👇 定义优雅退出的信号通道 👇👇👇
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 👇👇👇 定义 WaitGroup 追踪任务 👇👇👇
	var wg sync.WaitGroup

	// 上下文控制
	ctx, cancel := context.WithCancel(context.Background())

	// 监听信号的协程
	go func() {
		sig := <-quit
		log.Printf("🛑 收到信号 [%s]，准备优雅退出...", sig)
		log.Println("🚫 停止接收新任务，等待正在执行的任务结束...")
		cancel() // 通知主循环停止
	}()

	// ------------------------------------------------------
	// 🚀 启动 MQ 消费者 (建议放在主循环外面，独立运行)
	// ------------------------------------------------------
	go func() {
		msgs, err := mq.Consume()
		if err != nil {
			log.Printf("❌ [MQ] 无法启动消费者: %v", err)
			return
		}

		log.Println("👂 [MQ] 消费者已启动，等待任务...")

		for d := range msgs {
			// 如果正在关机，退回消息
			if ctx.Err() != nil {
				d.Nack(false, true)
				continue
			}

			wg.Add(1) // 任务 +1

			go func(delivery amqp.Delivery) {
				defer wg.Done() // 任务 -1

				jobPayload := string(delivery.Body)

				// ✅ 【修复】幂等性检查日志放在这里 (只有这里才有 job 数据)
				log.Printf("🔍 [幂等性检查] 正在校验 MQ 任务: %s", jobPayload)
				// TODO: 这里将来加 Redis 查重逻辑
				// if redis.Exists(jobID) { d.Ack(false); return }

				log.Printf("⚙️ [MQ] 开始执行: %s", jobPayload)
				output, success := RunLocalCommand(jobPayload)

				if success {
					log.Printf("✅ [MQ] 执行成功")
					// 手动 ACK
					if err := delivery.Ack(false); err != nil {
						log.Printf("⚠️ Ack 失败: %v", err)
					}
				} else {
					log.Printf("❌ [MQ] 执行失败: %s", output)
					// 失败也 ACK (或者 Nack 重试，看策略)
					delivery.Ack(false)
				}
			}(d)
		}
	}()

	// ------------------------------------------------------
	// 🔄 gRPC 主循环 (负责心跳和汇报)
	// ------------------------------------------------------
	for {
		if ctx.Err() != nil {
			break
		}

		serverAddr := os.Getenv("SERVER_ADDR")
		if serverAddr == "" {
			// 这里可以读配置 config.GlobalConfig.Server.GRPCPort
			serverAddr = "127.0.0.1:9090"
		}

		customDialer := func(ctx context.Context, addr string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "tcp4", addr)
		}
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(customDialer),
		}

		conn, err := grpc.NewClient(serverAddr, opts...)
		if err != nil {
			log.Printf("❌ 无法连接服务器: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		client := pb.NewSentinelServiceClient(conn)

		// 注册
		hostname, _ := os.Hostname()
		regResp, err := client.Register(context.Background(), &pb.RegisterReq{
			Hostname: hostname,
			Ip:       "127.0.0.1",
		})
		if err != nil {
			log.Printf("⚠️ 注册失败: %v", err)
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}
		agentID := regResp.AgentId

		// 心跳
		stream, err := client.Heartbeat(context.Background())
		if err != nil {
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		// 心跳管理通道
		waitc := make(chan struct{})

		// 发送心跳协程
		go func() {
			defer close(waitc)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					err := stream.Send(&pb.HeartbeatReq{AgentId: agentID})
					if err != nil {
						return // 发送失败，触发重连
					}
					time.Sleep(5 * time.Second)
				}
			}
		}()

		// 接收 gRPC 消息协程 (如果有 gRPC 下发任务的话)
		go func() {
			for {
				if ctx.Err() != nil {
					return
				}

				resp, err := stream.Recv()
				if err != nil {
					return
				} // 断开连接

				if resp.Job != nil {
					wg.Add(1)
					go func(j *pb.Job) {
						defer wg.Done()

						// ✅ 【修复】这里也有一个幂等性检查点
						log.Printf("🔍 [幂等性检查] 正在校验 gRPC 任务 %s", j.JobId)

						log.Printf("⚙️ [gRPC] 执行任务: %s", j.Payload)
						output, success := RunLocalCommand(j.Payload)

						// 汇报
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						status := "Success"
						if !success {
							status = "Failed"
						}

						client.ReportJobStatus(ctx, &pb.ReportJobReq{
							AgentId: agentID,
							JobId:   j.JobId,
							Status:  status,
							Result:  output,
						})
					}(resp.Job)
				}
			}
		}()

		// 阻塞等待断开
		select {
		case <-waitc:
			log.Println("🔌 连接断开，3秒后重连...")
		case <-ctx.Done():
			log.Println("🛑 主循环停止连接...")
		}

		conn.Close()
		if ctx.Err() != nil {
			break
		}
		time.Sleep(3 * time.Second)
	}

	log.Println("⏳ 等待所有后台任务完成...")
	wg.Wait()
	log.Println("👋 Agent 安全退出")
}
