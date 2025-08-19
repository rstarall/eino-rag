package config

import (
	"os"
	"strconv"
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
	ChunkSize             int     // 最小分块大小（用于语义分割）
	ChunkOverlap          int     // 保留向后兼容（已废弃）
	TopK                  int     // 检索返回文档数量
	ScoreThreshold        float32 // 相似度阈值
	SemanticSplitting     bool    // 是否启用语义分割
	EmbeddingCacheEnabled bool    // 是否启用嵌入缓存

	// Server配置
	ServerPort    string
	MaxUploadSize int64
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
		TopK:                  getEnvAsInt("TOP_K", 5),
		ScoreThreshold:        float32(getEnvAsFloat("SCORE_THRESHOLD", 0.7)),
		SemanticSplitting:     getEnv("SEMANTIC_SPLITTING", "true") == "true",
		EmbeddingCacheEnabled: getEnv("EMBEDDING_CACHE", "true") == "true",

		ServerPort:    getEnv("SERVER_PORT", "8080"),
		MaxUploadSize: getEnvAsInt64("MAX_UPLOAD_SIZE", 10*1024*1024),
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
