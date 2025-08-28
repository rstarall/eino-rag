package rag

import (
	"context"
	"fmt"
	"sync"
	"time"

	"eino-rag/internal/config"

	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type MilvusRetriever struct {
	client         client.Client
	collectionName string
	embedding      *EmbeddingService
	topK           int
	logger         *zap.Logger
	insertTimeout  time.Duration
	config         *config.Config
	isConnected    bool
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewMilvusRetriever(cfg *config.Config, embedding *EmbeddingService, logger *zap.Logger) (*MilvusRetriever, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	retriever := &MilvusRetriever{
		collectionName: cfg.CollectionName,
		embedding:      embedding,
		topK:           cfg.TopK,
		logger:         logger,
		insertTimeout:  cfg.MilvusInsertTimeout,
		config:         cfg,
		ctx:            ctx,
		cancel:         cancel,
	}

	// 尝试初始连接
	if err := retriever.connect(); err != nil {
		logger.Warn("Initial connection to Milvus failed, will retry in background", 
			zap.Error(err),
			zap.String("address", cfg.MilvusAddress))
	}

	// 启动重连协程
	go retriever.reconnectLoop()

	return retriever, nil
}

// ensureCollectionWithClient 确保集合存在
func (r *MilvusRetriever) ensureCollectionWithClient(ctx context.Context, c client.Client) error {
	// 使用带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	// 检查集合是否存在
	r.logger.Info("Checking if collection exists", zap.String("collection", r.collectionName))
	exists, err := c.HasCollection(checkCtx, r.collectionName)
	if err != nil {
		r.logger.Error("Failed to check collection existence",
			zap.String("collection", r.collectionName),
			zap.Error(err))
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		// 创建集合
		schema := &entity.Schema{
			CollectionName: r.collectionName,
			Description:    "RAG document embeddings",
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: true,
					AutoID:     false,
					TypeParams: map[string]string{
						"max_length": "512",
					},
				},
				{
					Name:      "content",
					DataType:  entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "65535",
					},
				},
				{
					Name:     "embedding",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						"dim": fmt.Sprintf("%d", r.config.VectorDimension),
					},
				},
				{
					Name:     "kb_id",
					DataType: entity.FieldTypeInt64,
				},
				{
					Name:     "doc_id",
					DataType: entity.FieldTypeInt64,
				},
			},
		}

		if err := c.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}

		r.logger.Info("Created Milvus collection", zap.String("collection", r.collectionName))

		// 创建索引
		idx, err := entity.NewIndexIvfFlat(entity.L2, 1024)
		if err != nil {
			return fmt.Errorf("failed to create index definition: %w", err)
		}

		if err := c.CreateIndex(ctx, r.collectionName, "embedding", idx, false); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}

		// 加载集合
		if err := c.LoadCollection(ctx, r.collectionName, false); err != nil {
			return fmt.Errorf("failed to load collection: %w", err)
		}
	}

	return nil
}

// ensureCollection 确保集合存在
func (r *MilvusRetriever) ensureCollection(ctx context.Context, cfg *config.Config) error {
	// 使用带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	
	if client == nil {
		return fmt.Errorf("milvus client is not initialized")
	}
	
	// 检查集合是否存在
	r.logger.Info("Checking if collection exists", zap.String("collection", r.collectionName))
	exists, err := client.HasCollection(checkCtx, r.collectionName)
	if err != nil {
		r.logger.Error("Failed to check collection existence",
			zap.String("collection", r.collectionName),
			zap.Error(err))
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		// 创建集合
		schema := &entity.Schema{
			CollectionName: r.collectionName,
			Description:    "RAG document embeddings",
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: true,
					AutoID:     false,
					TypeParams: map[string]string{
						"max_length": "512",
					},
				},
				{
					Name:      "content",
					DataType:  entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "65535",
					},
				},
				{
					Name:     "embedding",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						"dim": fmt.Sprintf("%d", cfg.VectorDimension),
					},
				},
				{
					Name:     "kb_id",
					DataType: entity.FieldTypeInt64,
				},
				{
					Name:     "doc_id",
					DataType: entity.FieldTypeInt64,
				},
			},
		}

		if err := client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}

		r.logger.Info("Created Milvus collection", zap.String("collection", r.collectionName))

		// 创建索引
		idx, err := entity.NewIndexIvfFlat(entity.L2, 1024)
		if err != nil {
			return fmt.Errorf("failed to create index definition: %w", err)
		}

		if err := client.CreateIndex(ctx, r.collectionName, "embedding", idx, false); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}

		// 加载集合
		if err := client.LoadCollection(ctx, r.collectionName, false); err != nil {
			return fmt.Errorf("failed to load collection: %w", err)
		}
	}

	return nil
}

