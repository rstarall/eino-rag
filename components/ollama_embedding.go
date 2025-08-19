package components

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// OllamaEmbedding 实现 Eino 的 Embedding 接口
type OllamaEmbedding struct {
	baseURL string
	model   string
	dim     int
}

func NewOllamaEmbedding(baseURL, model string, dim int) *OllamaEmbedding {
	return &OllamaEmbedding{
		baseURL: baseURL,
		model:   model,
		dim:     dim,
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
	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	embeddings, err := o.EmbedStrings(ctx, texts, opts...)
	if err != nil {
		return nil, err
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
		result[i] = newDoc
	}

	return result, nil
}

func (o *OllamaEmbedding) Dimension() int {
	return o.dim
}
