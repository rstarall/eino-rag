package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/db"
	"eino-rag/internal/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SystemHandler struct {
	config *config.Config
	logger *zap.Logger
}

// 配置更新互斥锁，防止并发更新
var configUpdateMutex sync.Mutex

func NewSystemHandler(cfg *config.Config, logger *zap.Logger) *SystemHandler {
	return &SystemHandler{
		config: cfg,
		logger: logger,
	}
}

// Health 健康检查
// @Summary 健康检查
// @Description 检查服务健康状态
// @Tags 系统
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "服务健康"
// @Router /api/health [get]
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
		Service:   "eino-rag",
		Version:   "1.0.0",
	})
}

// GetConfig 获取系统配置
// @Summary 获取系统配置
// @Description 获取系统配置信息
// @Tags 系统
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} SystemConfigResponse "系统配置"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Router /api/system/config [get]
func (h *SystemHandler) GetConfig(c *gin.Context) {
	// 检查是否为管理员
	roleName, _ := c.Get("role_name")
	if roleName != "admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Success: false,
			Message: "Admin permission required",
		})
		return
	}

	// 从 Go 配置变量读取所有配置
	configMap := make(map[string]interface{})
	
	// Server 配置
	configMap["server_port"] = h.config.ServerPort
	configMap["server_host"] = h.config.ServerHost
	configMap["gin_mode"] = h.config.GinMode
	
	// Database 配置
	configMap["db_path"] = h.config.DBPath
	
	// Redis 配置
	configMap["redis_url"] = h.config.RedisURL
	configMap["redis_db"] = h.config.RedisDB
	configMap["redis_password"] = h.config.RedisPassword
	
	// Milvus 配置
	configMap["milvus_address"] = h.config.MilvusAddress
	configMap["collection_name"] = h.config.CollectionName
	configMap["vector_dimension"] = h.config.VectorDimension
	configMap["metric_type"] = h.config.MetricType
	configMap["index_type"] = h.config.IndexType
	
	// Ollama 配置
	configMap["ollama_base_url"] = h.config.OllamaBaseURL
	configMap["embedding_model"] = h.config.EmbeddingModel
	configMap["llm_model"] = h.config.LLMModel
	
	// OpenAI 配置
	configMap["openai_api_key"] = h.config.OpenAIAPIKey
	configMap["openai_model"] = h.config.OpenAIModel
	configMap["openai_base_url"] = h.config.OpenAIBaseURL
	
	// RAG 配置
	configMap["chunk_size"] = h.config.ChunkSize
	configMap["chunk_overlap"] = h.config.ChunkOverlap
	configMap["chunking_strategy"] = string(h.config.ChunkingStrategy)
	configMap["top_k"] = h.config.TopK
	configMap["score_threshold"] = h.config.ScoreThreshold
	configMap["embedding_cache"] = h.config.EmbeddingCache
	
	// Authentication 配置
	configMap["jwt_secret"] = h.config.JWTSecret
	configMap["jwt_expire_hours"] = h.config.JWTExpireHours
	configMap["session_secret"] = h.config.SessionSecret
	
	// Upload 配置
	configMap["max_upload_size"] = h.config.MaxUploadSize
	configMap["allowed_file_types"] = h.config.AllowedFileTypes
	
	// Timeouts 配置（转换为秒）
	configMap["index_timeout"] = h.config.IndexTimeout.Seconds()
	configMap["milvus_insert_timeout"] = h.config.MilvusInsertTimeout.Seconds()
	configMap["milvus_connect_timeout"] = h.config.MilvusConnectTimeout.Seconds()
	configMap["grpc_keepalive_time"] = h.config.GRPCKeepaliveTime.Seconds()
	configMap["embedding_timeout"] = h.config.EmbeddingTimeout.Seconds()
	configMap["grpc_keepalive_timeout"] = h.config.GRPCKeepaliveTimeout.Seconds()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": configMap,
	})
}

