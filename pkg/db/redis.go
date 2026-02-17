package db

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stywzn/Go-Cloud-Compute/pkg/config"
)

var RDB *redis.Client

func InitRedis() {
	cfg := config.GlobalConfig
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	_, err := RDB.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("Redis connect failed: %v", err)
	}
	log.Println("Redis connected")

}

func LockTask(taskKey string, ttl time.Duration) bool {
	ctx := context.Background()
	success, err := RDB.SetNX(ctx, taskKey, "processing", ttl).Result()
	if err != nil {
		log.Printf("Redis error: %v", err)
		return false
	}
	return success

}
