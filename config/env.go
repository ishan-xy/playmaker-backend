package config

import (
	"log"
	"os"
	"strconv"

	// utils "github.com/ItsMeSamey/go_utils"
	// "github.com/joho/godotenv"
)

var IsDebug bool

// func loadEnv() {
// 	err := godotenv.Load()
// 	utils.WithStack(err)
// 	if err != nil {
// 		log.Fatal("Error loading .env file")
// 	}
// 	IsDebug = os.Getenv("DEBUG") == "true"
// }

func Getenv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Missing environment variable: %s", key)
	}
	return value
}

func GetEnvAsInt(key string, defaultValue int) (val int) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	strv, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Invalid value for environment variable %s: %s", key, value)
		log.Fatalf("Error: %v", err)
	}
	return strv
}

func GetEnvAsBool(key string, defaultValue bool) (val bool) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value == "true"
}
