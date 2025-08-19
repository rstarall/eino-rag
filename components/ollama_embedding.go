package components

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// OllamaEmbedding 实现 Eino 的 Embedding 接口
type OllamaEmbedding struct {
	baseURL string
	model   string
	dim     int
	logger  *zap.Logger
}

func NewOllamaEmbedding(baseURL, model string, dim int, logger *zap.Logger) *OllamaEmbedding {
	return &OllamaEmbedding{
		baseURL: baseURL,
		model:   model,
		dim:     dim,
		logger:  logger,
	}
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (o *OllamaEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))

	for i, text := range texts {
		req := ollamaEmbedRequest{
			Model:  o.model,
			Prompt: text,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		resp, err := http.Post(
			fmt.Sprintf("%s/api/embeddings", o.baseURL),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return nil, fmt.Errorf("ollama request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		var embedResp ollamaEmbedResponse
		if err := json.Unmarshal(body, &embedResp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		// 转换float32到float64
		float64Embedding := make([]float64, len(embedResp.Embedding))
		for j, v := range embedResp.Embedding {
			float64Embedding[j] = float64(v)
		}
		embeddings[i] = float64Embedding
	}

	return embeddings, nil
}

func (o *OllamaEmbedding) EmbedDocuments(ctx context.Context, docs []*schema.Document, opts ...embedding.Option) ([]*schema.Document, error) {
	o.logger.Info("开始生成文档嵌入向量", 
		zap.Int("document_count", len(docs)),
		zap.String("model", o.model),
		zap.String("base_url", o.baseURL),
		zap.Int("expected_dimension", o.dim))

	texts := make([]string, len(docs))
	totalTextLength := 0
	for i, doc := range docs {
		texts[i] = doc.Content
		totalTextLength += len(doc.Content)
		o.logger.Debug("文档信息", 
			zap.Int("doc_index", i),
			zap.String("doc_id", doc.ID),
			zap.Int("content_length", len(doc.Content)),
			zap.String("content_preview", doc.Content[:min(50, len(doc.Content))]))
	}

	o.logger.Info("文档统计信息", 
		zap.Int("total_documents", len(docs)),
		zap.Int("total_text_length", totalTextLength),
		zap.Int("average_text_length", totalTextLength/len(docs)))

	startTime := time.Now()
	embeddings, err := o.EmbedStrings(ctx, texts, opts...)
	embeddingDuration := time.Since(startTime)
	
	if err != nil {
		o.logger.Error("生成嵌入向量失败", 
			zap.Error(err),
			zap.Duration("embedding_duration", embeddingDuration))
		return nil, err
	}

	o.logger.Info("嵌入向量生成完成", 
		zap.Int("embedding_count", len(embeddings)),
		zap.Duration("embedding_duration", embeddingDuration))

	// 验证嵌入向量维度
	if len(embeddings) > 0 && len(embeddings[0]) != o.dim {
		o.logger.Warn("嵌入向量维度不匹配", 
			zap.Int("expected_dim", o.dim),
			zap.Int("actual_dim", len(embeddings[0])))
	}

	result := make([]*schema.Document, len(docs))
	for i, doc := range docs {
		// 创建新的文档副本并添加嵌入向量
		newDoc := &schema.Document{
			ID:       doc.ID,
			Content:  doc.Content,
			MetaData: doc.MetaData,
		}
		// 将嵌入向量存储在MetaData中，因为schema.Document可能没有Embedding字段
		if newDoc.MetaData == nil {
			newDoc.MetaData = make(map[string]interface{})
		}
		// 将float64嵌入向量转回float32用于存储
		float32Embedding := make([]float32, len(embeddings[i]))
		for j, v := range embeddings[i] {
			float32Embedding[j] = float32(v)
		}
		newDoc.MetaData["embedding"] = float32Embedding
		
		o.logger.Debug("文档嵌入向量信息", 
			zap.String("doc_id", doc.ID),
			zap.Int("embedding_dimension", len(float32Embedding)),
			zap.Float32("first_value", float32Embedding[0]))
		
		result[i] = newDoc
	}

	o.logger.Info("文档嵌入处理完成", zap.Int("processed_documents", len(result)))
	return result, nil
}



func (o *OllamaEmbedding) Dimension() int {
	return o.dim
}
