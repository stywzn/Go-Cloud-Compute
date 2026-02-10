package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"github.com/stywzn/Go-Cloud-Compute/internal/server"
	"github.com/stywzn/Go-Cloud-Compute/pkg/db"
	"github.com/stywzn/Go-Cloud-Compute/pkg/mq"
)

func main() {

	db.InitMySQL()
	mqHost := os.Getenv("MQ_HOST")
	if mqHost == "" {
		mqHost = "localhost"
	}
	rabbit := mq.NewRabbitMQ(mqHost, "job_queue")
	defer rabbit.Close()

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dsn := fmt.Sprintf("root:root@tcp(%s:3306)/cloud_compute?charset=utf8mb4&parseTime=True&loc=Local", dbHost)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf(" 无法连接数据库: %v", err)
	}
	log.Println(" 数据库连接成功!")

	err = db.AutoMigrate(&server.AgentModel{}, &server.JobRecord{})
	if err != nil {
		log.Fatalf(" 自动建表失败: %v", err)
	}
	log.Println("表结构同步完成 (AgentModel + JobRecord)")

	s := grpc.NewServer()
	srv := &server.SentinelServer{DB: db}
	pb.RegisterSentinelServiceServer(s, srv)

	go func() {
		httpSrv := server.NewHttpServer(db, srv)
		log.Println("HTTP Management API 已启动 | 监听端口 :8080")
		httpSrv.Start()
	}()

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("端口监听失败: %v", err)
	}
	log.Println("Sentinel Control Plane 已启动 | 监听端口 :9090")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC 服务启动失败: %v", err)
	}
}
