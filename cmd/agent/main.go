package main

import (
	"context"
	"fmt"
	"log"
	"net" // 👈 新增：网络包
	"os"
	"os/exec"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
)

// RunLocalCommand 执行本地 Shell 命令 (没变)
func RunLocalCommand(cmdStr string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("❌ 任务超时! (10s limit)\n输出: %s", string(output)), false
		}
		return fmt.Sprintf("❌ 执行出错: %v\n输出: %s", err, string(output)), false
	}
	return string(output), true
}

func main() {
	// 1. 读取环境变量
	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "127.0.0.1:9090"
	}

	log.Printf("🔌 准备连接 Server 地址: %s", serverAddr)

	// 👇👇👇 核心修改开始 👇👇👇
	// 自定义拨号器：强制使用 "tcp4" (IPv4)，彻底屏蔽 IPv6 问题
	customDialer := func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		// 关键点：这里写的是 "tcp4"，不是 "tcp"
		return d.DialContext(ctx, "tcp4", addr)
	}

	// 连接选项
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(customDialer), // 👈 注入我们的强制 IPv4 拨号器
	}

	// 建立连接
	conn, err := grpc.NewClient(serverAddr, opts...)
	// 👆👆👆 核心修改结束 👆👆👆

	if err != nil {
		log.Fatalf("无法连接服务器: %v", err)
	}
	defer conn.Close()

	client := pb.NewSentinelServiceClient(conn)

	// ... (后面的代码完全没变) ...
	// 注册 Agent
	hostname, _ := os.Hostname()
	ip := "Unknown"

	// 循环发心跳
	for {
		// 1. 发起注册
		log.Printf("Agent [%s] 正在向控制面注册...", hostname)
		regResp, err := client.Register(context.Background(), &pb.RegisterReq{
			Hostname: hostname,
			Ip:       ip,
		})

		if err != nil {
			log.Printf("⚠️ 注册失败: %v", err)
			time.Sleep(2 * time.Second) // 失败了等 2 秒重试
			continue
		}

		log.Printf("✅ 注册成功! ID: %s", regResp.AgentId)

		// 2. 建立心跳流
		stream, err := client.Heartbeat(context.Background())
		if err != nil {
			log.Printf("❌ 建立心跳流失败: %v", err)
			continue
		}

		// 3. 开始收发心跳
		waitc := make(chan struct{})

		// 发送协程
		go func() {
			for {
				err := stream.Send(&pb.HeartbeatReq{AgentId: regResp.AgentId})
				if err != nil {
					log.Printf("❌ 心跳发送失败: %v", err)
					close(waitc)
					return
				}
				time.Sleep(5 * time.Second)
			}
		}()

		// 接收协程 (接收任务)
		go func() {
			for {
				resp, err := stream.Recv()
				if err != nil {
					log.Printf("❌ 心跳接收断开: %v", err)
					close(waitc)
					return
				}

				if resp.Job != nil {
					// ⚡️ 收到任务，开启协程去干活
					go func(j *pb.Job) {
						log.Printf("⚙️ [执行中] 正在执行任务: %s", j.Payload)

						output, success := RunLocalCommand(j.Payload)
						log.Printf("📄 [执行结果] \n%s", output)

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
							log.Printf("❌ 汇报失败: %v", err)
						} else {
							log.Printf("✅ [汇报成功] 结果已上传")
						}
					}(resp.Job)
				}
			}
		}()

		<-waitc
		log.Println("🔌 连接断开，3秒后重连...")
		time.Sleep(3 * time.Second)
	}
}
