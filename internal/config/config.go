package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type ChunkingStrategy string

const (
	ChunkingStrategyLength   ChunkingStrategy = "length"
	ChunkingStrategySemantic ChunkingStrategy = "semantic"
)

type Config struct {
	// Server
	ServerPort string
	ServerHost string
	GinMode    string

	// Database
	DBPath string

	// Redis
	RedisURL      string
	RedisDB       int
	RedisPassword string

	// Milvus
	MilvusAddress   string // 完整的Milvus地址
	CollectionName  string
	VectorDimension int
	MetricType      string
	IndexType       string

	// Ollama
	OllamaBaseURL  string
	EmbeddingModel string
	LLMModel       string

	// OpenAI
	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string

	// RAG
	ChunkSize        int
	ChunkOverlap     int
	ChunkingStrategy ChunkingStrategy
	TopK             int
	ScoreThreshold   float32
	EmbeddingCache   bool

	// Authentication
	JWTSecret      string
	JWTExpireHours int
	SessionSecret  string

	// Upload
	MaxUploadSize    int64
	AllowedFileTypes []string

	// Timeouts
	IndexTimeout         time.Duration
	MilvusInsertTimeout  time.Duration
	MilvusConnectTimeout time.Duration
	GRPCKeepaliveTime    time.Duration
	EmbeddingTimeout     time.Duration
	GRPCKeepaliveTimeout time.Duration
}

var cfg *Config

func Load() *Config {
	if cfg != nil {
		return cfg
	}

	// Load .env file if exists
	godotenv.Load()

	cfg = &Config{
		// Server
		ServerPort: getEnv("SERVER_PORT", "8080"),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),
		GinMode:    getEnv("GIN_MODE", "debug"),

		// Database
		DBPath: getEnv("DB_PATH", "./data/eino-rag.db"),

		// Redis
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Milvus
		MilvusAddress:   getEnv("MILVUS_ADDRESS", "localhost:19530"),
		CollectionName:  getEnv("COLLECTION_NAME", "eino_rag_documents"),
		VectorDimension: getEnvAsInt("VECTOR_DIM", 1024),
		MetricType:      getEnv("METRIC_TYPE", "L2"),
		IndexType:       getEnv("INDEX_TYPE", "IVF_FLAT"),

		// Ollama
		OllamaBaseURL:  getEnv("OLLAMA_URL", "http://localhost:11434"),
		EmbeddingModel: getEnv("EMBEDDING_MODEL", "bge-m3"),
		LLMModel:       getEnv("LLM_MODEL", "llama2"),

		// OpenAI
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-4o"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", ""),

		// RAG
		ChunkSize:        getEnvAsInt("CHUNK_SIZE", 500),
		ChunkOverlap:     getEnvAsInt("CHUNK_OVERLAP", 50),
		ChunkingStrategy: ChunkingStrategy(getEnv("CHUNKING_STRATEGY", string(ChunkingStrategyLength))),
		TopK:             getEnvAsInt("TOP_K", 5),
		ScoreThreshold:   float32(getEnvAsFloat("SCORE_THRESHOLD", 0.7)),
		EmbeddingCache:   getEnvAsBool("EMBEDDING_CACHE", true),

		// Authentication
		JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-here"),
		JWTExpireHours: getEnvAsInt("JWT_EXPIRE_HOURS", 24),
		SessionSecret:  getEnv("SESSION_SECRET", "your-session-secret-here"),

		// Upload
		MaxUploadSize:    getEnvAsInt64("MAX_UPLOAD_SIZE", 10*1024*1024),
		AllowedFileTypes: strings.Split(getEnv("ALLOWED_FILE_TYPES", ".pdf,.txt,.md,.markdown,.json,.csv,.html,.htm"), ","),

		// Timeouts
		IndexTimeout:         time.Duration(getEnvAsInt("INDEX_TIMEOUT", 120)) * time.Second,
		MilvusInsertTimeout:  time.Duration(getEnvAsInt("MILVUS_INSERT_TIMEOUT", 60)) * time.Second,
		MilvusConnectTimeout: time.Duration(getEnvAsInt("MILVUS_CONNECT_TIMEOUT", 30)) * time.Second,
		GRPCKeepaliveTime:    time.Duration(getEnvAsInt("GRPC_KEEPALIVE_TIME", 30)) * time.Second,
		EmbeddingTimeout:     time.Duration(getEnvAsInt("EMBEDDING_TIMEOUT", 120)) * time.Second,
		GRPCKeepaliveTimeout: time.Duration(getEnvAsInt("GRPC_KEEPALIVE_TIMEOUT", 5)) * time.Second,
	}

	return cfg
}

