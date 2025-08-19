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
)

type DocumentProcessor struct {
	splitter            document.Transformer
	embedding           *OllamaEmbedding
	minChunkSize        int
	maxChunkSize        int
	enableSemanticSplit bool
	embeddingCache      *EmbeddingCache
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

func NewDocumentProcessor(embedding *OllamaEmbedding, minChunkSize, maxChunkSize int, enableSemanticSplit bool) (*DocumentProcessor, error) {
	processor := &DocumentProcessor{
		embedding:           embedding,
		minChunkSize:        minChunkSize,
		maxChunkSize:        maxChunkSize,
		enableSemanticSplit: enableSemanticSplit,
		embeddingCache:      NewEmbeddingCache(),
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

func (p *DocumentProcessor) ProcessText(text string, metadata map[string]interface{}) ([]*schema.Document, error) {
	// 性能优化：根据文本大小和配置选择处理策略
	textLength := len([]rune(text))

	// 小文档或禁用语义分割时使用传统分块（避免嵌入开销）
	// 小于 CHUNK_SIZE 的文档不需要语义分割
	if !p.enableSemanticSplit || textLength < p.minChunkSize {
		return p.processTextLegacy(text, metadata), nil
	}

	// 记录处理开始时间（用于性能分析）
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime)
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["processing_time_ms"] = processingTime.Milliseconds()
	}()

	ctx := context.Background()

	// 生成文档ID
	docID := fmt.Sprintf("%x", md5.Sum([]byte(text)))

	// 创建原始文档对象
	originalDoc := &schema.Document{
		ID:       docID,
		Content:  text,
		MetaData: metadata,
	}

	// 使用语义分割器进行智能分割
	chunks, err := p.splitter.Transform(ctx, []*schema.Document{originalDoc})
	if err != nil {
		// 语义分割失败时回退到传统分块
		return p.processTextLegacy(text, metadata), nil
	}

	// 为每个分块添加元数据和ID
	var processedChunks []*schema.Document
	for i, chunk := range chunks {
		// 生成新的chunk ID
		chunkID := fmt.Sprintf("%s_%d", docID, i)

		// 复制原始元数据
		chunkMeta := make(map[string]interface{})
		for k, v := range metadata {
			chunkMeta[k] = v
		}

		// 添加分块特定的元数据
		chunkMeta["parent_id"] = docID
		chunkMeta["chunk_index"] = i
		chunkMeta["chunk_total"] = len(chunks)
		chunkMeta["content_length"] = len([]rune(chunk.Content))
		chunkMeta["splitting_method"] = "semantic"

		// 如果分块太长，进行额外的递归分割
		if len([]rune(chunk.Content)) > p.maxChunkSize {
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
			// 直接使用语义分割的结果
			processedChunk := &schema.Document{
				ID:       chunkID,
				Content:  chunk.Content,
				MetaData: chunkMeta,
			}
			processedChunks = append(processedChunks, processedChunk)
		}
	}

	return processedChunks, nil
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
	return p.ProcessTextLegacy(text, metadata, p.minChunkSize/5, 10) // 转换字符数为大概的单词数
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
		"semantic_splitting_enabled": p.enableSemanticSplit,
		"min_chunk_size":             p.minChunkSize,
		"max_chunk_size":             p.maxChunkSize,
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
			MinChunkSize: p.minChunkSize,
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
