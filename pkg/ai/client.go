package ai

import (
	"fmt"
	"time"
)

func AnalyzeHealthy(cpu, mem float64) string {

	time.Sleep(2 * time.Second)

	if cpu > 80 || mem > 80 {
		return fmt.Sprintf("[AI Analysis] 警告：服务器负载过高 (CPU:%.1f%%, MEM:%.1f%%)。建议：1. 检查是否有死循环进程；2. 考虑水平扩容 (HPA)。", cpu, mem)
	}

	return fmt.Sprintf("[AI Analysis] 系统运行健康 (CPU:%.1f%%)，继续保持。", cpu)
}
