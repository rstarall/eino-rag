package components

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go.uber.org/zap"
)

// MilvusRetriever 实现 Eino 的 Retriever 接口
type MilvusRetriever struct {
	client         client.Client
	collectionName string
	embedding      *OllamaEmbedding
	topK           int
	logger         *zap.Logger
}

func NewMilvusRetriever(host string, port int, collection string, embedding *OllamaEmbedding, topK int, logger *zap.Logger) (*MilvusRetriever, error) {
	ctx := context.Background()

	logger.Info("初始化Milvus连接", 
		zap.String("host", host),
		zap.Int("port", port),
		zap.String("collection", collection),
		zap.Int("topK", topK),
		zap.Int("vector_dimension", embedding.Dimension()))

	c, err := client.NewClient(ctx, client.Config{
		Address: fmt.Sprintf("%s:%d", host, port),
	})
	if err != nil {
		logger.Error("连接Milvus失败", zap.Error(err))
		return nil, fmt.Errorf("connect milvus: %w", err)
	}

	retriever := &MilvusRetriever{
		client:         c,
		collectionName: collection,
		embedding:      embedding,
		topK:           topK,
		logger:         logger,
	}

	// 初始化集合
	if err := retriever.initCollection(ctx); err != nil {
		return nil, fmt.Errorf("init collection: %w", err)
	}

	return retriever, nil
}

func (m *MilvusRetriever) initCollection(ctx context.Context) error {
	has, err := m.client.HasCollection(ctx, m.collectionName)
	if err != nil {
		m.logger.Error("检查集合是否存在失败", zap.Error(err))
		return err
	}

	expectedDim := m.embedding.Dimension()
	
	if has {
		m.logger.Info("集合已存在，检查维度兼容性", 
			zap.String("collection", m.collectionName),
			zap.Int("expected_dim", expectedDim))
		
		// 检查现有集合的维度
		err = m.validateCollectionDimension(ctx, expectedDim)
		if err != nil {
			return err
		}
	} else {
		m.logger.Info("集合不存在，创建新集合", 
			zap.String("collection", m.collectionName),
			zap.Int("vector_dim", expectedDim))
		
		err = m.createCollection(ctx, expectedDim)
		if err != nil {
			return err
		}
	}

	// 加载集合
	m.logger.Debug("加载集合到内存")
	err = m.client.LoadCollection(ctx, m.collectionName, false)
	if err != nil {
		m.logger.Error("加载集合失败", zap.Error(err))
		return err
	}
	
	m.logger.Info("集合初始化完成", zap.String("collection", m.collectionName))
	return nil
}

// validateCollectionDimension 验证现有集合的向量维度
func (m *MilvusRetriever) validateCollectionDimension(ctx context.Context, expectedDim int) error {
	// 获取集合信息
	desc, err := m.client.DescribeCollection(ctx, m.collectionName)
	if err != nil {
		m.logger.Error("获取集合描述失败", zap.Error(err))
		return fmt.Errorf("failed to describe collection: %w", err)
	}
	
	// 查找embedding字段的维度
	for _, field := range desc.Schema.Fields {
		if field.Name == "embedding" && field.DataType == entity.FieldTypeFloatVector {
			if dimStr, exists := field.TypeParams["dim"]; exists {
				if actualDim := parseInt(dimStr); actualDim > 0 {
					if actualDim != expectedDim {
						m.logger.Error("集合维度不匹配", 
							zap.String("collection", m.collectionName),
							zap.Int("expected_dim", expectedDim),
							zap.Int("actual_dim", actualDim))
						return fmt.Errorf("collection %s has vector dimension %d, but expected %d. Please use a different collection name or update VECTOR_DIM in .env", 
							m.collectionName, actualDim, expectedDim)
					}
					m.logger.Info("集合维度验证通过", 
						zap.Int("dimension", actualDim))
					return nil
				}
			}
		}
	}
	
	return fmt.Errorf("could not find embedding field dimension in collection %s", m.collectionName)
}

// createCollection 创建新集合
func (m *MilvusRetriever) createCollection(ctx context.Context, vectorDim int) error {
	schema := &entity.Schema{
		CollectionName: m.collectionName,
		Fields: []*entity.Field{
			{
				Name:       "doc_id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				TypeParams: map[string]string{"max_length": "128"},
			},
			{
				Name:       "content",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "65535"},
			},
										{
					Name:       "metadata",
					DataType:   entity.FieldTypeVarChar,
					TypeParams: map[string]string{"max_length": "65535"},
				},
			{
				Name:       "embedding",
				DataType:   entity.FieldTypeFloatVector,
				TypeParams: map[string]string{"dim": fmt.Sprintf("%d", vectorDim)},
			},
		},
	}

	m.logger.Info("创建集合", 
		zap.String("collection", m.collectionName),
		zap.Int("vector_dimension", vectorDim))

	err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		m.logger.Error("创建集合失败", zap.Error(err))
		return err
	}

	// 创建索引
	m.logger.Debug("创建向量索引")
	idx, err := entity.NewIndexIvfFlat(entity.L2, 128)
	if err != nil {
		m.logger.Error("创建索引配置失败", zap.Error(err))
		return err
	}

	err = m.client.CreateIndex(ctx, m.collectionName, "embedding", idx, false)
	if err != nil {
		m.logger.Error("创建索引失败", zap.Error(err))
		return err
	}

	m.logger.Info("集合和索引创建完成")
	return nil
}

