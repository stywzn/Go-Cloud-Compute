package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal" // 👈 引入信号包
	"sync"      // 👈 引入等待组
	"syscall"   // 👈 引入系统调用
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
)

// RunLocalCommand (保持不变)
func RunLocalCommand(cmdStr string) (string, bool) {
	// ... (代码略) ...
	// 为了演示，这里假设执行逻辑没变
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
	// ... (环境变量读取、Dialer 设置代码保持不变) ...

	// 👇👇👇 核心修改 1: 定义优雅退出的信号通道 👇👇👇
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 👇👇👇 核心修改 2: 定义 WaitGroup 追踪任务 👇👇👇
	var wg sync.WaitGroup

	// 这里需要一个 context 来控制连接的生命周期
	ctx, cancel := context.WithCancel(context.Background())

	// 监听信号的协程
	go func() {
		sig := <-quit
		log.Printf("🛑 收到信号 [%s]，准备优雅退出...", sig)
		log.Println("🚫 停止接收新任务，等待正在执行的任务结束...")
		cancel() // 通知主循环停止
	}()

	// 主循环 (被 ctx 控制)
	for {
		// 如果已经收到了退出信号，就跳出大循环
		if ctx.Err() != nil {
			break
		}

		// ... (连接 gRPC 代码保持不变) ...
		serverAddr := os.Getenv("SERVER_ADDR")
		if serverAddr == "" {
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
			log.Printf("无法连接服务器: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		client := pb.NewSentinelServiceClient(conn)
		// ... (注册逻辑略) ...
		// 假设注册成功拿到 agentID
		agentID := "test-agent-id"

		stream, err := client.Heartbeat(context.Background())
		if err != nil {
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		// 3. 开始收发心跳
		waitc := make(chan struct{})

		// 发送心跳 (增加 ctx 退出控制)
		go func() {
			defer close(waitc) // 任何情况退出都要关闭通道
			for {
				select {
				case <-ctx.Done(): // 收到退出信号
					return
				default:
					err := stream.Send(&pb.HeartbeatReq{AgentId: agentID})
					if err != nil {
						log.Printf("❌ 心跳发送失败: %v", err)
						return
					}
					time.Sleep(5 * time.Second)
				}
			}
		}()

		// 接收任务 (主线程阻塞在这里)
		// 我们把接收逻辑放在主线程或者受控的循环里
		go func() {
			for {
				// 如果正在退出，就断开接收
				if ctx.Err() != nil {
					return
				}

				resp, err := stream.Recv()
				if err != nil {
					log.Printf("❌ 连接断开: %v", err)
					// 这里不能直接 close waitc，因为由发送协程管理
					return
				}

				if resp.Job != nil {
					// 👇👇👇 核心修改 3: 任务计数加 1 👇👇👇
					wg.Add(1)

					go func(j *pb.Job) {
						// 👇👇👇 核心修改 4: 任务结束减 1 👇👇👇
						defer wg.Done()

						log.Printf("⚙️ [执行中] 任务: %s", j.Payload)
						output, success := RunLocalCommand(j.Payload)

						// 汇报结果 (超时控制)
						reportCtx, rCancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer rCancel()

						status := "Success"
						if !success {
							status = "Failed"
						}
						_, err := client.ReportJobStatus(reportCtx, &pb.ReportJobReq{
							AgentId: agentID,
							JobId:   j.JobId,
							Status:  status,
							Result:  output,
						})
						if err != nil {
							log.Printf("❌ 汇报失败: %v", err)
						}
					}(resp.Job)
				}
			}
		}()

		// 等待心跳断开 或者 收到退出信号
		select {
		case <-waitc:
			log.Println("🔌 连接断开，3秒后重连...")
		case <-ctx.Done():
			log.Println("🛑 主循环停止连接...")
		}

		conn.Close() // 关闭连接

		if ctx.Err() != nil {
			break // 彻底退出大循环
		}
		time.Sleep(3 * time.Second)
	}

	// 👇👇👇 核心修改 5: 阻塞等待所有任务跑完 👇👇👇
	log.Println("⏳ 等待所有后台任务完成...")
	wg.Wait()
	log.Println("👋 Agent 安全退出")
}
