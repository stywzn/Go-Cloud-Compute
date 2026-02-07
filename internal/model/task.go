package model

import (
	"gorm.io/gorm"
)

type Task struct {
	gorm.Model
	Target string `json:"target"`
	Status string `json:"status"` // PENDING, RUNNING, FINISHED
	// 👇 新增这个字段，用来存 "[80, 443]" 这样的字符串
	Results string `json:"results"`
}