// parseInt 解析字符串为整数
func parseInt(s string) int {
	if i, err := fmt.Sscanf(s, "%d", new(int)); err == nil && i == 1 {
		var result int
		fmt.Sscanf(s, "%d", &result)
		return result
	}
	return 0
}

// optimizeMetadata 优化元数据，控制在65536字节限制内
func (m *MilvusRetriever) optimizeMetadata(metadata map[string]interface{}) map[string]interface{} {
	optimized := make(map[string]interface{})
	
	// 保留重要字段，适当限制长度
	importantFields := map[string]int{
		"filename":       500,  // 文件名最多500字符
		"file_type":      50,   // 文件类型最多50字符
		"upload_time":    50,   // 上传时间
		"chunk_index":    -1,   // 数字字段不限制
		"chunk_total":    -1,   // 数字字段不限制
		"parent_id":      200,  // 父文档ID
		"splitting_method": 100, // 分割方法
		"content_length": -1,   // 内容长度
		"original_size":  -1,   // 原始大小
		"parsed_size":    -1,   // 解析后大小
	}
	
	for field, maxLen := range importantFields {
		if value, exists := metadata[field]; exists {
			if maxLen > 0 {
				// 字符串字段，适当截断
				if str, ok := value.(string); ok {
					if len(str) > maxLen {
						optimized[field] = str[:maxLen] + "..."
					} else {
						optimized[field] = str
					}
				} else {
					optimized[field] = value
				}
			} else {
				// 数字或其他类型字段，直接保留
				optimized[field] = value
			}
		}
	}
	
	// 移除巨大的字段
	delete(optimized, "embedding")
	delete(optimized, "raw_content") 
	delete(optimized, "full_text")
	
	// 选择性保留其他字段
	fieldCount := 0
	for key, value := range metadata {
		if _, exists := optimized[key]; !exists && fieldCount < 20 {
			// 跳过已知的大字段
			if key == "embedding" || key == "raw_content" || key == "full_text" {
				continue
			}
			
			// 截断长字符串
			if str, ok := value.(string); ok && len(str) > 1000 {
				optimized[key] = str[:1000] + "..."
			} else {
				optimized[key] = value
			}
			fieldCount++
		}
	}
	
	return optimized
}

func (m *MilvusRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	// 生成查询嵌入
	embeddings, err := m.embedding.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Convert []float64 to []float32
	embedding32 := make([]float32, len(embeddings[0]))
	for i, v := range embeddings[0] {
		embedding32[i] = float32(v)
	}

	vectors := []entity.Vector{
		entity.FloatVector(embedding32),
	}

	// 创建搜索参数
	searchParam, err := entity.NewIndexIvfFlatSearchParam(10)
	if err != nil {
		return nil, fmt.Errorf("create search param: %w", err)
	}

	// 搜索
	searchResult, err := m.client.Search(
		ctx,
		m.collectionName,
		nil,
		"",
		[]string{"doc_id", "content", "metadata"},
		vectors,
		"embedding",
		entity.L2,
		m.topK,
		searchParam,
	)

	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	var results []*schema.Document

	for _, sr := range searchResult {
		for i := 0; i < sr.ResultCount; i++ {
			id, _ := sr.Fields.GetColumn("doc_id").GetAsString(i)
			content, _ := sr.Fields.GetColumn("content").GetAsString(i)
			metadataStr, _ := sr.Fields.GetColumn("metadata").GetAsString(i)

			var metadata map[string]interface{}
			json.Unmarshal([]byte(metadataStr), &metadata)

			// 获取距离分数并转换为相似度
			distance := sr.Scores[i]
			// 对于L2距离，转换为相似度分数 (距离越小，相似度越高)
			// 使用 1/(1+distance) 公式将距离转换为 0-1 之间的相似度
			similarity := 1.0 / (1.0 + float64(distance))
			
			// 将相似度分数存储在metadata中
			if metadata == nil {
				metadata = make(map[string]interface{})
			}
			metadata["similarity_score"] = similarity

			doc := &schema.Document{
				ID:       id,
				Content:  content,
				MetaData: metadata,
			}

			results = append(results, doc)
		}
	}

	return results, nil
}

