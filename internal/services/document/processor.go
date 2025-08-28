package document

import (
	"fmt"
	"strings"

	"eino-rag/internal/config"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DocumentProcessor struct {
	chunkSize        int
	chunkOverlap     int
	chunkingStrategy config.ChunkingStrategy
	logger           *zap.Logger
}

func NewDocumentProcessor(cfg *config.Config, logger *zap.Logger) *DocumentProcessor {
	return &DocumentProcessor{
		chunkSize:        cfg.ChunkSize,
		chunkOverlap:     cfg.ChunkOverlap,
		chunkingStrategy: cfg.ChunkingStrategy,
		logger:           logger,
	}
}

// ProcessText 处理文本并分块
func (p *DocumentProcessor) ProcessText(content string, metadata map[string]interface{}) ([]*schema.Document, error) {
	p.logger.Info("Starting text processing",
		zap.Int("content_length", len(content)),
		zap.String("strategy", string(p.chunkingStrategy)))
	
	// 清理文本
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty content")
	}

	// 根据策略进行分块
	var chunks []string
	var err error

	p.logger.Info("Starting content splitting",
		zap.String("strategy", string(p.chunkingStrategy)))
		
	switch p.chunkingStrategy {
	case config.ChunkingStrategyLength:
		chunks = p.splitByLength(content)
	case config.ChunkingStrategySemantic:
		chunks = p.splitBySemantic(content)
	default:
		chunks = p.splitByLength(content)
	}
	
	p.logger.Info("Content splitting completed",
		zap.Int("chunk_count", len(chunks)))

	if err != nil {
		return nil, fmt.Errorf("failed to split content: %w", err)
	}

	// 创建文档对象
	p.logger.Info("Creating document objects from chunks")
	documents := make([]*schema.Document, 0, len(chunks))
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		doc := &schema.Document{
			ID:      uuid.New().String(),
			Content: chunk,
			MetaData: map[string]interface{}{
				"chunk_index": i,
				"total_chunks": len(chunks),
			},
		}

		// 合并用户提供的元数据
		for k, v := range metadata {
			doc.MetaData[k] = v
		}

		documents = append(documents, doc)
		
		if i > 0 && i%100 == 0 {
			p.logger.Info("Document creation progress",
				zap.Int("processed", i),
				zap.Int("total", len(chunks)))
		}
	}

	p.logger.Info("Processed document",
		zap.Int("total_chunks", len(documents)),
		zap.String("strategy", string(p.chunkingStrategy)))

	return documents, nil
}

// splitByLength 基于长度的分块（支持滑动窗口）
func (p *DocumentProcessor) splitByLength(content string) []string {
	p.logger.Debug("splitByLength started",
		zap.Int("content_length", len(content)),
		zap.Int("chunk_size", p.chunkSize),
		zap.Int("chunk_overlap", p.chunkOverlap))
	
	if len(content) <= p.chunkSize {
		p.logger.Debug("Content smaller than chunk size, returning as single chunk")
		return []string{content}
	}

	var chunks []string
	start := 0
	iteration := 0

	for start < len(content) {
		iteration++
		if iteration > 1000 {
			p.logger.Error("Too many iterations in splitByLength, possible infinite loop",
				zap.Int("iteration", iteration),
				zap.Int("start", start),
				zap.Int("content_length", len(content)))
			break
		}
		end := start + p.chunkSize
		if end > len(content) {
			end = len(content)
		}

		// 尝试在单词边界处分割
		if end < len(content) {
			// 向前查找空格
			for i := end; i > start && i > end-50; i-- {
				if content[i] == ' ' || content[i] == '\n' {
					end = i
					break
				}
			}
		}

		chunk := content[start:end]
		chunks = append(chunks, strings.TrimSpace(chunk))

		// 如果已经到达末尾，退出循环
		if end >= len(content) {
			break
		}

		// 计算下一个开始位置（考虑重叠）
		nextStart := end - p.chunkOverlap
		
		// 确保有进展：下一个开始位置必须大于当前开始位置
		if nextStart <= start {
			nextStart = start + 1
		}
		
		start = nextStart
	}

	return chunks
}

// splitBySemantic 基于语义的分块（简化版本）
func (p *DocumentProcessor) splitBySemantic(content string) []string {
	// 首先按段落分割
	paragraphs := strings.Split(content, "\n\n")
	
	var chunks []string
	var currentChunk strings.Builder
	currentSize := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraSize := len(para)
		
		// 如果段落本身就超过块大小，使用长度分割
		if paraSize > p.chunkSize {
			// 保存当前块
			if currentSize > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				currentSize = 0
			}
			
			// 分割大段落
			subChunks := p.splitByLength(para)
			chunks = append(chunks, subChunks...)
			continue
		}

		// 如果添加这个段落会超过块大小，先保存当前块
		if currentSize+paraSize+2 > p.chunkSize && currentSize > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentSize = 0
		}

		// 添加段落到当前块
		if currentSize > 0 {
			currentChunk.WriteString("\n\n")
			currentSize += 2
		}
		currentChunk.WriteString(para)
		currentSize += paraSize
	}

	// 保存最后一个块
	if currentSize > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// EstimateTokens 估算文本的token数量（简单估算）
func (p *DocumentProcessor) EstimateTokens(text string) int {
	// 简单估算：平均每4个字符一个token
	return len(text) / 4
}