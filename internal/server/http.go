package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/stywzn/Go-Cloud-Compute/pkg/mq" // ✅ 引入 MQ 包
)

type HttpServer struct {
	DB  *gorm.DB
	Srv *SentinelServer
}

// NewHttpServer 初始化 HTTP 服务 (标准库版本)
func NewHttpServer(db *gorm.DB, srv *SentinelServer) http.Handler {
	mux := http.NewServeMux()
	server := &HttpServer{DB: db, Srv: srv}

	// 注册路由
	mux.HandleFunc("/task", server.handleTask)     // 发任务接口
	mux.HandleFunc("/health", server.handleHealth) // 健康检查

	// 👇 套上我们写的日志中间件
	return LoggingMiddleware(mux)
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r) // 执行业务逻辑

		duration := time.Since(start)

		// 打印结构化日志: [HTTP] 方法 路径 | 耗时 | IP
		log.Printf("🌐 [HTTP] %s %s | ⏳ %v | 📍 %s",
			r.Method, r.URL.Path, duration, r.RemoteAddr)

		if duration > 500*time.Millisecond {
			log.Printf("⚠️ [Slow Request] 发现慢请求: %s", r.URL.Path)
		}
	})
}

// handleHealth 健康检查
func (s *HttpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// 🔥🔥 核心逻辑：接收 HTTP 请求 -> 发送给 RabbitMQ 🔥🔥
func (s *HttpServer) handleTask(w http.ResponseWriter, r *http.Request) {
	// 1. 只允许 POST 方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. 解析请求 JSON
	// 我们统一定义一个简单的任务结构
	var req struct {
		Type    string `json:"type"`    // 任务类型: shell, python, etc.
		Payload string `json:"payload"` // 具体命令: "echo hello"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: JSON 格式错误", http.StatusBadRequest)
		return
	}

	// 3. (可选) 可以在这里存入 MySQL 做审计记录
	// s.DB.Create(...)

	// 4. 发送到 RabbitMQ
	// ⚠️ 注意：这里不再存入 Srv.JobQueue (内存Map)，那是旧架构
	// 我们直接把 payload 扔进 MQ，让 Agent 自己去抢
	err := mq.Publish(req.Payload)

	if err != nil {
		log.Printf("❌ [MQ] 投递失败: %v", err)
		http.Error(w, "MQ Publish Failed", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [MQ] 任务已进入队列: %s", req.Payload)

	// 5. 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"code": 200, "msg": "任务已派发至 MQ"}`))
}
