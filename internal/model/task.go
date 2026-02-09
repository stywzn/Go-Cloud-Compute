package model

import (
	"time"

	"gorm.io/gorm"
)

// Task 对应数据库里的 tasks 表
type Task struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Target string `json:"target"` // 扫描目标
	Status string `json:"status"` // 状态: Pending, Running, Completed

	Result string `json:"result"` // 扫描结果

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
