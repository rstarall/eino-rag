package chat

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/db"
	"eino-rag/internal/models"
	"eino-rag/internal/services/document"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	chatModel  *openai.ChatModel
	docService *document.Service
	logger     *zap.Logger
	config     *config.Config
}

func NewService(
	docService *document.Service,
	cfg *config.Config,
	logger *zap.Logger,
) (*Service, error) {
	service := &Service{
		docService: docService,
		logger:     logger,
		config:     cfg,
	}

	// 初始化ChatModel（如果配置了）
	if cfg.OpenAIAPIKey != "" {
		chatModelConfig := &openai.ChatModelConfig{
			APIKey:  cfg.OpenAIAPIKey,
			Model:   cfg.OpenAIModel,
			Timeout: 60 * time.Second,
		}

		if cfg.OpenAIBaseURL != "" {
			chatModelConfig.BaseURL = cfg.OpenAIBaseURL
		}

		var err error
		service.chatModel, err = openai.NewChatModel(context.Background(), chatModelConfig)
		if err != nil {
			logger.Warn("Failed to initialize OpenAI ChatModel", zap.Error(err))
		}
	}

	return service, nil
}

// Chat 处理聊天请求
func (s *Service) Chat(
	ctx context.Context,
	message string,
	conversationID string,
	userID uint,
	kbID uint,
	useRAG bool,
) (string, string, string, error) {
	// 如果没有对话ID，创建新的
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	// 获取或创建对话
	conv, err := s.getOrCreateConversation(ctx, conversationID, userID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get conversation: %w", err)
	}

	// 添加用户消息
	userMsg := models.ChatMessage{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}
	conv.Messages = append(conv.Messages, userMsg)

	// 准备上下文
	var ragContext string
	if useRAG && kbID > 0 {
		// 检索相关文档
		docs, err := s.docService.SearchDocuments(ctx, message, kbID, s.config.TopK)
		if err != nil {
			s.logger.Error("Failed to retrieve documents", zap.Error(err))
		} else if len(docs) > 0 {
			ragContext = s.buildRAGContext(docs)
		}
	}

	// 生成回复
	reply, err := s.generateReply(ctx, message, ragContext, conv.Messages)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate reply: %w", err)
	}

	// 添加助手消息
	assistantMsg := models.ChatMessage{
		Role:      "assistant",
		Content:   reply,
		Timestamp: time.Now(),
	}
	conv.Messages = append(conv.Messages, assistantMsg)
	conv.UpdatedAt = time.Now()

	// 保存对话
	if err := db.SaveConversation(ctx, conv); err != nil {
		s.logger.Error("Failed to save conversation", zap.Error(err))
	}

	// 保存对话历史到数据库（如果是新对话）
	if len(conv.Messages) == 2 { // 第一轮对话
		s.saveConversationHistory(userID, conversationID, message)
	}

	return reply, conversationID, ragContext, nil
}

// ChatStream 处理流式聊天请求
func (s *Service) ChatStream(
	ctx context.Context,
	message string,
	conversationID string,
	userID uint,
	kbID uint,
	useRAG bool,
) (interface {
	Recv() (*schema.Message, error)
	Close()
}, string, string, []*schema.Document, error) {
	// 如果没有对话ID，创建新的
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	// 获取或创建对话
	conv, err := s.getOrCreateConversation(ctx, conversationID, userID)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// 添加用户消息
	userMsg := models.ChatMessage{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}
	conv.Messages = append(conv.Messages, userMsg)

	// 准备上下文
	var ragContext string
	var retrievedDocs []*schema.Document
	if useRAG && kbID > 0 {
		// 检索相关文档
		docs, err := s.docService.SearchDocuments(ctx, message, kbID, s.config.TopK)
		if err != nil {
			s.logger.Error("Failed to retrieve documents", zap.Error(err))
		} else if len(docs) > 0 {
			retrievedDocs = docs
			ragContext = s.buildRAGContext(docs)
		}
	}

	// 生成流式回复
	reader, err := s.generateStreamReply(ctx, message, ragContext, conv.Messages)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("failed to generate stream reply: %w", err)
	}

	// 注意：流式聊天的对话保存需要在handler中处理，因为我们无法在这里收集完整回复

	return reader, conversationID, ragContext, retrievedDocs, nil
}

// generateReply 生成回复
func (s *Service) generateReply(ctx context.Context, message, ragContext string, history []models.ChatMessage) (string, error) {
	// 如果没有配置ChatModel，返回模拟回复
	if s.chatModel == nil {
		if ragContext != "" {
			return fmt.Sprintf("基于检索到的文档内容，这是我的回答：\n\n%s\n\n（注：这是模拟回复，请配置OpenAI API以获得真实的AI回答）",
				s.extractKeyPoints(ragContext)), nil
		}
		return "抱歉，AI模型未配置。请在环境变量中设置OPENAI_API_KEY。", nil
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+2)

	// 添加系统消息
	systemPrompt := "你是一个有帮助的AI助手。"
	if ragContext != "" {
		systemPrompt += fmt.Sprintf("\n\n请基于以下检索到的文档内容回答用户的问题：\n\n%s", ragContext)
	}

	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})

	// 添加历史消息（限制最近10条）
	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}

	for i := start; i < len(history); i++ {
		role := schema.User
		if history[i].Role == "assistant" {
			role = schema.Assistant
		}
		messages = append(messages, &schema.Message{
			Role:    role,
			Content: history[i].Content,
		})
	}

	// 调用ChatModel
	resp, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if resp == nil || resp.Content == "" {
		return "", fmt.Errorf("empty response from model")
	}

	return resp.Content, nil
}

