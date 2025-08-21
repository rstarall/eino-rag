package config

import (
	"os"
	"strconv"
)

// ChunkingStrategy 分块策略类型
type ChunkingStrategy string

const (
	// ChunkingStrategyLength 基于长度的分块（支持滑动窗口）
	ChunkingStrategyLength ChunkingStrategy = "length"
	// ChunkingStrategySemantic 语义分块
	ChunkingStrategySemantic ChunkingStrategy = "semantic"
	// ChunkingStrategyWordBased 基于单词的分块（向后兼容）
	ChunkingStrategyWordBased ChunkingStrategy = "word_based"
)

type Config struct {
	// Milvus配置
	MilvusHost      string
	MilvusPort      int
	CollectionName  string
	VectorDimension int
	MetricType      string
	IndexType       string

	// Ollama配置
	OllamaBaseURL  string
	EmbeddingModel string
	LLMModel       string

	// OpenAI配置
	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string

	// RAG配置
	ChunkSize             int              // 分块大小（字符数）
	ChunkOverlap          int              // 分块重叠大小（字符数）
	ChunkingStrategy      ChunkingStrategy // 分块策略
	TopK                  int              // 检索返回文档数量
	ScoreThreshold        float32          // 相似度阈值
	SemanticSplitting     bool             // 向后兼容：是否启用语义分割（已废弃，使用ChunkingStrategy代替）
	EmbeddingCacheEnabled bool             // 是否启用嵌入缓存

	// Server配置
	ServerPort    string
	MaxUploadSize int64

	// 超时配置
	IndexTimeout         int // 文档索引总超时时间（秒）
	MilvusInsertTimeout  int // Milvus插入操作超时时间（秒）
	MilvusConnectTimeout int // Milvus连接超时时间（秒）
	GRPCKeepaliveTime    int // gRPC keepalive时间（秒）
	GRPCKeepaliveTimeout int // gRPC keepalive超时时间（秒）

	// PDF解析配置 - 现在固定使用 ledongthuc/pdf 库进行解析
	// 移除了复杂的配置选项，简化为只支持单一快速解析器
}

func Load() *Config {
	return &Config{
		MilvusHost:      getEnv("MILVUS_HOST", "localhost"),
		MilvusPort:      getEnvAsInt("MILVUS_PORT", 19530),
		CollectionName:  getEnv("COLLECTION_NAME", "rag_documents"),
		VectorDimension: getEnvAsInt("VECTOR_DIM", 1024), // bge-m3 模型的向量维度为 1024
		MetricType:      getEnv("METRIC_TYPE", "L2"),
		IndexType:       getEnv("INDEX_TYPE", "IVF_FLAT"),

		OllamaBaseURL:  getEnv("OLLAMA_URL", "http://localhost:11434"),
		EmbeddingModel: getEnv("EMBEDDING_MODEL", "bge-m3"),
		LLMModel:       getEnv("LLM_MODEL", "llama2"),

		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", ""),

		ChunkSize:             getEnvAsInt("CHUNK_SIZE", 500),
		ChunkOverlap:          getEnvAsInt("CHUNK_OVERLAP", 50),
		ChunkingStrategy:      ChunkingStrategy(getEnv("CHUNKING_STRATEGY", string(ChunkingStrategyLength))), // 默认使用长度分块
		TopK:                  getEnvAsInt("TOP_K", 5),
		ScoreThreshold:        float32(getEnvAsFloat("SCORE_THRESHOLD", 0.7)),
		SemanticSplitting:     getEnv("SEMANTIC_SPLITTING", "false") == "true", // 默认关闭语义分割
		EmbeddingCacheEnabled: getEnv("EMBEDDING_CACHE", "true") == "true",

		ServerPort:    getEnv("SERVER_PORT", "8080"),
		MaxUploadSize: getEnvAsInt64("MAX_UPLOAD_SIZE", 10*1024*1024),

		IndexTimeout:         getEnvAsInt("INDEX_TIMEOUT", 120),
		MilvusInsertTimeout:  getEnvAsInt("MILVUS_INSERT_TIMEOUT", 60),
		MilvusConnectTimeout: getEnvAsInt("MILVUS_CONNECT_TIMEOUT", 30),
		GRPCKeepaliveTime:    getEnvAsInt("GRPC_KEEPALIVE_TIME", 30),
		GRPCKeepaliveTimeout: getEnvAsInt("GRPC_KEEPALIVE_TIMEOUT", 5),

		// PDF配置已简化，不再需要额外配置
	}
}

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

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
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