// UpdateConfig 更新系统配置
// @Summary 更新系统配置
// @Description 更新系统配置信息
// @Tags 系统
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body SystemConfigRequest true "配置信息"
// @Success 200 {object} SuccessResponse "更新成功"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Router /api/system/config [put]
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	// 检查是否为管理员
	roleName, _ := c.Get("role_name")
	if roleName != "admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Success: false,
			Message: "Admin permission required",
		})
		return
	}

	// 解析请求
	var req SystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// 加锁防止并发更新
	configUpdateMutex.Lock()
	defer configUpdateMutex.Unlock()

	database := db.GetDB()
	
	// 更新配置，带重试逻辑
	var err error
	for i := 0; i < 3; i++ {
		err = database.Transaction(func(tx *gorm.DB) error {
		for key, value := range req.Configs {
			// 将值转换为字符串存储
			var valueStr string
			switch v := value.(type) {
			case string:
				valueStr = v
			case float64:
				valueStr = strconv.FormatFloat(v, 'f', -1, 64)
			case int:
				valueStr = strconv.Itoa(v)
			case bool:
				valueStr = strconv.FormatBool(v)
			case []interface{}:
				// 处理数组类型（如 allowed_file_types）
				var strSlice []string
				for _, item := range v {
					if s, ok := item.(string); ok {
						strSlice = append(strSlice, s)
					}
				}
				valueStr = strings.Join(strSlice, ",")
			default:
				// 尝试将其他类型转换为JSON字符串
				if jsonBytes, err := json.Marshal(v); err == nil {
					valueStr = string(jsonBytes)
				} else {
					valueStr = ""
				}
			}
			
			config := models.SystemConfig{
				Key:   key,
				Value: valueStr,
			}
			
			// 使用更高效的 Save 方法
			if err := tx.Save(&config).Error; err != nil {
				return err
			}
		}
		return nil
	})
	
		// 如果没有错误或不是数据库锁定错误，则跳出循环
		if err == nil || !strings.Contains(err.Error(), "database is locked") {
			break
		}
		
		// 重试前等待
		time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
	}

	if err != nil {
		h.logger.Error("Failed to update system config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to update system config",
		})
		return
	}

	// 从数据库重新加载配置到内存
	var updatedConfigs []models.SystemConfig
	if err := database.Find(&updatedConfigs).Error; err == nil {
		configMap := make(map[string]string)
		for _, cfg := range updatedConfigs {
			configMap[cfg.Key] = cfg.Value
		}
		// 更新内存中的配置
		config.UpdateFromDB(configMap)
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "System config updated successfully",
	})
}

// GetStats 获取系统统计
// @Summary 获取系统统计
// @Description 获取系统统计信息
// @Tags 系统
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]interface{} "统计信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/system/stats [get]
func (h *SystemHandler) GetStats(c *gin.Context) {
	database := db.GetDB()
	
	stats := make(map[string]interface{})
	
	// 用户统计
	var userCount int64
	database.Model(&models.User{}).Count(&userCount)
	stats["user_count"] = userCount
	
	// 知识库统计
	var kbCount int64
	database.Model(&models.KnowledgeBase{}).Count(&kbCount)
	stats["knowledge_base_count"] = kbCount
	
	// 文档统计
	var docCount int64
	database.Model(&models.Document{}).Count(&docCount)
	stats["document_count"] = docCount
	
	// 对话统计
	var chatCount int64
	database.Model(&models.ChatHistory{}).Count(&chatCount)
	stats["chat_count"] = chatCount
	
	// 今日新增用户
	var todayUsers int64
	today := time.Now().Format("2006-01-02")
	database.Model(&models.User{}).Where("DATE(created_at) = ?", today).Count(&todayUsers)
	stats["today_new_users"] = todayUsers
	
	// 今日新增文档
	var todayDocs int64
	database.Model(&models.Document{}).Where("DATE(created_at) = ?", today).Count(&todayDocs)
	stats["today_new_documents"] = todayDocs
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}