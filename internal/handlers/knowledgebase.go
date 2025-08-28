package handlers

import (
	"net/http"
	"strconv"
	"time"

	"eino-rag/internal/db"
	"eino-rag/internal/models"
	"eino-rag/internal/services/rag"
	"gorm.io/gorm"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type KnowledgeBaseHandler struct {
	retriever *rag.MilvusRetriever
	logger    *zap.Logger
}

func NewKnowledgeBaseHandler(retriever *rag.MilvusRetriever, logger *zap.Logger) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		retriever: retriever,
		logger:    logger,
	}
}

// Create 创建知识库
// @Summary 创建知识库
// @Description 创建新的知识库
// @Tags 知识库
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body CreateKBRequest true "创建请求"
// @Success 200 {object} models.KnowledgeBase "创建成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/knowledge-bases [post]
func (h *KnowledgeBaseHandler) Create(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found in context",
		})
		return
	}

	// 解析请求
	var req CreateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// 创建知识库
	kb := &models.KnowledgeBase{
		Name:        req.Name,
		Description: req.Description,
		CreatorID:   userID.(uint),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	database := db.GetDB()
	if err := database.Create(kb).Error; err != nil {
		h.logger.Error("Failed to create knowledge base", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to create knowledge base",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"knowledge_base": kb,
	})
}

// List 获取知识库列表
// @Summary 获取知识库列表
// @Description 获取所有知识库列表
// @Tags 知识库
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} KBListResponse "知识库列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/knowledge-bases [get]
func (h *KnowledgeBaseHandler) List(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	database := db.GetDB()
	
	// 计算总数
	var total int64
	if err := database.Model(&models.KnowledgeBase{}).Count(&total).Error; err != nil {
		h.logger.Error("Failed to count knowledge bases", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get knowledge bases",
		})
		return
	}

	// 分页查询
	var kbs []models.KnowledgeBase
	offset := (page - 1) * pageSize
	if err := database.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&kbs).Error; err != nil {
		h.logger.Error("Failed to get knowledge bases", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get knowledge bases",
		})
		return
	}

	// 转换结果
	kbWithDocs := make([]KnowledgeBaseWithDocs, len(kbs))
	for i, kb := range kbs {
		kbWithDocs[i] = KnowledgeBaseWithDocs{
			ID:          kb.ID,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			CreatorID:   kb.CreatorID,
			CreatedAt:   kb.CreatedAt,
			UpdatedAt:   kb.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, KBListResponse{
		Success:        true,
		KnowledgeBases: kbWithDocs,
		Total:          total,
		Page:           page,
		PageSize:       pageSize,
	})
}

// Get 获取知识库详情
// @Summary 获取知识库详情
// @Description 获取指定知识库的详细信息
// @Tags 知识库
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "知识库ID"
// @Success 200 {object} models.KnowledgeBase "知识库详情"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 404 {object} ErrorResponse "知识库不存在"
// @Router /api/knowledge-bases/{id} [get]
func (h *KnowledgeBaseHandler) Get(c *gin.Context) {
	// 获取知识库ID
	kbID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid knowledge base ID",
		})
		return
	}

	database := db.GetDB()
	
	var kb models.KnowledgeBase
	if err := database.First(&kb, kbID).Error; err != nil {
		h.logger.Error("Failed to get knowledge base", zap.Error(err))
		
		status := http.StatusInternalServerError
		message := "Failed to get knowledge base"
		
		if err.Error() == "record not found" {
			status = http.StatusNotFound
			message = "Knowledge base not found"
		}
		
		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"knowledge_base": kb,
	})
}

// Update 更新知识库
// @Summary 更新知识库
// @Description 更新知识库信息
// @Tags 知识库
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "知识库ID"
// @Param request body UpdateKBRequest true "更新请求"
// @Success 200 {object} SuccessResponse "更新成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 404 {object} ErrorResponse "知识库不存在"
// @Router /api/knowledge-bases/{id} [put]
func (h *KnowledgeBaseHandler) Update(c *gin.Context) {
	// 获取知识库ID
	kbID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid knowledge base ID",
		})
		return
	}

	// 解析请求
	var req UpdateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	database := db.GetDB()
	
	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	updates["updated_at"] = time.Now()

	// 执行更新
	result := database.Model(&models.KnowledgeBase{}).Where("id = ?", kbID).Updates(updates)
	if result.Error != nil {
		h.logger.Error("Failed to update knowledge base", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to update knowledge base",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Message: "Knowledge base not found",
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Knowledge base updated successfully",
	})
}

// Delete 删除知识库
// @Summary 删除知识库
// @Description 删除知识库及其所有文档
// @Tags 知识库
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "知识库ID"
// @Success 200 {object} SuccessResponse "删除成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 404 {object} ErrorResponse "知识库不存在"
// @Router /api/knowledge-bases/{id} [delete]
func (h *KnowledgeBaseHandler) Delete(c *gin.Context) {
	// 获取知识库ID
	kbID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid knowledge base ID",
		})
		return
	}

	database := db.GetDB()
	
	// 开始事务
	err = database.Transaction(func(tx *gorm.DB) error {
		// 检查知识库是否存在
		var kb models.KnowledgeBase
		if err := tx.First(&kb, kbID).Error; err != nil {
			return err
		}

		// 删除向量数据库中的文档
		if h.retriever != nil {
			if err := h.retriever.DeleteByKnowledgeBase(c.Request.Context(), uint(kbID)); err != nil {
				h.logger.Error("Failed to delete vectors", zap.Error(err))
				// 继续删除，不中断流程
			}
		} else {
			h.logger.Warn("Vector deletion skipped - retriever not available", zap.Uint64("kb_id", kbID))
		}

		// 删除所有文档记录
		if err := tx.Where("knowledge_base_id = ?", kbID).Delete(&models.Document{}).Error; err != nil {
			return err
		}

		// 删除知识库
		if err := tx.Delete(&kb).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		h.logger.Error("Failed to delete knowledge base", zap.Error(err))
		
		status := http.StatusInternalServerError
		message := "Failed to delete knowledge base"
		
		if err.Error() == "record not found" {
			status = http.StatusNotFound
			message = "Knowledge base not found"
		}
		
		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Knowledge base deleted successfully",
	})
}