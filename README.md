# Go Cloud Compute (分布式任务调度系统)

![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)
![Architecture](https://img.shields.io/badge/Architecture-Master%2FWorker-green)
![Status](https://img.shields.io/badge/Status-V1.5-orange)

## 📖 项目简介

Go Cloud Compute 是一个轻量级的分布式任务调度平台。采用 **Master-Worker** 架构设计，旨在解决单机计算瓶颈问题。

系统将任务分发与执行解耦，利用 **RabbitMQ** 实现流量削峰与负载均衡，通过 **gRPC** 实现节点的心跳保活与状态监控。目前版本已支持 Shell 脚本的远程执行、结果回传以及节点的自动注册与发现。

---

## 🏗 系统架构

[Client] (HTTP)
   ⬇
[Server (Master)]  <---(gRPC Heartbeat/Report)--->  [Agent (Worker)]
   ⬇ (Publish)                                         ⬆ (Consume)
[RabbitMQ] --------------------------------------------+
   ⬇
[MySQL] (Persistence)

* **Server (控制面)**: 提供 HTTP 接口接收用户任务；通过 gRPC 管理 Agent 节点的生命周期（注册、心跳、状态更新）。
* **Agent (数据面)**: 无状态计算节点，可水平扩容。通过 MQ 抢占式消费任务，执行完毕后通过 gRPC 汇报结果。
* **RabbitMQ**: 充当任务总线，确保任务在海量并发下的可靠缓冲与分发。

---

## ✨ 核心特性 (V1.5)

### 1. 混合通信架构 (Hybrid Communication)
* **外部交互**: 使用标准 RESTful API (HTTP) 接收外部请求。
* **内部治理**: 使用 **gRPC (Protobuf)** 进行高性能的节点注册与心跳检测。
* **异步解耦**: 使用 **RabbitMQ** 进行任务投递，实现了生产者与消费者的完全解耦。

### 2. 高可靠性设计 (Reliability)
* **消息确认 (ACK)**: 实现了 RabbitMQ 的手动 ACK 机制。只有当 Agent 真正执行完任务后才会确认消息，防止因节点宕机导致任务丢失。
* **优雅停机 (Graceful Shutdown)**: Server 和 Agent 均监听系统信号 (`SIGINT/SIGTERM`)。在关闭时，Agent 会停止接收新任务，并等待当前正在执行的任务完成后再退出，确保**“零数据丢失”**。
* **QoS 控制**: 配置了 `QoS Prefetch`，防止单个节点因积压过多任务而崩溃。

### 3. 可观测性 (Observability)
* **链路日志**: 集成了自定义的 HTTP 中间件与 gRPC 拦截器，记录每个请求的耗时、来源 IP 及状态码，便于性能分析与故障排查。
* **状态管理**: MySQL 实时记录 Agent 的在线状态及任务的执行历史。

---

## 🚀 快速开始

### 环境要求
* Go 1.21+
* Docker & Docker Compose
* Make (可选)

### 1. 启动基础设施
使用 Docker 一键启动 MySQL 和 RabbitMQ：
```bash
# 启动中间件
docker compose up -d