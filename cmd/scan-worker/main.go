package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/streadway/amqp"
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
	"github.com/stywzn/Go-Sentinel-Platform/internal/service"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/config"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/db"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/mq"
)

// MAX_CONCURRENT_JOBS 控制同时有多少个扫描任务在跑
// 防止把机器或者带宽跑崩
const MAX_CONCURRENT_JOBS = 5

func main() {

	config.InitConfig()
	db.Init()
	mq.Init()

	// 设置 QoS (Quality of Service)
	//  RabbitMQ：确认(Ack)前，不要发超过 5 条消息。
	// 保证负载均衡
	err := mq.Channel.Qos(
		MAX_CONCURRENT_JOBS, // prefetch count
		0,                   // prefetch size
		false,               // global
	)
	if err != nil {
		log.Fatal("Failed to set QoS: ", err)
	}

	// 开始消费
	msgs, err := mq.Channel.Consume(
		mq.QueueName, // queue
		"",           // consumer
		false,        // auto-ack (必须手动 ACK)
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.Fatal(err)
	}

	//  优雅退出信号捕获
	// 创建一个监听 SIGINT (Ctrl+C) 和 SIGTERM 的 Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf(" [*] Worker started. Press CTRL+C to exit.")

	// 并发控制 (Semaphore 模式)
	// 使用带缓冲的 channel 作为信号量，限制并发数
	sem := make(chan struct{}, MAX_CONCURRENT_JOBS)
	var wg sync.WaitGroup // 用于等待正在运行的任务完成

	// 主循环
Loop:
	for {
		select {
		case <-ctx.Done():
			// 收到退出信号，跳出循环
			log.Println("Shutdown signal received, stop accepting new tasks...")
			break Loop
		case d, ok := <-msgs:
			if !ok {
				log.Println("RabbitMQ channel closed.")
				break Loop
			}

			// 获取信号量（如果满了，这里会阻塞，直到有空位）
			// 实现了“只有 5 个任务同时跑”
			sem <- struct{}{}
			wg.Add(1)

			// 启动 Goroutine 异步处理
			go func(delivery amqp.Delivery) {
				defer wg.Done()
				defer func() { <-sem }() // 处理完释放信号量

				processTask(delivery)
			}(d)
		}
	}

	// 等待所有正在跑的任务结束
	log.Println("Waiting for active tasks to finish...")
	wg.Wait()
	log.Println("All tasks finished. Goodbye!")
}

// processTask 封装业务逻辑，保持 main 函数干净

func processTask(d amqp.Delivery) {
	// 解析 ID
	taskID, err := strconv.Atoi(string(d.Body))
	if err != nil {
		log.Printf("Error parsing task ID: %v", err)
		d.Reject(false)
		return
	}

	// 查库
	var task model.Task
	if err := db.DB.First(&task, taskID).Error; err != nil {
		log.Printf("[Task %d] Not found in DB, skipping...", taskID)
		d.Reject(false)
		return
	}

	// 更新状态
	db.DB.Model(&task).Update("status", "Running")
	log.Printf("[Task %d] Start Processing %s...", taskID, task.Target)

	users := []string{"root", "admin", "ubuntu", "test", "oracle"}
	passwords := []string{"123456", "password", "12345678", "root", "admin", "ubuntu"}

	// 调用 Pure Go SSH 爆破模块
	user, pass, success := service.SSHBruteForce(task.Target, 22, users, passwords)

	var resultMsg string
	if success {
		resultMsg = fmt.Sprintf("PWNED! SSH Creds -> User: %s, Pass: %s", user, pass)
		log.Printf("[Task %d] %s", taskID, resultMsg)
	} else {
		resultMsg = "Scan Finished. No valid SSH credentials found."
		log.Printf("[Task %d] Failed to crack SSH.", taskID)
	}

	// 保存结果
	db.DB.Model(&task).Updates(map[string]interface{}{
		"status": "Completed",
		"result": resultMsg,
	})

	// 手动确认
	d.Ack(false)
}

func ScanTarget(target string) string {

	ports := []string{"80", "443", "8080", "22", "3306", "6379"}
	var openPorts []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 这里其实也可以加一个超时控制 context，但先不展开太复杂
	for _, port := range ports {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			address := fmt.Sprintf("%s:%s", target, p)
			// 这里的 2秒 超时是关键，防止被防火墙 DROP 导致挂死
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()

	if len(openPorts) == 0 {
		return "No open ports found"
	}
	return fmt.Sprintf("Open Ports: %s", strings.Join(openPorts, ", "))
}
