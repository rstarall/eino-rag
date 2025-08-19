package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

// 添加到 APIHandler 结构体
func (h *APIHandler) SetupFrontendRoutes(router *gin.Engine) {
	// 静态文件
	router.Static("/static", "./frontend/static")

	// 主页
	router.GET("/", h.HomePage)

	// API 路由组
	api := router.Group("/api/v1")
	{
		api.GET("/health", h.Health)
		api.POST("/upload", h.Upload)
		api.POST("/search", h.Search)
		api.GET("/chat/stream", h.ChatStream) // SSE 端点
		api.GET("/documents", h.GetDocuments)
		api.GET("/stats", h.GetProcessingStats) // 处理统计信息
	}
}

// HomePage 渲染主页
func (h *APIHandler) HomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "RAG Chat System",
	})
}

// ChatStream SSE流式响应
func (h *APIHandler) ChatStream(c *gin.Context) {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取查询参数
	query := c.Query("query")
	if query == "" {
		h.sendSSEEvent(c.Writer, "error", gin.H{"message": "Query parameter is required"})
		return
	}

	topK := 5
	if topKStr := c.Query("top_k"); topKStr != "" {
		if parsed, err := strconv.Atoi(topKStr); err == nil {
			topK = parsed
		}
	}
	returnContext := c.Query("return_context") == "true"

	// 创建SSE writer
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// 发送开始事件
	h.sendSSEEvent(w, "start", gin.H{"message": "Starting search"})
	flusher.Flush()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 执行搜索
	docs, err := h.ragService.Search(ctx, query)
	if err != nil {
		h.sendSSEEvent(w, "error", gin.H{"message": err.Error()})
		flusher.Flush()
		return
	}

	// 如果需要，发送上下文
	if returnContext && len(docs) > 0 {
		h.sendSSEEvent(w, "context", gin.H{
			"documents": h.convertDocsForSSE(docs[:min(topK, len(docs))]),
		})
		flusher.Flush()
	}

	// 构建RAG上下文消息
	messages := h.buildRAGMessages(query, docs, topK)

	// 使用 Eino OpenAI 流式接口
	if h.chatModel != nil {
		h.streamWithOpenAI(ctx, w, flusher, messages)
	} else {
		// 如果没有配置 OpenAI，回退到原来的模拟响应
		h.streamFallback(w, flusher, query, docs)
	}
}

// sendSSEEvent 发送SSE事件
func (h *APIHandler) sendSSEEvent(w http.ResponseWriter, eventType string, data interface{}) {
	sseData := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	jsonData, _ := json.Marshal(sseData)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
}

// convertDocsForSSE 转换文档格式用于SSE
func (h *APIHandler) convertDocsForSSE(docs []*schema.Document) []map[string]interface{} {
	results := make([]map[string]interface{}, len(docs))
	for i, doc := range docs {
		results[i] = map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.Content,
			"score":    0.0, // Score字段暂时设为0
			"metadata": doc.MetaData,
		}
	}
	return results
}

// buildRAGMessages 构建RAG消息
func (h *APIHandler) buildRAGMessages(query string, docs []*schema.Document, topK int) []*schema.Message {
	messages := []*schema.Message{
		// 系统消息：定义AI助手的角色
		schema.SystemMessage(`你是一个专业的AI助手，擅长基于提供的文档内容回答问题。
请根据以下原则回答用户问题：
1. 优先使用提供的文档内容来回答问题
2. 如果文档内容不足以回答问题，请明确说明
3. 保持回答的准确性和相关性
4. 用中文回答问题`),
	}

	// 如果有检索到的文档，添加上下文
	if len(docs) > 0 {
		var contextBuilder strings.Builder
		contextBuilder.WriteString("以下是相关的文档内容：\n\n")

		// 限制文档数量和长度
		maxDocs := min(topK, len(docs))
		for i, doc := range docs[:maxDocs] {
			maxLength := min(500, len(doc.Content))
			contextBuilder.WriteString(fmt.Sprintf("文档 %d (相关度: %.3f):\n%s\n\n",
				i+1, doc.Score, doc.Content[:maxLength]))
		}

		// 添加上下文消息
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: contextBuilder.String(),
		})
	}

	// 添加用户问题
	messages = append(messages, schema.UserMessage(query))

	return messages
}

// streamWithOpenAI 使用 Eino OpenAI 流式接口
func (h *APIHandler) streamWithOpenAI(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, messages []*schema.Message) {
	// 获取流式回复
	reader, err := h.chatModel.Stream(ctx, messages)
	if err != nil {
		h.sendSSEEvent(w, "error", gin.H{"message": fmt.Sprintf("OpenAI stream error: %v", err)})
		flusher.Flush()
		return
	}
	defer reader.Close()

	// 处理流式内容
	for {
		chunk, err := reader.Recv()
		if err != nil {
			// 流结束或出错
			break
		}

		// 发送内容块
		if chunk.Content != "" {
			h.sendSSEEvent(w, "content", gin.H{
				"content": chunk.Content,
			})
			flusher.Flush()
		}
	}

	// 发送结束事件
	h.sendSSEEvent(w, "end", gin.H{"message": "Completed"})
	flusher.Flush()
}

// streamFallback 回退的模拟流式响应
func (h *APIHandler) streamFallback(w http.ResponseWriter, flusher http.Flusher, query string, docs []*schema.Document) {
	response := h.generateFallbackResponse(query, docs)
	words := strings.Fields(response)

	// 分批发送内容
	for i, word := range words {
		h.sendSSEEvent(w, "content", gin.H{
			"content": word + " ",
		})
		flusher.Flush()

		// 模拟打字效果
		if i < len(words)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 发送结束事件
	h.sendSSEEvent(w, "end", gin.H{"message": "Completed"})
	flusher.Flush()
}

// generateFallbackResponse 生成回退响应
func (h *APIHandler) generateFallbackResponse(query string, docs []*schema.Document) string {
	if len(docs) == 0 {
		return "抱歉，我没有找到相关的文档来回答您的问题。请先上传相关文档，或者尝试更改您的问题。注意：当前未配置OpenAI，使用的是模拟回复。"
	}

	var context strings.Builder
	context.WriteString("基于检索到的文档，我找到了以下相关信息（注意：当前未配置OpenAI，使用的是简单摘要）：\n\n")

	for i, doc := range docs[:min(3, len(docs))] {
		maxLength := min(200, len(doc.Content))
		context.WriteString(fmt.Sprintf("%d. %s\n\n", i+1, doc.Content[:maxLength]))
	}

	return context.String()
}

// GetDocuments 获取已索引的文档列表
func (h *APIHandler) GetDocuments(c *gin.Context) {
	// 这里应该从数据库获取文档列表
	// 示例返回
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"count":     0,
		"documents": []gin.H{},
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
