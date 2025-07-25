package config

import (
	"context"
	"log"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/go-redis/redis/v8"
)


var RedisClient *redis.Client

func init() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     Getenv("REDIS_ADDR"),
	})

	// Test the connection
	ping, err := RedisClient.Ping(context.Background()).Result()
	if err != nil {
		panic("Failed to connect to Redis: " + utils.WithStack(err).Error())
	}
	log.Println("Connected to Redis:", ping)
}
