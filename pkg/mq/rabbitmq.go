package mq

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"github.com/streadway/amqp"
)

var Conn *amqp.Connection
var Channel *amqp.Channel
var QueueName string

func Init() {
	// 1. 读取配置
	// 建议：生产环境这里应该加个默认值兜底，或者检查配置是否存在
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		viper.GetString("rabbitmq.user"),
		viper.GetString("rabbitmq.password"),
		viper.GetString("rabbitmq.host"),
		viper.GetString("rabbitmq.port"),
	)

	var err error
	// 建立连接
	Conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatalf("❌ Failed to connect to RabbitMQ: %v", err)
	}

	// 建立通道
	Channel, err = Conn.Channel()
	if err != nil {
		log.Fatalf("❌ Failed to open a channel: %v", err)
	}

	QueueName = viper.GetString("rabbitmq.queue_name")

	// 2. 声明队列 (即使队列已存在也没关系，确保属性一致)
	_, err = Channel.QueueDeclare(
		QueueName, // name
		true,      // durable (持久化：MQ 重启后队列还在)
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("❌ Failed to declare a queue: %v", err)
	}

	// 👇👇👇【新增关键点 1】设置 QoS (公平分发) 👇👇👇
	// prefetchCount = 1: 告诉 MQ，在我 Ack 之前，最多只给我发 1 条消息。
	// 这样能保证能者多劳，不会让处理慢的 Agent 堆积任务。
	err = Channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Fatalf("❌ Failed to set QoS: %v", err)
	}

	log.Println("✅ RabbitMQ connected (QoS=1).")
}

func Publish(body string) error {
	// 消息持久化 (Persistent)
	// 只有队列持久化 + 消息持久化，MQ 挂了数据才不丢
	return Channel.Publish(
		"",        // exchange
		QueueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent, // 👈 记得加上这个，消息持久化
			Body:         []byte(body),
		})
}

// 👇👇👇【新增关键点 2】封装消费者方法 👇👇👇
// 返回一个只读通道，让 Agent 去 range 遍历
func Consume() (<-chan amqp.Delivery, error) {
	msgs, err := Channel.Consume(
		QueueName, // queue
		"",        // consumer
		false,     // 👈 auto-ack = false (关键！必须手动 Ack)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	return msgs, err
}
