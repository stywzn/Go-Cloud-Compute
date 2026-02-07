package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	// 👇 记得改成你的 module 名
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/db"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/mq"
)

// 定义并发数量 (同时扫 5 个 IP)
const WorkerCount = 5

func main() {
	db.InitMySQL()

	// --- 1. 连接 RabbitMQ ---
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "无法连接 RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "无法创建通道")
	defer ch.Close()

	q, err := ch.QueueDeclare(mq.QueueName, true, false, false, false, nil)
	failOnError(err, "无法声明队列")

	// QoS: 预取数量。设置成 WorkerCount * 2，保证每个 Worker 都有活干，但又不会积压太多
	err = ch.Qos(WorkerCount*2, 0, false)
	failOnError(err, "无法设置 QoS")

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	failOnError(err, "无法注册消费者")

	// --- 2. 创建任务通道 (Job Channel) ---
	// 这是一个缓冲通道，用来连接 RabbitMQ 和 Go Workers
	jobs := make(chan string, 100)

	// --- 3. 启动 Worker Pool (关键点) ---
	// 启动 5 个 Goroutine，它们同时在后台跑
	for w := 1; w <= WorkerCount; w++ {
		go worker(w, jobs)
	}

	log.Printf(" [*] 🚀 并发池已启动 (Worker数量: %d)，等待任务...", WorkerCount)

	// --- 4. 主线程：负责从 RabbitMQ 取货，分发给 jobs 通道 ---
	go func() {
		for d := range msgs {
			targetIP := string(d.Body)
			// 把任务扔进通道，空闲的 worker 会抢走
			jobs <- targetIP
		}
		close(jobs)
	}()

	// 阻塞主进程
	select {}
}

// worker 是每个工头的具体工作逻辑
func worker(id int, jobs <-chan string) {
	for targetIP := range jobs {
		log.Printf(" [Worker-%d] 正在处理: %s", id, targetIP)

		// 1. 更新数据库状态 -> RUNNING
		// (为了演示简单，我们这里先省略根据 ID 查 Task 的步骤，直接扫)
		// 实际项目中这里应该传 TaskID 进来

		// 2. 执行扫描
		openPorts := scanPorts(targetIP)

		// 3. 更新数据库 -> FINISHED
		var task model.Task
		// 查找最近一条未完成的任务
		db.DB.Where("target = ? AND status != ?", targetIP, "FINISHED").Last(&task)

		if task.ID != 0 {
			resultsJSON := fmt.Sprintf("%v", openPorts)
			db.DB.Model(&task).Updates(map[string]interface{}{
				"status":  "FINISHED",
				"results": resultsJSON,
			})
			log.Printf(" [Worker-%d] ✅ 完成: %s (ID: %d) -> %s", id, targetIP, task.ID, resultsJSON)
		} else {
			log.Printf(" [Worker-%d] ⚠️ 警告: 数据库没找到对应任务 %s", id, targetIP)
		}
	}
}

// 端口扫描逻辑 (保持不变)
func scanPorts(ip string) []int {
	var openPorts []int
	// 增加一些端口，模拟更真实的扫描
	ports := []int{21, 22, 23, 80, 443, 3306, 6379, 8080, 9000}

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			address := fmt.Sprintf("%s:%d", ip, p)
			conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond) // 超时设短一点
			if err == nil {
				conn.Close()
				mutex.Lock()
				openPorts = append(openPorts, p)
				mutex.Unlock()
			}
		}(port)
	}
	wg.Wait()
	return openPorts
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}
