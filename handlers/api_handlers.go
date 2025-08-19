package handlers

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"eino-rag/components"
	"eino-rag/services"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type APIHandler struct {
	ragService     *services.RAGService
	maxUploadSize  int64
	chatModel      *openai.ChatModel
	logger         *zap.Logger
	documentParser *components.DocumentParser
}

func NewAPIHandler(ragService *services.RAGService, maxUploadSize int64, chatModel *openai.ChatModel, logger *zap.Logger) *APIHandler {
	return &APIHandler{
		ragService:     ragService,
		maxUploadSize:  maxUploadSize,
		chatModel:      chatModel,
		logger:         logger,
		documentParser: components.NewDocumentParser(logger),
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
	h.logger.Info("开始处理文档上传请求", zap.String("client_ip", c.ClientIP()))
	
	// 获取文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.logger.Error("获取上传文件失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}
	defer file.Close()

	h.logger.Info("文件上传信息", 
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size),
		zap.Int64("max_size", h.maxUploadSize))

	// 检查文件大小
	if header.Size > h.maxUploadSize {
		h.logger.Warn("文件大小超出限制", 
			zap.Int64("file_size", header.Size),
			zap.Int64("max_size", h.maxUploadSize))
		c.JSON(http.StatusRequestEntityTooLarge, UploadResponse{
			Success: false,
			Message: "File too large",
		})
		return
	}

	// 检查文件类型是否支持
	if !h.documentParser.IsSupported(header.Filename) {
		h.logger.Warn("不支持的文件类型", 
			zap.String("filename", header.Filename),
			zap.Strings("supported_types", h.documentParser.GetSupportedExtensions()))
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Unsupported file type. Supported types: " + strings.Join(h.documentParser.GetSupportedExtensions(), ", "),
		})
		return
	}

	// 读取文件内容
	h.logger.Debug("开始读取文件内容")
	rawContent, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error("读取文件内容失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Failed to read file",
		})
		return
	}

	h.logger.Info("原始文件读取完成", 
		zap.Int("raw_content_length", len(rawContent)))

	// 解析文档内容
	h.logger.Debug("开始解析文档内容")
	parseStart := time.Now()
	parsedContent, err := h.documentParser.ParseDocument(header.Filename, rawContent)
	parseDuration := time.Since(parseStart)
	
	if err != nil {
		h.logger.Error("文档解析失败", 
			zap.Error(err),
			zap.Duration("parse_duration", parseDuration))
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Failed to parse document: " + err.Error(),
		})
		return
	}

	contentLength := len(parsedContent)
	h.logger.Info("文档解析完成", 
		zap.Int("raw_length", len(rawContent)),
		zap.Int("parsed_length", contentLength),
		zap.Duration("parse_duration", parseDuration),
		zap.String("content_preview", parsedContent[:min(100, contentLength)]))

	// 准备元数据
	metadata := map[string]interface{}{
		"filename":     header.Filename,
		"original_size": header.Size,
		"parsed_size":  contentLength,
		"file_type":    strings.ToLower(filepath.Ext(header.Filename)),
		"upload_time":  time.Now().Format(time.RFC3339),
	}

	// 获取额外的元数据
	if meta := c.PostForm("metadata"); meta != "" {
		h.logger.Debug("接收到额外元数据", zap.String("metadata", meta))
		// 这里可以解析JSON格式的元数据
	}

	h.logger.Info("开始索引文档", zap.Any("metadata", metadata))

	// 索引文档
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	err = h.ragService.IndexDocument(ctx, parsedContent, metadata)
	indexingDuration := time.Since(startTime)
	
	if err != nil {
		h.logger.Error("文档索引失败", 
			zap.Error(err),
			zap.Duration("indexing_duration", indexingDuration))
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("文档索引成功", 
		zap.String("filename", header.Filename),
		zap.Duration("indexing_duration", indexingDuration))

	c.JSON(http.StatusOK, UploadResponse{
		Success: true,
		Message: "Document indexed successfully",
	})
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
		// 从metadata中获取实际的相似度分数
		var score float64 = 0.0
		if doc.MetaData != nil {
			if similarityScore, exists := doc.MetaData["similarity_score"]; exists {
				if scoreFloat, ok := similarityScore.(float64); ok {
					score = scoreFloat
				}
			}
		}
		
		results[i] = DocResult{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    score,
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
