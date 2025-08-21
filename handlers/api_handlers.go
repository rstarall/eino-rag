package handlers

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"eino-rag/components"
	"eino-rag/config"
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
	config         *config.Config
}

func NewAPIHandler(ragService *services.RAGService, maxUploadSize int64, chatModel *openai.ChatModel, logger *zap.Logger, cfg *config.Config) *APIHandler {
	return &APIHandler{
		ragService:     ragService,
		maxUploadSize:  maxUploadSize,
		chatModel:      chatModel,
		logger:         logger,
		documentParser: components.NewDocumentParser(logger),
		config:         cfg,
	}
}

// Request/Response 结构

// UploadResponse 文档上传响应
type UploadResponse struct {
	Success    bool   `json:"success" example:"true"`                          // 操作是否成功
	Message    string `json:"message" example:"Document indexed successfully"` // 响应消息
	ChunkCount int    `json:"chunk_count,omitempty" example:"5"`               // 文档分块数量
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query         string `json:"query" binding:"required" example:"人工智能的发展历史"` // 搜索查询内容
	ReturnContext bool   `json:"return_context" example:"true"`                // 是否返回上下文信息
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Success   bool        `json:"success" example:"true"`                  // 操作是否成功
	Query     string      `json:"query" example:"人工智能的发展历史"`               // 原始查询内容
	Context   string      `json:"context,omitempty" example:"根据检索到的文档..."` // 生成的上下文信息
	Documents []DocResult `json:"documents"`                               // 检索到的文档列表
	Timestamp int64       `json:"timestamp" example:"1640995200"`          // 响应时间戳
}

// DocResult 文档搜索结果
type DocResult struct {
	ID       string                 `json:"id" example:"doc_12345"`         // 文档ID
	Content  string                 `json:"content" example:"这是文档的内容片段..."` // 文档内容
	Score    float64                `json:"score" example:"0.85"`           // 相似度分数
	Metadata map[string]interface{} `json:"metadata"`                       // 文档元数据
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status" example:"healthy"`       // 服务状态
	Timestamp int64  `json:"timestamp" example:"1640995200"` // 时间戳
	Service   string `json:"service" example:"eino-rag"`     // 服务名称
}

// ProcessingStatsResponse 处理统计响应
type ProcessingStatsResponse struct {
	Success bool        `json:"success" example:"true"` // 操作是否成功
	Stats   interface{} `json:"stats"`                  // 统计信息
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`   // 操作是否成功
	Message string `json:"message" example:"操作失败的原因"` // 错误信息
}

// Upload 处理文档上传
// @Summary 上传文档
// @Description 上传PDF、TXT、Markdown、JSON、CSV、HTML等格式的文档进行向量化处理和索引
// @Tags 文档管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文档文件 (支持 .pdf, .txt, .md, .markdown, .json, .csv, .html, .htm 格式)"
// @Param metadata formData string false "文档元数据 (JSON格式)"
// @Success 200 {object} UploadResponse "上传成功"
// @Failure 400 {object} ErrorResponse "请求错误 (文件格式不支持、文件过大等)"
// @Failure 413 {object} ErrorResponse "文件过大"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/upload [post]
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
		"filename":      header.Filename,
		"original_size": header.Size,
		"parsed_size":   contentLength,
		"file_type":     strings.ToLower(filepath.Ext(header.Filename)),
		"upload_time":   time.Now().Format(time.RFC3339),
	}

	// 获取额外的元数据
	if meta := c.PostForm("metadata"); meta != "" {
		h.logger.Debug("接收到额外元数据", zap.String("metadata", meta))
		// 这里可以解析JSON格式的元数据
	}

	h.logger.Info("开始索引文档", zap.Any("metadata", metadata))

	// 索引文档 - 使用配置中的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.config.IndexTimeout)*time.Second)
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
// @Summary 搜索文档
// @Description 根据查询内容在已索引的文档中进行语义搜索
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求参数"
// @Success 200 {object} SearchResponse "搜索成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "搜索服务错误"
// @Router /api/search [post]
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
// @Summary 健康检查
// @Description 检查服务运行状态
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse "服务正常"
// @Router /api/health [get]
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
// @Summary 获取处理统计
// @Description 获取文档处理和索引的统计信息
// @Tags 系统
// @Produce json
// @Success 200 {object} ProcessingStatsResponse "获取统计信息成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/stats [get]
func (h *APIHandler) GetProcessingStats(c *gin.Context) {
	// 从 RAG 服务获取真实的处理器统计信息
	stats := h.ragService.GetProcessingStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}
