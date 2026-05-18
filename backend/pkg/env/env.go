package env

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	config "github.com/umohsamuel/elcompresso/config/file"
)

type RedisConfig struct {
	REDIS_ADDR     string
	REDIS_PASSWORD string
}

type RabbitMQConfig struct {
	RABBITMQ_ADDR     string
	RABBITMQ_PASSWORD string
}

type S3Config struct {
	AWS_REGION            string
	AWS_BUCKET            string
	AWS_ACCESS_KEY_ID     string
	AWS_SECRET_ACCESS_KEY string
}

type EnvironmentVariables struct {
	Port                  string
	ProductionEnvironment bool
	ClientDomain          string
	ProjectName           string
	STORAGE_TYPE          string
	S3                    *S3Config
	// Redis                 *RedisConfig
	// RabbitMQ              *RabbitMQConfig
}

func loadEnv() {
	rootPath := config.GetRootPath()
	err := godotenv.Load(rootPath + `/.env`)

	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}
}

func LoadEnvironment() *EnvironmentVariables {
	loadEnv()
	return &EnvironmentVariables{
		Port:                  getEnv("PORT", ":5000"),
		ProductionEnvironment: getEnvAsBool("PRODUCTION_ENVIRONMENT", false),
		ClientDomain:          getEnv("CLIENT_DOMAIN", "localhost"),
		ProjectName:           getEnv("PROJECT_NAME", "rider"),

		STORAGE_TYPE: getEnv("STORAGE_TYPE", "local"),

		S3: &S3Config{

			AWS_REGION:            getEnvOrError("AWS_REGION"),
			AWS_BUCKET:            getEnvOrError("AWS_BUCKET"),
			AWS_ACCESS_KEY_ID:     getEnvOrError("AWS_ACCESS_KEY_ID"),
			AWS_SECRET_ACCESS_KEY: getEnvOrError("AWS_SECRET_ACCESS_KEY"),
		},

		// Redis: &RedisConfig{
		// 	REDIS_ADDR:     getEnvOrError("REDIS_ADDR"),
		// 	REDIS_PASSWORD: getEnvOrError("REDIS_PASSWORD"),
		// },

		// RabbitMQ: &RabbitMQConfig{
		// 	RABBITMQ_ADDR:     getEnvOrError("RABBITMQ_ADDR"),
		// 	RABBITMQ_PASSWORD: getEnvOrError("RABBITMQ_PASSWORD"),
		// },
	}
}

func getEnvOrError(key string) string {
	value, exists := os.LookupEnv(key)
	if exists {
		return value
	}
	panic("Environment variable " + key + " not set")
}

func getEnv(key string, fallback string) string {
	value, exists := os.LookupEnv(key)
	if exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	value, exist := os.LookupEnv(key)
	if exist {
		valueInt, err := strconv.Atoi(value)
		if err != nil {
			log.Panicf("Environment variable \"%v\" not set properly", key)
		}
		return valueInt
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	value, exist := os.LookupEnv(key)
	if exist {
		valueBool, err := strconv.ParseBool(value)
		if err != nil {
			log.Panicf("Environment variable \"%v\" not set properly", key)
		}
		return valueBool
	}
	return fallback
}
