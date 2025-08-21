package components

import (
	"context"
	"crypto/md5"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"eino-rag/config"
)

type DocumentProcessor struct {
	splitter            document.Transformer
	embedding           *OllamaEmbedding
	chunkSize           int                     // 分块大小（字符数）
	chunkOverlap        int                     // 分块重叠大小（字符数）
	chunkingStrategy    config.ChunkingStrategy // 分块策略
	maxChunkSize        int                     // 最大分块大小（用于语义分块后的递归分割）
	enableSemanticSplit bool                    // 向后兼容字段
	embeddingCache      *EmbeddingCache
	logger              *zap.Logger
}

// EmbeddingCache 嵌入向量缓存
type EmbeddingCache struct {
	cache map[string][]float64
	mutex sync.RWMutex
}

func NewEmbeddingCache() *EmbeddingCache {
	return &EmbeddingCache{
		cache: make(map[string][]float64),
	}
}

// NewDocumentProcessorWithStrategy 创建支持新分块策略的文档处理器
func NewDocumentProcessorWithStrategy(embedding *OllamaEmbedding, chunkSize, chunkOverlap int, strategy config.ChunkingStrategy, logger *zap.Logger) (*DocumentProcessor, error) {
	processor := &DocumentProcessor{
		embedding:           embedding,
		chunkSize:           chunkSize,
		chunkOverlap:        chunkOverlap,
		chunkingStrategy:    strategy,
		maxChunkSize:        chunkSize * 3, // 最大分块大小为普通分块大小的3倍
		enableSemanticSplit: strategy == config.ChunkingStrategySemantic,
		embeddingCache:      NewEmbeddingCache(),
		logger:              logger,
	}

	// 如果使用语义分割，初始化语义分割器
	if strategy == config.ChunkingStrategySemantic {
		ctx := context.Background()
		splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
			Embedding:    embedding,
			BufferSize:   2,
			MinChunkSize: chunkSize,
			Separators:   []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?", "；", ";", "，", ","},
			Percentile:   0.85,
			LenFunc: func(s string) int {
				return len([]rune(s))
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create semantic splitter: %w", err)
		}
		processor.splitter = splitter
	}

	return processor, nil
}

// NewDocumentProcessor 保留向后兼容的构造函数
func NewDocumentProcessor(embedding *OllamaEmbedding, minChunkSize, maxChunkSize int, enableSemanticSplit bool, logger *zap.Logger) (*DocumentProcessor, error) {
	var strategy config.ChunkingStrategy
	if enableSemanticSplit {
		strategy = config.ChunkingStrategySemantic
	} else {
		strategy = config.ChunkingStrategyWordBased // 向后兼容使用原来的word-based方式
	}

	processor := &DocumentProcessor{
		embedding:           embedding,
		chunkSize:           minChunkSize,
		chunkOverlap:        minChunkSize / 10, // 默认重叠为分块大小的10%
		chunkingStrategy:    strategy,
		maxChunkSize:        maxChunkSize,
		enableSemanticSplit: enableSemanticSplit,
		embeddingCache:      NewEmbeddingCache(),
		logger:              logger,
	}

	// 根据官方文档初始化语义分割器
	if enableSemanticSplit {
		ctx := context.Background()

		// 根据官方文档配置语义分割器
		splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
			Embedding:    embedding,                                                                // 必需：用于生成文本向量的嵌入器
			BufferSize:   2,                                                                        // 可选：上下文缓冲区大小
			MinChunkSize: minChunkSize,                                                             // 可选：最小片段大小
			Separators:   []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?", "；", ";", "，", ","}, // 可选：分隔符列表
			Percentile:   0.85,                                                                     // 可选：分割阈值百分位数
			LenFunc: func(s string) int {
				// 使用 unicode 字符数而不是字节数，更适合中文
				return len([]rune(s))
			},
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create semantic splitter: %w", err)
		}

		processor.splitter = splitter
	}

	return processor, nil
}

