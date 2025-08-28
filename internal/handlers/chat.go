package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"eino-rag/internal/db"
	"eino-rag/internal/models"
	"eino-rag/internal/services/chat"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ChatHandler struct {
	chatService *chat.Service
	logger      *zap.Logger
}

func NewChatHandler(chatService *chat.Service, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		logger:      logger,
	}
}

// Chat 处理聊天请求
// @Summary 发送聊天消息
// @Description 发送消息并获取AI回复
// @Tags 聊天
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body ChatRequest true "聊天请求"
// @Success 200 {object} ChatResponse "聊天回复"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/chat [post]
func (h *ChatHandler) Chat(c *gin.Context) {
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
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// 处理聊天
	reply, convID, context, err := h.chatService.Chat(
		c.Request.Context(),
		req.Message,
		req.ConversationID,
		userID.(uint),
		req.KnowledgeBaseID,
		req.UseRAG,
	)
	if err != nil {
		h.logger.Error("Failed to process chat", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to process chat request",
		})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{
		Success:        true,
		Message:        reply,
		ConversationID: convID,
		Context:        context,
		Timestamp:      time.Now().Unix(),
	})
}

// ListConversations 获取对话列表
// @Summary 获取对话列表
// @Description 获取当前用户的对话历史列表
// @Tags 聊天
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} ConversationListResponse "对话列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/chat/conversations [get]
func (h *ChatHandler) ListConversations(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found in context",
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

	// 获取对话列表
	conversations, total, err := h.chatService.GetUserConversations(userID.(uint), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get conversations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get conversations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"conversations": conversations,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

// GetConversation 获取对话详情
// @Summary 获取对话详情
// @Description 获取指定对话的所有消息
// @Tags 聊天
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "对话ID"
// @Success 200 {object} ConversationDetailResponse "对话详情"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "对话不存在"
// @Router /api/chat/conversations/{id} [get]
func (h *ChatHandler) GetConversation(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found in context",
		})
		return
	}

	// 获取对话ID
	convID := c.Param("id")
	if convID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Conversation ID is required",
		})
		return
	}

	// 获取对话消息
	messages, err := h.chatService.GetConversationMessages(c.Request.Context(), convID, userID.(uint))
	if err != nil {
		h.logger.Error("Failed to get conversation messages", zap.Error(err))

		status := http.StatusInternalServerError
		message := "Failed to get conversation"

		if err.Error() == "conversation not found" {
			status = http.StatusNotFound
			message = err.Error()
		} else if err.Error() == "unauthorized" {
			status = http.StatusForbidden
			message = "You don't have permission to access this conversation"
		}

		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"id":       convID,
		"messages": messages,
	})
}

// ChatStream 处理流式聊天请求
// @Summary 发送聊天消息（流式）
// @Description 发送消息并获取AI流式回复
// @Tags 聊天
// @Accept json
// @Produce text/plain
// @Security ApiKeyAuth
// @Param request body ChatRequest true "聊天请求"
// @Success 200 {string} string "流式回复"
// @Failure 400 {object} ErrorResponse "请求错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/chat/stream [post]
func (h *ChatHandler) ChatStream(c *gin.Context) {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		h.sendSSEEvent(c.Writer, "error", map[string]interface{}{
			"message": "User not found in context",
		})
		return
	}

	// 解析请求
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendSSEEvent(c.Writer, "error", map[string]interface{}{
			"message": "Invalid request data",
		})
		return
	}

	// 创建flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.sendSSEEvent(c.Writer, "error", map[string]interface{}{
			"message": "Streaming not supported",
		})
		return
	}

	// 发送开始事件
	h.sendSSEEvent(c.Writer, "start", map[string]interface{}{
		"conversation_id": req.ConversationID,
		"message":         "Starting chat",
	})
	flusher.Flush()

	// 处理流式聊天
	reader, convID, _, retrievedDocs, err := h.chatService.ChatStream(
		c.Request.Context(),
		req.Message,
		req.ConversationID,
		userID.(uint),
		req.KnowledgeBaseID,
		req.UseRAG,
	)
	if err != nil {
		h.logger.Error("Failed to process stream chat", zap.Error(err))
		h.sendSSEEvent(c.Writer, "error", map[string]interface{}{
			"message": "Failed to process chat request",
		})
		flusher.Flush()
		return
	}
	defer reader.Close()

	// 发送检索到的文档上下文（如果有）
	if len(retrievedDocs) > 0 {
		h.sendSSEEvent(c.Writer, "context", map[string]interface{}{
			"documents": h.convertDocsForSSE(retrievedDocs),
		})
		flusher.Flush()
	}

	// 读取并转发流式内容，同时收集完整回复
	var fullReply strings.Builder
	for {
		chunk, err := reader.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			h.logger.Error("Error reading stream", zap.Error(err))
			break
		}

		if chunk.Content != "" {
			fullReply.WriteString(chunk.Content)
			h.sendSSEEvent(c.Writer, "content", map[string]interface{}{
				"content": chunk.Content,
			})
			flusher.Flush()
		}
	}

	// 异步保存完整对话
	go func() {
		h.saveStreamConversation(userID.(uint), req.Message, fullReply.String(), convID)
	}()

	// 发送结束事件
	h.sendSSEEvent(c.Writer, "end", map[string]interface{}{
		"conversation_id": convID,
		"message":         "Completed",
		"timestamp":       time.Now().Unix(),
	})
	flusher.Flush()
}

// sendSSEEvent 发送SSE事件
func (h *ChatHandler) sendSSEEvent(w http.ResponseWriter, eventType string, data interface{}) {
	sseData := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	jsonData, err := json.Marshal(sseData)
	if err != nil {
		h.logger.Error("Failed to marshal SSE data", zap.Error(err))
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", jsonData)
}

// saveStreamConversation 保存流式聊天对话
func (h *ChatHandler) saveStreamConversation(userID uint, userMessage, assistantReply, conversationID string) {
	ctx := context.Background()

	// 获取或创建对话
	conv, err := db.GetConversation(ctx, conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation for saving", zap.Error(err))
		return
	}

	if conv == nil {
		// 创建新对话
		conv = &models.Conversation{
			ID:        conversationID,
			UserID:    userID,
			Messages:  []models.ChatMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// 添加用户消息
		userMsg := models.ChatMessage{
			Role:      "user",
			Content:   userMessage,
			Timestamp: time.Now(),
		}
		conv.Messages = append(conv.Messages, userMsg)

		// 保存对话历史到数据库（如果是新对话）
		database := db.GetDB()
		title := userMessage
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		history := &models.ChatHistory{
			UserID:         userID,
			ConversationID: conversationID,
			Title:          title,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := database.Create(history).Error; err != nil {
			h.logger.Error("Failed to save chat history", zap.Error(err))
		}
	}

	// 添加助手回复
	assistantMsg := models.ChatMessage{
		Role:      "assistant",
		Content:   assistantReply,
		Timestamp: time.Now(),
	}
	conv.Messages = append(conv.Messages, assistantMsg)
	conv.UpdatedAt = time.Now()

	// 保存对话到Redis
	if err := db.SaveConversation(ctx, conv); err != nil {
		h.logger.Error("Failed to save conversation", zap.Error(err))
	}
}

// convertDocsForSSE 转换文档格式用于SSE
func (h *ChatHandler) convertDocsForSSE(docs []*schema.Document) []map[string]interface{} {
	results := make([]map[string]interface{}, len(docs))
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

		results[i] = map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.Content,
			"score":    score,
			"metadata": doc.MetaData,
		}
	}
	return results
}
