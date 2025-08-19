package components

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusRetriever 实现 Eino 的 Retriever 接口
type MilvusRetriever struct {
	client         client.Client
	collectionName string
	embedding      *OllamaEmbedding
	topK           int
}

func NewMilvusRetriever(host string, port int, collection string, embedding *OllamaEmbedding, topK int) (*MilvusRetriever, error) {
	ctx := context.Background()

	c, err := client.NewClient(ctx, client.Config{
		Address: fmt.Sprintf("%s:%d", host, port),
	})
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}

	retriever := &MilvusRetriever{
		client:         c,
		collectionName: collection,
		embedding:      embedding,
		topK:           topK,
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
		return err
	}

	if !has {
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
					TypeParams: map[string]string{"max_length": "4096"},
				},
				{
					Name:       "embedding",
					DataType:   entity.FieldTypeFloatVector,
					TypeParams: map[string]string{"dim": fmt.Sprintf("%d", m.embedding.Dimension())},
				},
			},
		}

		err = m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return err
		}

		// 创建索引
		idx, err := entity.NewIndexIvfFlat(entity.L2, 128)
		if err != nil {
			return err
		}

		err = m.client.CreateIndex(ctx, m.collectionName, "embedding", idx, false)
		if err != nil {
			return err
		}
	}

	// 加载集合
	return m.client.LoadCollection(ctx, m.collectionName, false)
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

			doc := &schema.Document{
				ID:       id,
				Content:  content,
				MetaData: metadata,
				// Score字段在当前版本的schema.Document中不存在
			}

			results = append(results, doc)
		}
	}

	return results, nil
}

func (m *MilvusRetriever) AddDocuments(ctx context.Context, docs []*schema.Document) error {
	// 准备数据
	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	metadatas := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))

	// 生成嵌入
	docsWithEmbedding, err := m.embedding.EmbedDocuments(ctx, docs)
	if err != nil {
		return fmt.Errorf("embed documents: %w", err)
	}

	for i, doc := range docsWithEmbedding {
		ids[i] = doc.ID
		contents[i] = doc.Content

		metaBytes, _ := json.Marshal(doc.MetaData)
		metadatas[i] = string(metaBytes)

		// 从MetaData中获取嵌入向量
		if embeddingData, exists := doc.MetaData["embedding"]; exists {
			if embVec, ok := embeddingData.([]float32); ok {
				embeddings[i] = embVec
			}
		}
	}

	// 插入到Milvus
	columns := []entity.Column{
		entity.NewColumnVarChar("doc_id", ids),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnVarChar("metadata", metadatas),
		entity.NewColumnFloatVector("embedding", m.embedding.Dimension(), embeddings),
	}

	_, err = m.client.Insert(ctx, m.collectionName, "", columns...)
	return err
}

func (m *MilvusRetriever) Close() error {
	return m.client.Close()
}
