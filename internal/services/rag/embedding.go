package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/db"

	"go.uber.org/zap"
)

type EmbeddingService struct {
	ollamaURL      string
	embeddingModel string
	dimension      int
	logger         *zap.Logger
	httpClient     *http.Client
	useCache       bool
}

func NewEmbeddingService(cfg *config.Config, logger *zap.Logger) *EmbeddingService {
	// 使用可配置的超时时间，避免大文件处理时超时
	embeddingTimeout := cfg.EmbeddingTimeout
	if embeddingTimeout == 0 {
		embeddingTimeout = 120 * time.Second // 默认2分钟
	}
	
	logger.Info("Initializing embedding service",
		zap.Duration("timeout", embeddingTimeout),
		zap.String("model", cfg.EmbeddingModel))
	
	return &EmbeddingService{
		ollamaURL:      cfg.OllamaBaseURL,
		embeddingModel: cfg.EmbeddingModel,
		dimension:      cfg.VectorDimension,
		logger:         logger,
		httpClient: &http.Client{
			Timeout: embeddingTimeout,
		},
		useCache: cfg.EmbeddingCache,
	}
}

// EmbedText 将文本转换为向量
func (s *EmbeddingService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	// 尝试从缓存获取
	if s.useCache {
		cached, err := db.GetCachedEmbedding(ctx, text)
		if err == nil && cached != nil {
			s.logger.Debug("Using cached embedding", zap.Int("text_length", len(text)))
			return cached, nil
		}
	}

	// 调用Ollama API生成嵌入
	embedding, err := s.generateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	// 缓存结果
	if s.useCache {
		if err := db.CacheEmbedding(ctx, text, embedding); err != nil {
			s.logger.Warn("Failed to cache embedding", zap.Error(err))
		}
	}

	return embedding, nil
}

// EmbedTexts 批量转换文本为向量
func (s *EmbeddingService) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	
	for i, text := range texts {
		embedding, err := s.EmbedText(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}
	
	return embeddings, nil
}

// generateEmbedding 调用Ollama API生成嵌入向量
func (s *EmbeddingService) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 记录开始时间
	startTime := time.Now()
	textLen := len(text)
	
	s.logger.Debug("Generating embedding",
		zap.Int("text_length", textLen),
		zap.String("model", s.embeddingModel))
	
	reqBody := map[string]interface{}{
		"model":  s.embeddingModel,
		"prompt": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.ollamaURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s, body: %s", resp.Status, body)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Embedding) != s.dimension {
		return nil, fmt.Errorf("unexpected embedding dimension: got %d, expected %d", len(result.Embedding), s.dimension)
	}

	// 记录耗时
	duration := time.Since(startTime)
	s.logger.Debug("Embedding generated successfully",
		zap.Int("text_length", textLen),
		zap.Duration("duration", duration),
		zap.Int("vector_dimension", len(result.Embedding)))

	return result.Embedding, nil
}

// GetDimension 获取嵌入向量维度
func (s *EmbeddingService) GetDimension() int {
	return s.dimension
}