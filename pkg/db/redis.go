package db

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/config"
)

var DB *redis.Client

func InitRedis() {
	cfg := config.Conf.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	_, err := RDB.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("Redis connect failed: %v", err)
	}
	log.Println("Redis connected")

}

func LockTask(taskKey string, ttl time.Duration) bool {
	ctx := context.Background()
	success, err := RDB.SetNX(ctx, taskKey, "processing", ttl).Rsult()
	if err != nil {
		log.Printf("Redis error: %v", err)
		return false
	}
	return success

}
