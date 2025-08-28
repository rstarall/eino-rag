package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
	"eino-rag/internal/services/document"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DocumentHandler struct {
	docService *document.Service
	logger     *zap.Logger
}

func NewDocumentHandler(docService *document.Service, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{
		docService: docService,
		logger:     logger,
	}
}

// Upload 上传文档
// @Summary 上传文档
// @Description 上传文档到指定知识库
// @Tags 文档管理
// @Accept multipart/form-data
// @Produce json
// @Security ApiKeyAuth
// @Param kb_id formData int true "知识库ID"
// @Param file formData file true "文档文件"
// @Success 200 {object} UploadResponse "上传成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/documents/upload [post]
func (h *DocumentHandler) Upload(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found in context",
		})
		return
	}

	// 获取知识库ID
	kbIDStr := c.PostForm("kb_id")
	kbID, err := strconv.ParseUint(kbIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid knowledge base ID",
		})
		return
	}

	// 获取文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Failed to get file",
		})
		return
	}
	defer file.Close()

	// 上传文档
	// 设置上传超时时间，避免前端无限等待
	uploadCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	
	h.logger.Info("Starting document upload",
		zap.String("filename", header.Filename),
		zap.Int64("filesize", header.Size),
		zap.Uint64("kb_id", kbID))
	
	doc, chunkCount, err := h.docService.UploadDocument(
		uploadCtx,
		header.Filename,
		file,
		uint(kbID),
		userID.(uint),
	)
	if err != nil {
		h.logger.Error("Failed to upload document", 
			zap.String("filename", header.Filename),
			zap.Error(err))
		
		// 检查是否是超时错误
		if errors.Is(err, context.DeadlineExceeded) {
			c.JSON(http.StatusRequestTimeout, ErrorResponse{
				Success: false,
				Message: "Upload timeout. The file is too large or processing is taking too long.",
			})
			return
		}
		
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("Document uploaded successfully",
		zap.String("filename", header.Filename),
		zap.Uint("document_id", doc.ID),
		zap.Int("chunk_count", chunkCount))
	
	c.JSON(http.StatusOK, UploadResponse{
		Success:    true,
		Message:    "Document uploaded successfully",
		DocumentID: doc.ID,
		ChunkCount: chunkCount,
	})
}

// Search 搜索文档
// @Summary 搜索文档
// @Description 在知识库中搜索相关文档
// @Tags 文档管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} SearchResponse "搜索结果"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Router /api/documents/search [post]
func (h *DocumentHandler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// 搜索文档
	docs, err := h.docService.SearchDocuments(
		c.Request.Context(),
		req.Query,
		req.KnowledgeBaseID,
		req.TopK,
	)
	if err != nil {
		h.logger.Error("Failed to search documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to search documents",
		})
		return
	}

	// 转换结果
	results := make([]DocResult, 0, len(docs))
	for _, doc := range docs {
		score := 0.0
		if v, ok := doc.MetaData["score"].(float64); ok {
			score = v
		}
		
		results = append(results, DocResult{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    score,
			Metadata: doc.MetaData,
		})
	}

	c.JSON(http.StatusOK, SearchResponse{
		Success:   true,
		Query:     req.Query,
		Documents: results,
		Timestamp: time.Now().Unix(),
	})
}

// List 获取文档列表
// @Summary 获取文档列表
// @Description 获取指定知识库的文档列表
// @Tags 文档管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param kb_id path int true "知识库ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} DocumentListResponse "文档列表"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Router /api/knowledge-bases/{kb_id}/documents [get]
func (h *DocumentHandler) List(c *gin.Context) {
	// 获取知识库ID
	kbID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid knowledge base ID",
		})
		return
	}

	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 获取文档列表
	docs, total, err := h.docService.GetDocumentsByKB(uint(kbID), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get documents",
		})
		return
	}

	// 转换结果
	docInfos := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		docInfos[i] = DocumentInfo{
			ID:              doc.ID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			FileName:        doc.FileName,
			FileSize:        doc.FileSize,
			Hash:            doc.Hash,
			CreatorID:       doc.CreatorID,
			CreatedAt:       doc.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, DocumentListResponse{
		Success:   true,
		Documents: docInfos,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	})
}

// Delete 删除文档
// @Summary 删除文档
// @Description 删除指定文档
// @Tags 文档管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "文档ID"
// @Success 200 {object} SuccessResponse "删除成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 404 {object} ErrorResponse "文档不存在"
// @Router /api/documents/{id} [delete]
func (h *DocumentHandler) Delete(c *gin.Context) {
	// 获取文档ID
	docID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid document ID",
		})
		return
	}

	// 删除文档
	if err := h.docService.DeleteDocument(c.Request.Context(), uint(docID)); err != nil {
		h.logger.Error("Failed to delete document", zap.Error(err))
		
		status := http.StatusInternalServerError
		message := "Failed to delete document"
		
		if err.Error() == "document not found" {
			status = http.StatusNotFound
			message = err.Error()
		}
		
		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Document deleted successfully",
	})
}

// ListAll 获取所有文档列表
// @Summary 获取所有文档列表
// @Description 获取系统中所有文档的列表（管理员接口）
// @Tags 文档管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} DocumentListResponse "文档列表"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Router /api/documents [get]
func (h *DocumentHandler) ListAll(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 获取文档列表
	docs, total, err := h.docService.GetAllDocuments(page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get all documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get documents",
		})
		return
	}

	// 转换结果
	docInfos := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		docInfo := DocumentInfo{
			ID:              doc.ID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			FileName:        doc.FileName,
			FileSize:        doc.FileSize,
			Hash:            doc.Hash,
			CreatorID:       doc.CreatorID,
			CreatedAt:       doc.CreatedAt,
		}
		
		// 如果预加载了知识库信息，添加知识库名称
		if doc.KnowledgeBase != nil {
			docInfo.KnowledgeBaseName = doc.KnowledgeBase.Name
		}
		
		docInfos[i] = docInfo
	}

	c.JSON(http.StatusOK, DocumentListResponse{
		Success:   true,
		Documents: docInfos,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	})
}