// processTextByLength 基于字符长度的分块处理（支持滑动窗口）
func (p *DocumentProcessor) processTextByLength(text string, metadata map[string]interface{}) []*schema.Document {
	// 生成文档ID
	docID := fmt.Sprintf("%x", md5.Sum([]byte(text)))

	// 将文本转为rune数组以正确处理中文字符
	runes := []rune(text)
	textLength := len(runes)

	var chunks []*schema.Document

	// 如果文本长度小于等于分块大小，直接返回整个文本
	if textLength <= p.chunkSize {
		chunkMeta := make(map[string]interface{})
		for k, v := range metadata {
			chunkMeta[k] = v
		}
		chunkMeta["parent_id"] = docID
		chunkMeta["chunk_index"] = 0
		chunkMeta["chunk_total"] = 1
		chunkMeta["content_length"] = textLength
		chunkMeta["splitting_method"] = "length_based"

		chunk := &schema.Document{
			ID:       fmt.Sprintf("%s_0", docID),
			Content:  text,
			MetaData: chunkMeta,
		}
		return []*schema.Document{chunk}
	}

	// 使用滑动窗口进行分块
	chunkIndex := 0
	for start := 0; start < textLength; start += (p.chunkSize - p.chunkOverlap) {
		end := start + p.chunkSize
		if end > textLength {
			end = textLength
		}

		// 提取分块内容
		chunkContent := string(runes[start:end])

		// 如果这是最后一个分块且长度太短，合并到前一个分块
		if end == textLength && len([]rune(chunkContent)) < p.chunkOverlap && len(chunks) > 0 {
			lastChunk := chunks[len(chunks)-1]
			lastChunk.Content += chunkContent
			lastChunk.MetaData["content_length"] = len([]rune(lastChunk.Content))
			break
		}

		chunkMeta := make(map[string]interface{})
		for k, v := range metadata {
			chunkMeta[k] = v
		}
		chunkMeta["parent_id"] = docID
		chunkMeta["chunk_index"] = chunkIndex
		chunkMeta["content_length"] = len([]rune(chunkContent))
		chunkMeta["splitting_method"] = "length_based"
		chunkMeta["chunk_start"] = start
		chunkMeta["chunk_end"] = end

		chunk := &schema.Document{
			ID:       fmt.Sprintf("%s_%d", docID, chunkIndex),
			Content:  chunkContent,
			MetaData: chunkMeta,
		}
		chunks = append(chunks, chunk)
		chunkIndex++

		// 如果已经到达文本末尾，退出循环
		if end >= textLength {
			break
		}
	}

	// 更新总分块数量
	for _, chunk := range chunks {
		chunk.MetaData["chunk_total"] = len(chunks)
	}

	p.logger.Info("长度分块完成",
		zap.Int("text_length", textLength),
		zap.Int("chunk_size", p.chunkSize),
		zap.Int("chunk_overlap", p.chunkOverlap),
		zap.Int("chunk_count", len(chunks)))

	return chunks
}

func (p *DocumentProcessor) ProcessText(text string, metadata map[string]interface{}) ([]*schema.Document, error) {
	p.logger.Info("开始处理文档文本",
		zap.Int("text_length", len([]rune(text))),
		zap.String("chunking_strategy", string(p.chunkingStrategy)),
		zap.Int("chunk_size", p.chunkSize),
		zap.Int("chunk_overlap", p.chunkOverlap))

	// 记录处理开始时间
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime)
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["processing_time_ms"] = processingTime.Milliseconds()
		p.logger.Debug("文档处理完成", zap.Duration("processing_time", processingTime))
	}()

	// 根据分块策略选择处理方法
	switch p.chunkingStrategy {
	case config.ChunkingStrategyLength:
		p.logger.Info("使用长度分块方法")
		return p.processTextByLength(text, metadata), nil

	case config.ChunkingStrategySemantic:
		p.logger.Info("使用语义分割方法")
		return p.processTextSemantic(text, metadata)

	case config.ChunkingStrategyWordBased:
		p.logger.Info("使用基于单词的分块方法（向后兼容）")
		return p.processTextLegacy(text, metadata), nil

	default:
		p.logger.Warn("未知的分块策略，使用长度分块", zap.String("strategy", string(p.chunkingStrategy)))
		return p.processTextByLength(text, metadata), nil
	}
}