func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// UpdateFromDB 从数据库更新配置
func UpdateFromDB(configs map[string]string) {
	if cfg == nil {
		return
	}
	
	// 更新Milvus配置
	if val, ok := configs["milvus_address"]; ok && val != "" {
		cfg.MilvusAddress = val
	}
	if val, ok := configs["collection_name"]; ok {
		cfg.CollectionName = val
	}
	
	// 更新Ollama配置
	if val, ok := configs["ollama_url"]; ok {
		cfg.OllamaBaseURL = val
	}
	if val, ok := configs["embedding_model"]; ok {
		cfg.EmbeddingModel = val
	}
	if val, ok := configs["llm_model"]; ok {
		cfg.LLMModel = val
	}
	
	// 更新OpenAI配置
	if val, ok := configs["openai_model"]; ok {
		cfg.OpenAIModel = val
	}
	if val, ok := configs["openai_base_url"]; ok && val != "" {
		cfg.OpenAIBaseURL = val
	}
	
	// 更新RAG配置
	if val, ok := configs["chunk_size"]; ok {
		if size, err := strconv.Atoi(val); err == nil {
			cfg.ChunkSize = size
		}
	}
	if val, ok := configs["chunk_overlap"]; ok {
		if overlap, err := strconv.Atoi(val); err == nil {
			cfg.ChunkOverlap = overlap
		}
	}
	if val, ok := configs["chunking_strategy"]; ok {
		cfg.ChunkingStrategy = ChunkingStrategy(val)
	}
	if val, ok := configs["top_k"]; ok {
		if topK, err := strconv.Atoi(val); err == nil {
			cfg.TopK = topK
		}
	}
	if val, ok := configs["score_threshold"]; ok {
		if threshold, err := strconv.ParseFloat(val, 32); err == nil {
			cfg.ScoreThreshold = float32(threshold)
		}
	}
	
	// 更新文件上传限制
	if val, ok := configs["max_file_size"]; ok {
		if size, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxUploadSize = size * 1024 * 1024 // MB to bytes
		}
	}
	if val, ok := configs["max_upload_size"]; ok {
		if size, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxUploadSize = size
		}
	}
	
	// 更新OpenAI API Key
	if val, ok := configs["openai_api_key"]; ok && val != "" {
		cfg.OpenAIAPIKey = val
	}
	
	// 更新向量维度
	if val, ok := configs["vector_dim"]; ok {
		if dim, err := strconv.Atoi(val); err == nil {
			cfg.VectorDimension = dim
		}
	}
	
	// 更新Milvus额外配置
	if val, ok := configs["metric_type"]; ok && val != "" {
		cfg.MetricType = val
	}
	if val, ok := configs["index_type"]; ok && val != "" {
		cfg.IndexType = val
	}
	
	// 更新嵌入缓存配置
	if val, ok := configs["embedding_cache"]; ok {
		if cache, err := strconv.ParseBool(val); err == nil {
			cfg.EmbeddingCache = cache
		}
	}
	
	// 更新文件类型配置
	if val, ok := configs["allowed_file_types"]; ok && val != "" {
		// 简单处理：按逗号分隔
		types := strings.Split(val, ",")
		for i := range types {
			types[i] = strings.TrimSpace(types[i])
		}
		if len(types) > 0 {
			cfg.AllowedFileTypes = types
		}
	}
	
	// 更新超时配置
	if val, ok := configs["index_timeout"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.IndexTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val, ok := configs["milvus_insert_timeout"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.MilvusInsertTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val, ok := configs["embedding_timeout"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.EmbeddingTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val, ok := configs["milvus_connect_timeout"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.MilvusConnectTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val, ok := configs["grpc_keepalive_time"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.GRPCKeepaliveTime = time.Duration(timeout) * time.Second
		}
	}
	if val, ok := configs["grpc_keepalive_timeout"]; ok {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.GRPCKeepaliveTimeout = time.Duration(timeout) * time.Second
		}
	}
}