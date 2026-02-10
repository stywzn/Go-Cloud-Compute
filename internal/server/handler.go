package server

import (
	"context"
	"log"
	"sync"
	"time"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"gorm.io/gorm"
)

type AgentModel struct {
	gorm.Model
	AgentID  string `gorm:"uniqueIndex;size:191"`
	Hostname string
	IP       string
	Status   string
}

type JobRecord struct {
	gorm.Model
	JobID      string `gorm:"uniqueIndex;size:191"`
	AgentID    string `gorm:"index;size:191"`
	Type       string
	Result     string
	Payload    string
	Status     string
	ExecutedAt time.Time
}

type SentinelServer struct {
	pb.UnimplementedSentinelServiceServer
	DB       *gorm.DB
	JobQueue sync.Map
}

func (s *SentinelServer) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	agentID := req.Hostname

	log.Printf(" [Register] 收到注册请求: %s (%s)", req.Hostname, req.Ip)

	var agent AgentModel
	result := s.DB.Where("agent_id = ?", agentID).First(&agent)

	if result.Error != nil {
		newAgent := AgentModel{
			AgentID:  agentID,
			Hostname: req.Hostname,
			IP:       req.Ip,
			Status:   "online",
		}
		s.DB.Create(&newAgent)
		log.Println(" [DB] 新节点已入库")
	} else {
		agent.Status = "Online"
		agent.IP = req.Ip
		s.DB.Save(&agent)
		log.Println(" [DB] 节点信息已更新")
	}

	return &pb.RegisterResp{
		AgentId: agentID,
		Success: true,
	}, nil
}

func (s *SentinelServer) Heartbeat(stream pb.SentinelService_HeartbeatServer) error {
	for {

		req, err := stream.Recv()

		if err != nil {
			log.Printf(" 接收错误: %v", err)
			return err
		}

		if val, ok := s.JobQueue.LoadAndDelete(req.AgentId); ok {
			job := val.(*pb.Job)
			log.Printf("[Dispatch] 发现信箱有任务! 派发给 %s -> %s", req.AgentId, job.Payload)

			err := stream.Send(&pb.HeartbeatResp{
				Job: job,
			})
			if err != nil {
				return err
			}
		} else {
			stream.Send(&pb.HeartbeatResp{ConfigOutdated: false})
		}
	}
}

func (s *SentinelServer) ReportJobStatus(ctx context.Context, req *pb.ReportJobReq) (*pb.ReportJobResp, error) {

	log.Printf(" [Report] 收到任务汇报! Agent: %s | Job: %s | 状态: %s | 结果: %s",
		req.AgentId, req.JobId, req.Status, req.Result)
	record := JobRecord{
		JobID:      req.JobId,
		AgentID:    req.AgentId,
		Type:       "PING",
		Payload:    "Unknown",
		Result:     req.Result,
		Status:     req.Status,
		ExecutedAt: time.Now(),
	}
	if err := s.DB.Create(&record).Error; err != nil {
		log.Printf("[DB] 保存任务记录失败: %v", err)
	} else {
		log.Printf("[DB] 任务记录已入库 (ID: %d)", record.ID)
	}
	return &pb.ReportJobResp{Received: true}, nil
}
