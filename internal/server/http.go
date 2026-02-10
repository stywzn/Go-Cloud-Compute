package server

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"gorm.io/gorm"
)

type HttpServer struct {
	DB  *gorm.DB
	Srv *SentinelServer
}

func NewHttpServer(db *gorm.DB, srv *SentinelServer) *HttpServer {
	return &HttpServer{
		DB:  db,
		Srv: srv,
	}
}

type JobRequest struct {
	TargetAgent string `json:"target"`
	Cmd         string `json:"cmd"`
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
