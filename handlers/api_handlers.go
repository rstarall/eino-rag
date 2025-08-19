package handlers

import (
	"context"
	"io"
	"net/http"
	"time"

	"eino-rag/services"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ragService    *services.RAGService
	maxUploadSize int64
	chatModel     *openai.ChatModel
}

func NewAPIHandler(ragService *services.RAGService, maxUploadSize int64, chatModel *openai.ChatModel) *APIHandler {
	return &APIHandler{
		ragService:    ragService,
		maxUploadSize: maxUploadSize,
		chatModel:     chatModel,
	}
}

// Request/Response 结构
type UploadResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ChunkCount int    `json:"chunk_count,omitempty"`
}

type SearchRequest struct {
	Query         string `json:"query" binding:"required"`
	ReturnContext bool   `json:"return_context"`
}

type SearchResponse struct {
	Success   bool        `json:"success"`
	Query     string      `json:"query"`
	Context   string      `json:"context,omitempty"`
	Documents []DocResult `json:"documents"`
	Timestamp int64       `json:"timestamp"`
}

type DocResult struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Upload 处理文档上传
func (h *APIHandler) Upload(c *gin.Context) {
	// 获取文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.maxUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, UploadResponse{
			Success: false,
			Message: "File too large",
		})
		return
	}

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Failed to read file",
		})
		return
	}

	// 准备元数据
	metadata := map[string]interface{}{
		"filename":    header.Filename,
		"size":        header.Size,
		"upload_time": time.Now().Format(time.RFC3339),
	}

	// 获取额外的元数据
	if meta := c.PostForm("metadata"); meta != "" {
		// 这里可以解析JSON格式的元数据
	}

	// 索引文档
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = h.ragService.IndexDocument(ctx, string(content), metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		Success: true,
		Message: "Document indexed successfully",
	})
}

// Search 处理搜索请求
func (h *APIHandler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var response SearchResponse
	response.Query = req.Query
	response.Timestamp = time.Now().Unix()

	if req.ReturnContext {
		// 获取带上下文的结果
		context, docs, err := h.ragService.SearchWithContext(ctx, req.Query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		response.Context = context
		response.Documents = convertDocs(docs)
	} else {
		// 仅获取文档
		docs, err := h.ragService.Search(ctx, req.Query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		response.Documents = convertDocs(docs)
	}

	response.Success = true
	c.JSON(http.StatusOK, response)
}

// Health 健康检查
func (h *APIHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "eino-rag",
	})
}

func convertDocs(docs []*schema.Document) []DocResult {
	results := make([]DocResult, len(docs))
	for i, doc := range docs {
		results[i] = DocResult{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    0.0, // Score字段暂时设为0
			Metadata: doc.MetaData,
		}
	}
	return results
}

// GetProcessingStats 获取文档处理统计信息
func (h *APIHandler) GetProcessingStats(c *gin.Context) {
	// 从 RAG 服务获取真实的处理器统计信息
	stats := h.ragService.GetProcessingStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}
