FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

# 编译两个二进制文件
# CGO_ENABLED=0 表示静态编译，不需要依赖系统库
RUN CGO_ENABLED=0 GOOS=linux go build -o api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -o scan-worker ./cmd/scan-worker

# ----------------------------------------------------

# 运行 (Runner)
# 使用极小的 Alpine 镜像 (只有 5MB)
FROM alpine:latest

WORKDIR /root/

# 从构建阶段把编译好的文件拿过来
COPY --from=builder /app/api-server .
COPY --from=builder /app/scan-worker .
COPY --from=builder /app/config.yaml .

# 暴露端口
EXPOSE 8080

# 默认运行 api-server (可以在 docker-compose 里覆盖)
CMD ["./api-server"]