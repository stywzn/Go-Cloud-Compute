package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http" // 👈 引入标准 http 包
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"github.com/stywzn/Go-Cloud-Compute/internal/server"
)

func main() {
	// 1. 配置加载 (建议以后用 viper，现在先用 env 顶一下)
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "root"
	}
	dbPass := os.Getenv("DB_PASS")
	if dbPass == "" {
		dbPass = "root"
	} // 👈 密码别写死，从环境变量读

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/cloud_compute?charset=utf8mb4&parseTime=True&loc=Local", dbUser, dbPass, dbHost)

	// 2. 数据库初始化
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 无法连接数据库: %v", err)
	}
	log.Println("✅ 数据库连接成功!")

	if err := db.AutoMigrate(&server.AgentModel{}, &server.JobRecord{}); err != nil {
		log.Fatalf("❌ 自动建表失败: %v", err)
	}

	// 3. 准备 gRPC 服务
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("❌ gRPC 端口监听失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	srv := &server.SentinelServer{DB: db}
	pb.RegisterSentinelServiceServer(grpcServer, srv)

	// 4. 准备 HTTP 服务
	// 注意：这里我们需要拿到原生 http.Server 对象，以便后面执行 Shutdown
	// 假设 server.NewHttpServer 返回的是一个 http.Handler (如 Gin Engine)
	httpHandler := server.NewHttpServer(db, srv)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: httpHandler, // 这里假设你的 NewHttpServer 返回的是 *gin.Engine 或者实现了 ServeHTTP 的结构
	}

	// ---------------------------------------------------------
	// 🚀 启动阶段 (全部放入协程，不阻塞主线程)
	// ---------------------------------------------------------

	// 启动 gRPC
	go func() {
		log.Println("🚀 Sentinel Control Plane 已启动 | gRPC :9090")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("❌ gRPC 服务崩溃: %v", err)
		}
	}()

	// 启动 HTTP
	go func() {
		log.Println("🚀 HTTP Management API 已启动 | HTTP :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP 服务崩溃: %v", err)
		}
	}()

	// ---------------------------------------------------------
	// 🛑 优雅退出阶段 (面试加分项)
	// ---------------------------------------------------------

	// 1. 监听信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 2. 阻塞直到收到信号
	sig := <-quit
	log.Printf("🛑 收到信号 [%s]，开始优雅停机...", sig)

	// 3. 创建超时上下文 (给程序 10秒 时间善后，超时强制杀)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 4. 先关 HTTP (入口网关)：停止接收新用户的任务
	log.Println("⏳ 正在停止 HTTP 服务...")
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️ HTTP 关闭报错: %v", err)
	} else {
		log.Println("✅ HTTP 服务已安全停止")
	}

	// 5. 再关 gRPC (内部通信)：停止接收 Agent 汇报
	// GracefulStop 会等待当前正在处理的 RPC 请求结束
	log.Println("⏳ 正在停止 gRPC 服务...")
	grpcServer.GracefulStop()
	log.Println("✅ gRPC 服务已安全停止")

	// 6. (可选) 关闭数据库连接
	sqlDB, _ := db.DB()
	sqlDB.Close()
	log.Println("✅ 数据库连接已关闭")

	log.Println("👋 Server 安全退出")
}