func (m *MilvusRetriever) AddDocuments(ctx context.Context, docs []*schema.Document) error {
	m.logger.Info("开始添加文档到Milvus", 
		zap.Int("document_count", len(docs)),
		zap.String("collection_name", m.collectionName),
		zap.Int("expected_dimension", m.embedding.Dimension()))

	// 准备数据
	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	metadatas := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))

	// 记录文档信息
	for i, doc := range docs {
		m.logger.Debug("准备添加文档", 
			zap.Int("doc_index", i),
			zap.String("doc_id", doc.ID),
			zap.Int("content_length", len(doc.Content)))
	}

	// 生成嵌入
	m.logger.Debug("开始生成文档嵌入向量")
	embedStart := time.Now()
	docsWithEmbedding, err := m.embedding.EmbedDocuments(ctx, docs)
	embedDuration := time.Since(embedStart)
	
	if err != nil {
		m.logger.Error("生成文档嵌入向量失败", 
			zap.Error(err),
			zap.Duration("embed_duration", embedDuration))
		return fmt.Errorf("embed documents: %w", err)
	}

	m.logger.Info("文档嵌入向量生成完成", 
		zap.Int("embedded_docs", len(docsWithEmbedding)),
		zap.Duration("embed_duration", embedDuration))

	// 准备Milvus插入数据
	for i, doc := range docsWithEmbedding {
		ids[i] = doc.ID
		contents[i] = doc.Content

		// 优化元数据：只保留重要信息并严格限制长度
		optimizedMetadata := m.optimizeMetadata(doc.MetaData)
		metaBytes, err := json.Marshal(optimizedMetadata)
		if err != nil {
			m.logger.Warn("序列化元数据失败", 
				zap.String("doc_id", doc.ID),
				zap.Error(err))
			metadatas[i] = "{}"
		} else {
			metadataStr := string(metaBytes)
			// 严格确保元数据长度不超过Milvus的65535字节限制
			maxLength := 65000 // 保守的限制，为JSON格式和UTF-8字符编码留出缓冲空间
			if len(metadataStr) > maxLength {
				// 如果仍然超长，进一步压缩
				compactMeta := map[string]interface{}{
					"filename":     optimizedMetadata["filename"],
					"file_type":    optimizedMetadata["file_type"],
					"chunk_index":  optimizedMetadata["chunk_index"],
					"chunk_total":  optimizedMetadata["chunk_total"],
					"content_length": optimizedMetadata["content_length"],
				}
				
				// 截断文件名如果太长
				if filename, ok := compactMeta["filename"].(string); ok && len(filename) > 200 {
					compactMeta["filename"] = filename[:200] + "..."
				}
				
				compactBytes, _ := json.Marshal(compactMeta)
				metadataStr = string(compactBytes)
				
				// 最后的保险措施：直接截断
				if len(metadataStr) > maxLength {
					// 按字节截断，确保JSON格式正确
					if maxLength > 100 {
						metadataStr = metadataStr[:maxLength-10] + "...\"}"
					} else {
						metadataStr = `{"filename":"truncated","chunk_index":0}`
					}
				}
				
				m.logger.Warn("元数据过长，已压缩", 
					zap.String("doc_id", doc.ID),
					zap.Int("original_bytes", len(string(metaBytes))),
					zap.Int("final_bytes", len(metadataStr)))
			}
			metadatas[i] = metadataStr
		}

		// 从MetaData中获取嵌入向量
		if embeddingData, exists := doc.MetaData["embedding"]; exists {
			if embVec, ok := embeddingData.([]float32); ok {
				embeddings[i] = embVec
				m.logger.Debug("获取嵌入向量", 
					zap.String("doc_id", doc.ID),
					zap.Int("embedding_dimension", len(embVec)),
					zap.Float32("first_value", embVec[0]))
			} else {
				m.logger.Error("嵌入向量类型转换失败", 
					zap.String("doc_id", doc.ID),
					zap.String("actual_type", fmt.Sprintf("%T", embeddingData)))
				return fmt.Errorf("invalid embedding type for document %s", doc.ID)
			}
		} else {
			m.logger.Error("文档缺少嵌入向量", zap.String("doc_id", doc.ID))
			return fmt.Errorf("missing embedding for document %s", doc.ID)
		}
	}

	// 验证数据完整性
	m.logger.Debug("验证插入数据", 
		zap.Int("ids_count", len(ids)),
		zap.Int("contents_count", len(contents)),
		zap.Int("metadatas_count", len(metadatas)),
		zap.Int("embeddings_count", len(embeddings)))

	// 插入到Milvus
	m.logger.Debug("开始插入数据到Milvus")
	insertStart := time.Now()
	
	columns := []entity.Column{
		entity.NewColumnVarChar("doc_id", ids),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnVarChar("metadata", metadatas),
		entity.NewColumnFloatVector("embedding", m.embedding.Dimension(), embeddings),
	}

	_, err = m.client.Insert(ctx, m.collectionName, "", columns...)
	insertDuration := time.Since(insertStart)
	
	if err != nil {
		m.logger.Error("插入Milvus失败", 
			zap.Error(err),
			zap.Duration("insert_duration", insertDuration))
		return err
	}

	m.logger.Info("文档成功插入Milvus", 
		zap.Int("inserted_count", len(docs)),
		zap.Duration("insert_duration", insertDuration))

	return nil
}



func (m *MilvusRetriever) Close() error {
	return m.client.Close()
}
