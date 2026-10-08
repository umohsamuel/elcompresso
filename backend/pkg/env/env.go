package env

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type S3Config struct {
	Region string
	Bucket string
	Prefix string
}

type Limits struct {
	VideoBytes  int64
	AudioBytes  int64
	ImageBytes  int64
	ImagePixels int64
}

type EnvironmentVariables struct {
	Port                  string
	ProductionEnvironment bool
	ProjectName           string

	// AllowedOrigins are the browser origins allowed to call the API. Entries
	// may contain a single "*" wildcard, e.g. https://*.vercel.app.
	AllowedOrigins []string
	TrustedProxies []string

	DataDir     string
	StorageType string // local | s3
	S3          S3Config

	ResultTTL  time.Duration
	JobTimeout time.Duration
	Workers    int
	QueueSize  int

	FFmpegPath    string
	FFprobePath   string
	FFmpegThreads int

	Limits             Limits
	RateLimitPerMinute int
	MaxActiveJobsPerIP int
}

func LoadEnvironment() *EnvironmentVariables {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using process environment")
	}

	port := getEnv("PORT", ":8080")
	if !strings.Contains(port, ":") {
		port = ":" + port
	}

	e := &EnvironmentVariables{
		Port:                  port,
		ProductionEnvironment: getEnvAsBool("PRODUCTION_ENVIRONMENT", false),
		ProjectName:           getEnv("PROJECT_NAME", "elcompresso"),

		AllowedOrigins: getEnvAsList("ALLOWED_ORIGINS", "http://localhost:3000"),
		TrustedProxies: getEnvAsList("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"),

		DataDir:     getEnv("DATA_DIR", "./data"),
		StorageType: strings.ToLower(getEnv("STORAGE_TYPE", "local")),
		S3: S3Config{
			Region: getEnv("AWS_REGION", ""),
			Bucket: getEnv("AWS_BUCKET", ""),
			Prefix: getEnv("S3_PREFIX", "compressed/"),
		},

		ResultTTL:  getEnvAsDuration("RESULT_TTL", time.Hour),
		JobTimeout: getEnvAsDuration("JOB_TIMEOUT", 30*time.Minute),
		Workers:    getEnvAsInt("WORKERS", max(1, runtime.NumCPU()/2)),
		QueueSize:  getEnvAsInt("QUEUE_SIZE", 100),

		FFmpegPath:    getEnv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:   getEnv("FFPROBE_PATH", "ffprobe"),
		FFmpegThreads: getEnvAsInt("FFMPEG_THREADS", 0),

		Limits: Limits{
			VideoBytes:  int64(getEnvAsInt("MAX_VIDEO_MB", 1024)) << 20,
			AudioBytes:  int64(getEnvAsInt("MAX_AUDIO_MB", 200)) << 20,
			ImageBytes:  int64(getEnvAsInt("MAX_IMAGE_MB", 50)) << 20,
			ImagePixels: int64(getEnvAsInt("MAX_IMAGE_MEGAPIXELS", 100)) * 1_000_000,
		},
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 30),
		MaxActiveJobsPerIP: getEnvAsInt("MAX_ACTIVE_JOBS_PER_IP", 5),
	}

	if err := e.validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	return e
}

func (e *EnvironmentVariables) validate() error {
	switch e.StorageType {
	case "local":
	case "s3":
		if e.S3.Bucket == "" {
			return fmt.Errorf("AWS_BUCKET is required when STORAGE_TYPE=s3")
		}
	default:
		return fmt.Errorf("STORAGE_TYPE must be local or s3, got %q", e.StorageType)
	}
	if e.Workers < 1 || e.QueueSize < 1 {
		return fmt.Errorf("WORKERS and QUEUE_SIZE must be at least 1")
	}
	if e.ResultTTL < time.Minute {
		return fmt.Errorf("RESULT_TTL must be at least 1m")
	}
	return nil
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvAsList(key, fallback string) []string {
	var out []string
	for item := range strings.SplitSeq(getEnv(key, fallback), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, strings.TrimSuffix(item, "/"))
		}
	}
	return out
}

func getEnvAsInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("environment variable %q must be an integer, got %q", key, value)
	}
	return v
}

func getEnvAsBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	v, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("environment variable %q must be true or false, got %q", key, value)
	}
	return v
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	v, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("environment variable %q must be a duration like 30m or 2h, got %q", key, value)
	}
	return v
}
