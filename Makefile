.PHONY: all build build-server build-agent run-server run-agent proto clean docker-up docker-down

# 默认动作：输入 make 就执行 build
all: build


build: build-server build-agent

build-server:
	@echo " Building Server..."
	@mkdir -p bin
	@go build -o bin/server cmd/server/main.go
	@echo " Server built at bin/server"

build-agent:
	@echo " Building Agent..."
	@mkdir -p bin
	@go build -o bin/agent cmd/agent/main.go
	@echo "Agent built at bin/agent"


# 运行 

run-server:
	@echo " Running Server..."
	@go run cmd/server/main.go

run-agent:
	@echo " Running Agent..."
	@go run cmd/agent/main.go


# 辅助工具 (Utils)

# 生成 gRPC 代码 (前提是你装了 protoc)
proto:
	@protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	api/proto/sentinel.proto
	@echo "✅ Protobuf code generated."

# 整理依赖
tidy:
	@go mod tidy

# 清理构建产物
clean:
	@rm -rf bin/
	@echo " Cleaned."


# Docker 快捷指令

up:
	@docker-compose up -d

down:
	@docker-compose down