package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultSESSenderEmail = "aniketjindal2002@gmail.com"

type Config struct {
	AppEnv             string
	HTTPPort           string
	AppBaseURL         string
	CORSAllowedOrigins []string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	S3Endpoint      string
	S3PublicBaseURL string
	S3Region        string
	S3Bucket        string
	S3AccessKey     string
	S3SecretKey     string
	S3SessionToken  string
	S3UsePathStyle  bool

	AWSRegion          string
	AWSEndpoint        string
	SESSenderEmail     string
	EventBridgeRoleARN string
	PayoutQueueURL     string
	PayoutQueueARN     string

	DefaultCurrency                string
	GuestFeeBPS                    int
	HostFeeBPS                     int
	RequestExpiryHours             int
	PayoutReleaseAfterCheckInHours int
	VersionMode                    string // "v1" or "v2"
}

func Load() *Config {
	loadDotEnv(".env")
	loadDotEnv("backend/.env")

	accessMinutes := getEnvInt("JWT_ACCESS_TTL_MINUTES", 60)
	refreshDays := getEnvInt("JWT_REFRESH_TTL_DAYS", 30)

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	var origins []string
	for _, o := range strings.Split(originsRaw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	s3Region := getEnvFirst([]string{"S3_REGION", "AWS_REGION", "AWS_DEFAULT_REGION"}, "eu-west-1")
	s3Bucket := getEnvFirst([]string{"S3_BUCKET", "AWS_S3_BUCKET"}, "stayhub-media")
	s3Endpoint := getEnv("S3_ENDPOINT", fmt.Sprintf("https://s3.%s.amazonaws.com", s3Region))
	s3PublicBaseURL := getEnv("S3_PUBLIC_BASE_URL", "/api/v1/media/file")

	defaultAppBaseURL := "http://localhost:5173"
	if len(origins) > 0 {
		defaultAppBaseURL = origins[len(origins)-1]
	}

	return &Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		AppBaseURL:         strings.TrimRight(getEnvFirst([]string{"APP_BASE_URL", "FRONTEND_BASE_URL"}, defaultAppBaseURL), "/"),
		CORSAllowedOrigins: origins,

		DatabaseURL: getEnv("DATABASE_URL", "postgres://stayhub:stayhub_dev_pwd@localhost:5432/stayhub?sslmode=disable"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		JWTSecret:     getEnv("JWT_SECRET", "stayhub-local-dev-secret-change-in-prod"),
		JWTAccessTTL:  time.Duration(accessMinutes) * time.Minute,
		JWTRefreshTTL: time.Duration(refreshDays) * 24 * time.Hour,

		S3Endpoint:      s3Endpoint,
		S3PublicBaseURL: s3PublicBaseURL,
		S3Region:        s3Region,
		S3Bucket:        s3Bucket,
		S3AccessKey:     getEnvFirst([]string{"S3_ACCESS_KEY", "AWS_ACCESS_KEY_ID"}, ""),
		S3SecretKey:     getEnvFirst([]string{"S3_SECRET_KEY", "AWS_SECRET_ACCESS_KEY"}, ""),
		S3SessionToken:  getEnvFirst([]string{"S3_SESSION_TOKEN", "AWS_SESSION_TOKEN"}, ""),
		S3UsePathStyle:  getEnvBool("S3_USE_PATH_STYLE", false),

		AWSRegion:          s3Region,
		AWSEndpoint:        getEnvFirst([]string{"AWS_ENDPOINT_URL", "AWS_ENDPOINT"}, ""),
		SESSenderEmail:     DefaultSESSenderEmail,
		EventBridgeRoleARN: getEnv("EVENTBRIDGE_ROLE_ARN", ""),
		PayoutQueueURL:     getEnv("PAYOUT_QUEUE_URL", ""),
		PayoutQueueARN:     getEnv("PAYOUT_QUEUE_ARN", ""),

		DefaultCurrency:                getEnv("DEFAULT_CURRENCY", "EUR"),
		GuestFeeBPS:                    getEnvInt("GUEST_FEE_BPS", 1400),
		HostFeeBPS:                     getEnvInt("HOST_FEE_BPS", 300),
		RequestExpiryHours:             getEnvInt("REQUEST_EXPIRY_HOURS", 24),
		PayoutReleaseAfterCheckInHours: getEnvInt("PAYOUT_RELEASE_AFTER_CHECKIN_HOURS", 24),
		VersionMode:                    strings.ToLower(getEnv("STAYHUB_VERSION_MODE", "v1")),
	}
}

func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getEnvFirst(keys []string, fallback string) string {
	for _, k := range keys {
		if val, ok := os.LookupEnv(k); ok && val != "" {
			return val
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed
		}
	}
	return fallback
}
