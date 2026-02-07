package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	// 👇 改成你的 module 名
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/db"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/mq"
)

type ScanRequest struct {
	Target string `json:"target" binding:"required"`
}

func main() {
	// 1. 初始化数据库
	db.InitMySQL()

	r := gin.Default()

	// 2. 提交任务接口
	r.POST("/api/scan", func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		newTask := model.Task{
			Target: req.Target,
			Status: "PENDING",
		}
		db.DB.Create(&newTask)

		// 发送消息到 RabbitMQ
		err := mq.PublishTask(newTask.Target)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "任务入队失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "任务已提交",
			"task_id": newTask.ID,
		})
	})

	// 3. 👇 [新增] 查询任务详情接口
	r.GET("/api/task", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供 id 参数"})
			return
		}

		var task model.Task
		// 根据 ID 查数据库
		if err := db.DB.First(&task, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": task, // 这里会自动包含 Results 字段
		})
	})

	r.Run(":8080")
}