// AddDocuments 添加文档到向量数据库
func (r *MilvusRetriever) AddDocuments(ctx context.Context, docs []*schema.Document, kbID, docID uint) error {
	if len(docs) == 0 {
		return nil
	}
	
	// 检查连接状态
	if !r.IsConnected() {
		return fmt.Errorf("milvus is not connected")
	}

	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))
	kbIDs := make([]int64, len(docs))
	docIDs := make([]int64, len(docs))

	// 准备数据
	r.logger.Info("Starting to generate embeddings",
		zap.Int("doc_count", len(docs)),
		zap.Uint("kb_id", kbID),
		zap.Uint("doc_id", docID))
	
	for i, doc := range docs {
		ids[i] = doc.ID
		contents[i] = doc.Content

		// 记录当前处理进度
		if i%10 == 0 {
			r.logger.Info("Embedding generation progress",
				zap.Int("processed", i),
				zap.Int("total", len(docs)),
				zap.String("doc_id", doc.ID))
		}
		
		// 生成嵌入向量
		embedding, err := r.embedding.EmbedText(ctx, doc.Content)
		if err != nil {
			r.logger.Error("Failed to generate embedding",
				zap.String("doc_id", doc.ID),
				zap.Int("content_length", len(doc.Content)),
				zap.Error(err))
			return fmt.Errorf("failed to generate embedding for document %s: %w", doc.ID, err)
		}
		embeddings[i] = embedding

		kbIDs[i] = int64(kbID)
		docIDs[i] = int64(docID)
	}

	// 插入数据
	r.logger.Info("All embeddings generated, inserting to Milvus",
		zap.Int("doc_count", len(docs)),
		zap.String("collection", r.collectionName))
	
	insertCtx, cancel := context.WithTimeout(ctx, r.insertTimeout)
	defer cancel()

	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	
	if client == nil {
		return fmt.Errorf("milvus client is not initialized")
	}

	_, err := client.Insert(insertCtx, r.collectionName, "",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnFloatVector("embedding", int(r.embedding.GetDimension()), embeddings),
		entity.NewColumnInt64("kb_id", kbIDs),
		entity.NewColumnInt64("doc_id", docIDs),
	)
	if err != nil {
		return fmt.Errorf("failed to insert documents: %w", err)
	}

	r.logger.Info("Inserted documents to Milvus",
		zap.Int("count", len(docs)),
		zap.String("collection", r.collectionName))

	return nil
}

