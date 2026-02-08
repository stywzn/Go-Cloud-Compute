# G-Asset-Platform (Distributed Security Scanner)

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker)
![RabbitMQ](https://img.shields.io/badge/RabbitMQ-Messaging-FF6600?style=flat&logo=rabbitmq)
![Gin](https://img.shields.io/badge/Gin-Web_Framework-008ECF?style=flat&logo=go)

## 📖 Introduction
**G-Asset-Platform** is an enterprise-level distributed security asset management and scanning system. Designed with a microservices architecture, it addresses performance bottlenecks in large-scale network asset detection.

The system utilizes **RabbitMQ** for asynchronous task distribution and traffic peak clipping, and employs **Golang**'s concurrency model (Goroutines) to build a high-performance Worker Pool. It supports rapid port scanning and liveness detection for massive IP ranges, with real-time result persistence to MySQL.

## 🏗️ Architecture

mermaid
graph LR
    User[User] -- POST /api/scan --> API_Gateway[API Gateway (Gin)]
    API_Gateway -- 1. Persist Task --> MySQL[(MySQL DB)]
    API_Gateway -- 2. Publish Message --> MQ[[RabbitMQ]]
    
    MQ -- Consume Task --> Worker_Pool[Scanner Worker Pool]
    
    subgraph Worker Nodes
    Worker1[Worker-1]
    Worker2[Worker-2]
    Worker3[Worker-3]
    end
    
    Worker_Pool --- Worker1
    Worker_Pool --- Worker2
    Worker_Pool --- Worker3
    
    Worker1 -- 3. Scan (net.Dial) --> Internet[Target Assets]
    Worker1 -- 4. Update Result --> MySQL
✨ Key Features
Distributed Architecture: Decoupled API Server and Worker nodes, supporting horizontal scaling.

High Concurrency: Implemented with Goroutine Pool, supporting thousands of concurrent scans per node.

Asynchronous Processing: Integrated RabbitMQ to handle traffic bursts and prevent database overload.

Containerization: One-click deployment for MySQL, RabbitMQ, and services using docker-compose.

RESTful API: Standard HTTP interfaces for task submission and result querying.

🛠️ Tech Stack
Language: Golang (1.21+)

Web Framework: Gin

Database: MySQL 8.0 + GORM

Message Queue: RabbitMQ

Concurrency: Goroutine + Channel + WaitGroup

Deployment: Docker + Docker Compose

🚀 Quick Start
1. Prerequisites
Ensure Docker and Docker Compose are installed.

2. Start Infrastructure
Bash
docker-compose -f deploy/docker-compose.yml up -d
3. Run Services
Start API Server (Terminal 1):

Bash
go run cmd/api-server/main.go
Start Scanning Worker (Terminal 2):

Bash
go run cmd/scan-worker/main.go
4. API Usage
Submit Scan Task:

Bash
curl -X POST http://localhost:8080/api/scan \
  -H "Content-Type: application/json" \
  -d '{"target": "127.0.0.1"}'
Query Task Result:

Bash
# Replace '1' with the actual task_id returned
curl "http://localhost:8080/api/task?id=1"
📄 Directory Structure
Plaintext
G-Asset-Platform/
├── cmd/
│   ├── api-server/   # API Gateway entry
│   └── scan-worker/  # Scanning Worker entry
├── internal/
│   └── model/        # Database models
├── pkg/
│   ├── db/           # Database utilities
│   └── mq/           # RabbitMQ utilities
├── deploy/           # Docker configuration
└── go.mod            # Dependencies
