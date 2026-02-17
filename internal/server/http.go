package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"gorm.io/gorm"
)

type HttpServer struct {
	DB  *gorm.DB
	Srv *SentinelServer
}

type JobRequest struct {
	TargetAgent string `json:"target"`
	Cmd         string `json:"cmd"`
}

func NewHttpServer(db *gorm.DB, srv *SentinelServer) http.Handler { // 👈 注意：这里返回值改成 http.Handler
	mux := http.NewServeMux()
	server := &HttpServer{DB: db, Srv: srv}

	// 注册路由
	mux.HandleFunc("/task", server.handleTask)     // 假设你有这个接口
	mux.HandleFunc("/health", server.handleHealth) // 健康检查

	// 👇👇👇 关键：套上日志中间件 👇👇👇
	return LoggingMiddleware(mux)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // 1. 掐表开始

		// 包装 ResponseWriter 以捕获状态码 (可选，稍微复杂点，这里先略过，只记耗时)
		next.ServeHTTP(w, r) // 2. 执行业务逻辑

		duration := time.Since(start) // 3. 掐表结束

		// 4. 打印带指标的日志
		// [协议] [方法] [路径] [耗时] [客户端IP]
		// 例子: [HTTP] POST /task | 12.5ms | 192.168.1.5
		log.Printf("🌐 [HTTP] %s %s | ⏳ %v | 📍 %s",
			r.Method,
			r.URL.Path,
			duration,
			r.RemoteAddr,
		)

		// 进阶思考：如果 duration > 500ms，可以打印一条 WARN 日志
		if duration > 500*time.Millisecond {
			log.Printf("⚠️ [Slow Request] 发现慢请求: %s", r.URL.Path)
		}
	})
}

func (h *HttpServer) Start() {
	r := gin.Default()

	r.GET("/agent", func(c *gin.Context) {
		var agents []AgentModel
		h.DB.Find(&agents)
		c.JSON(200, gin.H{"code": 200, "data": agents})
	})

	r.POST("/job", func(c *gin.Context) {
		var req JobRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "JSON 格式不对"})
			return
		}

		jobID := fmt.Sprintf("manual-%s-%d", req.TargetAgent, time.Now().UnixNano())

		job := &pb.Job{
			JobId:   jobID,
			Type:    pb.JobType_PING,
			Payload: req.Cmd,
		}
		h.Srv.JobQueue.Store(req.TargetAgent, job)
		log.Printf("[HTTP] 管理员下发任务 -> %s : %s", req.TargetAgent, req.Cmd)

		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "任务已进入队列，等待 Agent 心跳领取",
			"job":  jobID,
		})
	})
	r.Run(":8080")
}