// Retrieve 检索相关文档
func (r *MilvusRetriever) Retrieve(ctx context.Context, query string, kbID uint) ([]*schema.Document, error) {
	// 检查连接状态
	if !r.IsConnected() {
		return nil, fmt.Errorf("milvus is not connected")
	}
	// 生成查询向量
	queryEmbedding, err := r.embedding.EmbedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// 构建搜索向量
	vectors := []entity.Vector{
		entity.FloatVector(queryEmbedding),
	}

	// 搜索参数
	sp, _ := entity.NewIndexFlatSearchParam()

	// 构建表达式
	expr := ""
	if kbID > 0 {
		expr = fmt.Sprintf("kb_id == %d", kbID)
	}

	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	
	if client == nil {
		return nil, fmt.Errorf("milvus client is not initialized")
	}

	// 执行搜索
	searchResult, err := client.Search(
		ctx,
		r.collectionName,
		nil,
		expr,
		[]string{"id", "content"},
		vectors,
		"embedding",
		entity.L2,
		r.topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// 转换结果
	var documents []*schema.Document
	for _, result := range searchResult {
		for i := 0; i < result.ResultCount; i++ {
			id, _ := result.Fields.GetColumn("id").Get(i)
			content, _ := result.Fields.GetColumn("content").Get(i)
			score, _ := result.IDs.Get(i)

			doc := &schema.Document{
				ID:      id.(string),
				Content: content.(string),
				MetaData: map[string]interface{}{
					"score":    score,
					"distance": result.Scores[i],
				},
			}
			documents = append(documents, doc)
		}
	}

	r.logger.Debug("Retrieved documents",
		zap.String("query", query),
		zap.Int("results", len(documents)))

	return documents, nil
}

// DeleteByKnowledgeBase 删除指定知识库的所有文档
func (r *MilvusRetriever) DeleteByKnowledgeBase(ctx context.Context, kbID uint) error {
	// 检查连接状态
	if !r.IsConnected() {
		return fmt.Errorf("milvus is not connected")
	}
	
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	
	if client == nil {
		return fmt.Errorf("milvus client is not initialized")
	}
	
	expr := fmt.Sprintf("kb_id == %d", kbID)
	err := client.Delete(ctx, r.collectionName, "", expr)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	r.logger.Info("Deleted documents from knowledge base",
		zap.Uint("kb_id", kbID))

	return nil
}

// DeleteByDocument 删除指定文档的所有向量
func (r *MilvusRetriever) DeleteByDocument(ctx context.Context, docID uint) error {
	// 检查连接状态
	if !r.IsConnected() {
		return fmt.Errorf("milvus is not connected")
	}
	
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()
	
	if client == nil {
		return fmt.Errorf("milvus client is not initialized")
	}
	
	expr := fmt.Sprintf("doc_id == %d", docID)
	err := client.Delete(ctx, r.collectionName, "", expr)
	if err != nil {
		return fmt.Errorf("failed to delete document vectors: %w", err)
	}

	r.logger.Info("Deleted document vectors",
		zap.Uint("doc_id", docID))

	return nil
}

// Close 关闭连接
func (r *MilvusRetriever) Close() error {
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// IsConnected 检查是否已连接
func (r *MilvusRetriever) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isConnected
}

// connect 连接到Milvus
func (r *MilvusRetriever) connect() error {
	ctx, cancel := context.WithTimeout(r.ctx, r.config.MilvusConnectTimeout)
	defer cancel()

	// 配置gRPC连接选项
	keepaliveParams := keepalive.ClientParameters{
		Time:                r.config.GRPCKeepaliveTime,
		Timeout:             r.config.GRPCKeepaliveTimeout,
		PermitWithoutStream: true,
	}

	// 创建Milvus客户端
	r.logger.Info("Connecting to Milvus", 
		zap.String("address", r.config.MilvusAddress),
		zap.String("collection", r.collectionName))
	
	c, err := client.NewClient(ctx, client.Config{
		Address: r.config.MilvusAddress,
		DialOptions: []grpc.DialOption{
			grpc.WithKeepaliveParams(keepaliveParams),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Milvus at %s: %w", r.config.MilvusAddress, err)
	}

	// 确保集合存在
	if err := r.ensureCollectionWithClient(ctx, c); err != nil {
		c.Close()
		return err
	}

	// 更新状态
	r.mu.Lock()
	if r.client != nil {
		r.client.Close()
	}
	r.client = c
	r.isConnected = true
	r.mu.Unlock()

	r.logger.Info("Successfully connected to Milvus", 
		zap.String("address", r.config.MilvusAddress))

	return nil
}

// reconnectLoop 重连循环
func (r *MilvusRetriever) reconnectLoop() {
	retryDelay := time.Second
	maxRetryDelay := 5 * time.Minute

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(retryDelay):
			if !r.IsConnected() {
				r.logger.Info("Attempting to reconnect to Milvus", 
					zap.Duration("retry_delay", retryDelay))
				
				if err := r.connect(); err != nil {
					r.logger.Error("Failed to reconnect to Milvus", 
						zap.Error(err),
						zap.Duration("next_retry", retryDelay*2))
					
					// 指数退避
					retryDelay = retryDelay * 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				} else {
					// 重连成功，重置延迟
					retryDelay = time.Second
				}
			} else {
				// 已连接，检查连接健康状态
				ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
				r.mu.RLock()
				client := r.client
				r.mu.RUnlock()
				
				if client != nil {
					// 简单的健康检查
					if _, err := client.HasCollection(ctx, r.collectionName); err != nil {
						r.logger.Warn("Health check failed, marking as disconnected", 
							zap.Error(err))
						r.mu.Lock()
						r.isConnected = false
						r.mu.Unlock()
					}
				}
				cancel()
			}
		}
	}
}
