package services

import (
	"context"
	"fmt"
	"strings"

	"eino-rag/components"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type RAGService struct {
	retriever   *components.MilvusRetriever
	processor   *components.DocumentProcessor
	searchChain interface{} // 临时使用interface{}，因为类型定义可能有变化
}

func NewRAGService(retriever *components.MilvusRetriever, processor *components.DocumentProcessor) (*RAGService, error) {
	ctx := context.Background()

	service := &RAGService{
		retriever: retriever,
		processor: processor,
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
	// 处理文档为chunks（使用语义分割）
	chunks, err := s.processor.ProcessText(content, metadata)
	if err != nil {
		return fmt.Errorf("failed to process document: %w", err)
	}

	// 添加到检索器
	return s.retriever.AddDocuments(ctx, chunks)
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
