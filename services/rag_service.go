package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eino-rag/components"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type RAGService struct {
	retriever   *components.MilvusRetriever
	processor   *components.DocumentProcessor
	searchChain interface{} // 临时使用interface{}，因为类型定义可能有变化
	logger      *zap.Logger
}

func NewRAGService(retriever *components.MilvusRetriever, processor *components.DocumentProcessor, logger *zap.Logger) (*RAGService, error) {
	ctx := context.Background()

	service := &RAGService{
		retriever: retriever,
		processor: processor,
		logger:    logger,
	}

	// 构建RAG搜索链
	if err := service.buildSearchChain(ctx); err != nil {
		return nil, fmt.Errorf("failed to build search chain: %w", err)
	}

	return service, nil
}

func (s *RAGService) buildSearchChain(ctx context.Context) error {
	// 根据官方文档创建RAG搜索链
	// 使用Lambda函数进行文档检索
	retrieveLambda := compose.InvokableLambda(func(ctx context.Context, query string) ([]*schema.Document, error) {
		return s.retriever.Retrieve(ctx, query)
	})

	// 创建搜索链
	searchChain := compose.NewChain[string, []*schema.Document]()
	searchChain.AppendLambda(retrieveLambda)

	// 编译链
	runnable, err := searchChain.Compile(ctx)
	if err != nil {
		return fmt.Errorf("compile search chain: %w", err)
	}

	s.searchChain = runnable
	return nil
}

func (s *RAGService) IndexDocument(ctx context.Context, content string, metadata map[string]interface{}) error {
	s.logger.Info("开始索引文档",
		zap.Int("content_length", len(content)),
		zap.Any("metadata", metadata))

	// 处理文档为chunks（使用语义分割）
	s.logger.Debug("开始文档处理和分块")
	processStart := time.Now()
	chunks, err := s.processor.ProcessText(content, metadata)
	processDuration := time.Since(processStart)

	if err != nil {
		s.logger.Error("文档处理失败",
			zap.Error(err),
			zap.Duration("process_duration", processDuration))
		return fmt.Errorf("failed to process document: %w", err)
	}

	s.logger.Info("文档处理完成",
		zap.Int("chunk_count", len(chunks)),
		zap.Duration("process_duration", processDuration))

	// 记录分块信息
	for i, chunk := range chunks {
		s.logger.Debug("分块信息",
			zap.Int("chunk_index", i),
			zap.String("chunk_id", chunk.ID),
			zap.Int("chunk_length", len(chunk.Content)),
			zap.String("chunk_preview", chunk.Content[:min(50, len(chunk.Content))]))
	}

	// 添加到检索器
	s.logger.Debug("开始添加文档到向量数据库")
	addStart := time.Now()
	err = s.retriever.AddDocuments(ctx, chunks)
	addDuration := time.Since(addStart)

	if err != nil {
		s.logger.Error("添加文档到向量数据库失败",
			zap.Error(err),
			zap.Duration("add_duration", addDuration))
		return err
	}

	s.logger.Info("文档索引完成",
		zap.Int("chunk_count", len(chunks)),
		zap.Duration("add_duration", addDuration))

	return nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *RAGService) Search(ctx context.Context, query string) ([]*schema.Document, error) {
	// 使用Chain进行搜索
	if s.searchChain != nil {
		// 类型断言后调用
		if runnable, ok := s.searchChain.(interface {
			Invoke(context.Context, string) ([]*schema.Document, error)
		}); ok {
			return runnable.Invoke(ctx, query)
		}
	}

	// 回退到直接调用
	return s.retriever.Retrieve(ctx, query)
}

func (s *RAGService) SearchWithContext(ctx context.Context, query string) (string, []*schema.Document, error) {
	// 使用Chain进行搜索
	docs, err := s.Search(ctx, query)
	if err != nil {
		return "", nil, err
	}

	// 构建上下文字符串
	var contexts []string
	for _, doc := range docs {
		contexts = append(contexts, doc.Content)
	}

	return strings.Join(contexts, "\n\n"), docs, nil
}

// GetProcessingStats 获取文档处理统计信息
func (s *RAGService) GetProcessingStats() map[string]interface{} {
	return s.processor.GetProcessingStats()
}

// ClearProcessingCache 清空处理缓存
func (s *RAGService) ClearProcessingCache() {
	s.processor.ClearCache()
}

// SetSemanticSplitting 动态启用/禁用语义分割
func (s *RAGService) SetSemanticSplitting(enable bool) error {
	return s.processor.SetSemanticSplitting(enable)
}

// GetDocuments 获取已索引的文档列表
func (s *RAGService) GetDocuments(ctx context.Context) ([]map[string]interface{}, error) {
	s.logger.Debug("获取文档列表")

	documents, err := s.retriever.GetDocumentsList(ctx)
	if err != nil {
		s.logger.Error("获取文档列表失败", zap.Error(err))
		return nil, fmt.Errorf("failed to get documents list: %w", err)
	}

	s.logger.Info("成功获取文档列表", zap.Int("document_count", len(documents)))
	return documents, nil
}