// processTextSemantic 语义分割处理方法
func (p *DocumentProcessor) processTextSemantic(text string, metadata map[string]interface{}) ([]*schema.Document, error) {
	textLength := len([]rune(text))

	// 小文档直接使用长度分块（避免语义分割的开销）
	if textLength < p.chunkSize {
		p.logger.Info("文本过短，使用长度分块替代语义分割", zap.Int("text_length", textLength))
		return p.processTextByLength(text, metadata), nil
	}

	ctx := context.Background()
	docID := fmt.Sprintf("%x", md5.Sum([]byte(text)))

	// 创建原始文档对象
	originalDoc := &schema.Document{
		ID:       docID,
		Content:  text,
		MetaData: metadata,
	}

	// 使用语义分割器进行分割
	p.logger.Debug("开始语义分割")
	chunks, err := p.splitter.Transform(ctx, []*schema.Document{originalDoc})
	if err != nil {
		p.logger.Warn("语义分割失败，回退到长度分块", zap.Error(err))
		return p.processTextByLength(text, metadata), nil
	}

	p.logger.Info("语义分割完成", zap.Int("initial_chunk_count", len(chunks)))

	// 处理语义分割的结果
	var processedChunks []*schema.Document
	for i, chunk := range chunks {
		chunkLength := len([]rune(chunk.Content))

		// 复制元数据
		chunkMeta := make(map[string]interface{})
		for k, v := range metadata {
			chunkMeta[k] = v
		}
		chunkMeta["parent_id"] = docID
		chunkMeta["chunk_index"] = i
		chunkMeta["chunk_total"] = len(chunks)
		chunkMeta["content_length"] = chunkLength
		chunkMeta["splitting_method"] = "semantic"

		// 如果分块太长，进行递归分割
		if chunkLength > p.maxChunkSize {
			subChunks := p.recursiveSplit(chunk.Content, p.maxChunkSize)
			for j, subChunk := range subChunks {
				subChunkID := fmt.Sprintf("%s_%d_%d", docID, i, j)
				subChunkMeta := make(map[string]interface{})
				for k, v := range chunkMeta {
					subChunkMeta[k] = v
				}
				subChunkMeta["sub_chunk_index"] = j
				subChunkMeta["is_sub_chunk"] = true

				processedChunk := &schema.Document{
					ID:       subChunkID,
					Content:  subChunk,
					MetaData: subChunkMeta,
				}
				processedChunks = append(processedChunks, processedChunk)
			}
		} else {
			chunkID := fmt.Sprintf("%s_%d", docID, i)
			processedChunk := &schema.Document{
				ID:       chunkID,
				Content:  chunk.Content,
				MetaData: chunkMeta,
			}
			processedChunks = append(processedChunks, processedChunk)
		}
	}

	p.logger.Info("语义分割处理完成", zap.Int("final_chunk_count", len(processedChunks)))
	return processedChunks, nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// recursiveSplit 递归分割过长的文本块
func (p *DocumentProcessor) recursiveSplit(text string, maxSize int) []string {
	runes := []rune(text)
	if len(runes) <= maxSize {
		return []string{text}
	}

	var chunks []string

	// 按句号、换行符等分隔符进行分割
	separators := []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?"}

	for _, sep := range separators {
		parts := strings.Split(text, sep)
		if len(parts) > 1 {
			var currentChunk string
			for _, part := range parts {
				if len([]rune(currentChunk+part+sep)) > maxSize && currentChunk != "" {
					chunks = append(chunks, strings.TrimSpace(currentChunk))
					currentChunk = part + sep
				} else {
					if currentChunk != "" {
						currentChunk += sep
					}
					currentChunk += part
				}
			}
			if currentChunk != "" {
				chunks = append(chunks, strings.TrimSpace(currentChunk))
			}
			return chunks
		}
	}

	// 如果没有合适的分隔符，按字符数强制分割
	for i := 0; i < len(runes); i += maxSize {
		end := i + maxSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

// processTextLegacy 内部使用的传统分块方法
func (p *DocumentProcessor) processTextLegacy(text string, metadata map[string]interface{}) []*schema.Document {
	return p.ProcessTextLegacy(text, metadata, p.chunkSize/5, 10) // 转换字符数为大概的单词数
}

// ProcessTextLegacy 提供向后兼容的简单分块方法
func (p *DocumentProcessor) ProcessTextLegacy(text string, metadata map[string]interface{}, chunkSize, chunkOverlap int) []*schema.Document {
	// 生成文档ID
	docID := fmt.Sprintf("%x", md5.Sum([]byte(text)))

	// 简单的基于字数的分块（向后兼容）
	words := strings.Fields(text)
	var chunks []*schema.Document

	for i := 0; i < len(words); i += chunkSize - chunkOverlap {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunkContent := strings.Join(words[i:end], " ")
		chunkID := fmt.Sprintf("%s_%d", docID, len(chunks))

		chunkMeta := make(map[string]interface{})
		for k, v := range metadata {
			chunkMeta[k] = v
		}
		chunkMeta["parent_id"] = docID
		chunkMeta["chunk_index"] = len(chunks)
		chunkMeta["splitting_method"] = "word_based"

		chunk := &schema.Document{
			ID:       chunkID,
			Content:  chunkContent,
			MetaData: chunkMeta,
		}

		chunks = append(chunks, chunk)

		if end >= len(words) {
			break
		}
	}

	return chunks
}

// GetProcessingStats 获取处理统计信息
func (p *DocumentProcessor) GetProcessingStats() map[string]interface{} {
	stats := map[string]interface{}{
		"chunking_strategy":          string(p.chunkingStrategy),
		"chunk_size":                 p.chunkSize,
		"chunk_overlap":              p.chunkOverlap,
		"max_chunk_size":             p.maxChunkSize,
		"semantic_splitting_enabled": p.enableSemanticSplit,
		"has_semantic_splitter":      p.splitter != nil,
	}

	if p.embeddingCache != nil {
		p.embeddingCache.mutex.RLock()
		stats["cache_entries"] = len(p.embeddingCache.cache)
		p.embeddingCache.mutex.RUnlock()
	}

	return stats
}

// ClearCache 清空嵌入缓存
func (p *DocumentProcessor) ClearCache() {
	if p.embeddingCache != nil {
		p.embeddingCache.mutex.Lock()
		p.embeddingCache.cache = make(map[string][]float64)
		p.embeddingCache.mutex.Unlock()
	}
}

// SetSemanticSplitting 动态启用/禁用语义分割
func (p *DocumentProcessor) SetSemanticSplitting(enable bool) error {
	if enable && p.splitter == nil {
		// 如果要启用但分割器不存在，需要创建
		ctx := context.Background()
		splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
			Embedding:    p.embedding,
			BufferSize:   2,
			MinChunkSize: p.chunkSize,
			Separators:   []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?", "；", ";", "，", ","},
			Percentile:   0.85,
			LenFunc: func(s string) int {
				return len([]rune(s))
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create semantic splitter: %w", err)
		}
		p.splitter = splitter
	}

	p.enableSemanticSplit = enable
	return nil
}
