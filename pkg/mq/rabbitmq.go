package mq

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 定义队列名称
const QueueName = "scan_tasks"

// FailOnError 错误处理辅助函数
func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// PublishTask 发送任务到队列 (生产者)
func PublishTask(target string) error {
	// 1. 连接 RabbitMQ (账号密码 guest/guest，端口 5672)
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return err
	}
	defer conn.Close()

	// 2. 创建通道
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// 3. 声明队列 (如果队列不存在会自动创建)
	q, err := ch.QueueDeclare(
		QueueName, // 队列名字
		true,      // durable: 持久化 (重启 MQ 消息还在)
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return err
	}

	// 4. 发送消息
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(target),
		})

	log.Printf(" [x] Sent Task: %s", target)
	return err
}
