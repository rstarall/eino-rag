package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eino-rag/components"
	"eino-rag/config"
	"eino-rag/handlers"
	"eino-rag/services"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 初始化日志
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 加载配置
	cfg := config.Load()

	// 初始化Ollama嵌入
	embedding := components.NewOllamaEmbedding(
		cfg.OllamaBaseURL,
		cfg.EmbeddingModel,
		cfg.VectorDimension,
		logger,
	)

	// 初始化Milvus检索器
	retriever, err := components.NewMilvusRetriever(
		cfg.MilvusHost,
		cfg.MilvusPort,
		cfg.CollectionName,
		embedding,
		cfg.TopK,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to create retriever", zap.Error(err))
	}
	defer retriever.Close()

	// 初始化文档处理器（使用语义分割）
	processor, err := components.NewDocumentProcessor(
		embedding,             // 使用同一个嵌入器进行语义分割
		cfg.ChunkSize,         // 作为最小分块大小
		cfg.ChunkSize*3,       // 最大分块大小
		cfg.SemanticSplitting, // 是否启用语义分割
		logger,                // 传递logger
	)
	if err != nil {
		logger.Fatal("Failed to create document processor", zap.Error(err))
	}

	// 初始化RAG服务
	ragService, err := services.NewRAGService(retriever, processor, logger)
	if err != nil {
		logger.Fatal("Failed to create RAG service", zap.Error(err))
	}

	// 初始化OpenAI ChatModel（如果配置了API Key）
	var chatModel *openai.ChatModel
	if cfg.OpenAIAPIKey != "" {
		chatModelConfig := &openai.ChatModelConfig{
			APIKey:  cfg.OpenAIAPIKey,
			Model:   cfg.OpenAIModel,
			Timeout: 60 * time.Second,
		}

		// 如果配置了自定义BaseURL，使用它
		if cfg.OpenAIBaseURL != "" {
			chatModelConfig.BaseURL = cfg.OpenAIBaseURL
		}

		var err error
		chatModel, err = openai.NewChatModel(context.Background(), chatModelConfig)
		if err != nil {
			logger.Warn("Failed to initialize OpenAI ChatModel, will use fallback responses", zap.Error(err))
			chatModel = nil
		} else {
			logger.Info("OpenAI ChatModel initialized successfully", zap.String("model", cfg.OpenAIModel))
		}
	} else {
		logger.Info("OpenAI API key not configured, using fallback responses")
	}

	// 初始化API处理器
	apiHandler := handlers.NewAPIHandler(ragService, cfg.MaxUploadSize, chatModel, logger)

	// 设置Gin路由
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// 加载HTML模板
	router.LoadHTMLGlob("frontend/*.html")

	// 设置前端路由和API路由
	apiHandler.SetupFrontendRoutes(router)

	// 启动服务器
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Server started", zap.String("port", cfg.ServerPort))

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 设置5秒的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