// generateStreamReply 生成流式回复
func (s *Service) generateStreamReply(ctx context.Context, message, ragContext string, history []models.ChatMessage) (interface {
	Recv() (*schema.Message, error)
	Close()
}, error) {
	// 如果没有配置ChatModel，返回模拟流式回复
	if s.chatModel == nil {
		var fallbackResponse string
		if ragContext != "" {
			fallbackResponse = fmt.Sprintf("基于检索到的文档内容，这是我的回答：\n\n%s\n\n（注：这是模拟回复，请配置OpenAI API以获得真实的AI回答）",
				s.extractKeyPoints(ragContext))
		} else {
			fallbackResponse = "抱歉，AI模型未配置。请在环境变量中设置OPENAI_API_KEY。"
		}

		return s.createFallbackStreamReader(fallbackResponse), nil
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+2)

	// 添加系统消息
	systemPrompt := "你是一个有帮助的AI助手。"
	if ragContext != "" {
		systemPrompt += fmt.Sprintf("\n\n请基于以下检索到的文档内容回答用户的问题：\n\n%s", ragContext)
	}

	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})

	// 添加历史消息（限制最近10条）
	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}

	for i := start; i < len(history); i++ {
		role := schema.User
		if history[i].Role == "assistant" {
			role = schema.Assistant
		}
		messages = append(messages, &schema.Message{
			Role:    role,
			Content: history[i].Content,
		})
	}

	// 直接返回ChatModel的Stream结果
	return s.chatModel.Stream(ctx, messages)
}

// buildRAGContext 构建RAG上下文
func (s *Service) buildRAGContext(docs []*schema.Document) string {
	var context strings.Builder

	for i, doc := range docs {
		context.WriteString(fmt.Sprintf("文档 %d:\n", i+1))
		context.WriteString(doc.Content)
		context.WriteString("\n\n")

		// 限制上下文长度
		if context.Len() > 3000 {
			break
		}
	}

	return strings.TrimSpace(context.String())
}

// extractKeyPoints 提取关键点（简单实现）
func (s *Service) extractKeyPoints(context string) string {
	// 简单截取前1500个字符
	if len(context) > 1500 {
		return context[:1500] + "..."
	}
	return context
}

// getOrCreateConversation 获取或创建对话
func (s *Service) getOrCreateConversation(ctx context.Context, convID string, userID uint) (*models.Conversation, error) {
	// 尝试从Redis获取
	conv, err := db.GetConversation(ctx, convID)
	if err != nil {
		return nil, err
	}

	if conv == nil {
		// 创建新对话
		conv = &models.Conversation{
			ID:        convID,
			UserID:    userID,
			Messages:  []models.ChatMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	return conv, nil
}

// saveConversationHistory 保存对话历史到数据库
func (s *Service) saveConversationHistory(userID uint, convID string, firstMessage string) {
	database := db.GetDB()

	// 提取标题（取前50个字符）
	title := firstMessage
	if len(title) > 50 {
		title = title[:50] + "..."
	}

	history := &models.ChatHistory{
		UserID:         userID,
		ConversationID: convID,
		Title:          title,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := database.Create(history).Error; err != nil {
		s.logger.Error("Failed to save chat history", zap.Error(err))
	}
}

// GetUserConversations 获取用户的对话列表
func (s *Service) GetUserConversations(userID uint, page, pageSize int) ([]models.ChatHistory, int64, error) {
	database := db.GetDB()

	var total int64
	var histories []models.ChatHistory

	// 计算总数
	if err := database.Model(&models.ChatHistory{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := database.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

// GetConversationMessages 获取对话消息
func (s *Service) GetConversationMessages(ctx context.Context, convID string, userID uint) ([]models.ChatMessage, error) {
	conv, err := db.GetConversation(ctx, convID)
	if err != nil {
		return nil, err
	}

	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	// 验证用户权限
	if conv.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	return conv.Messages, nil
}

// createFallbackStreamReader 创建模拟StreamReader
func (s *Service) createFallbackStreamReader(response string) *fallbackStreamReader {
	words := strings.Fields(response)

	return &fallbackStreamReader{
		words:   words,
		current: 0,
		content: response,
	}
}

// fallbackStreamReader 模拟StreamReader
type fallbackStreamReader struct {
	words   []string
	current int
	content string
	closed  bool
}

func (r *fallbackStreamReader) Recv() (*schema.Message, error) {
	if r.closed || r.current >= len(r.words) {
		return nil, io.EOF
	}

	// 模拟打字效果
	time.Sleep(100 * time.Millisecond)

	word := r.words[r.current]
	r.current++

	// 如果不是最后一个词，添加空格
	content := word
	if r.current < len(r.words) {
		content += " "
	}

	return &schema.Message{
		Role:    schema.Assistant,
		Content: content,
	}, nil
}

func (r *fallbackStreamReader) Close() {
	r.closed = true
}
