package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/db"
	"eino-rag/internal/handlers"
	"eino-rag/internal/middleware"
	"eino-rag/internal/services/chat"
	"eino-rag/internal/services/document"
	"eino-rag/internal/services/rag"
	"eino-rag/pkg/logger"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// @title Eino RAG API
// @version 1.0
// @description 基于Eino框架的企业级RAG系统API
// @description 支持文档管理、向量检索、智能对话等功能

// @contact.name API Support
// @contact.url https://github.com/yourusername/eino-rag
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Bearer token authentication

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	if err := logger.Init(cfg.GinMode); err != nil {
		log.Fatal("Failed to init logger:", err)
	}
	defer logger.Sync()

	log := logger.Get()
	log.Info("Starting Eino RAG server...")

	// 初始化数据库
	if err := db.Init(cfg); err != nil {
		log.Fatal("Failed to init database", zap.Error(err))
	}
	defer db.Close()

	// 从数据库加载配置
	loadConfigFromDB(cfg, log)

	// 初始化Redis
	if err := db.InitRedis(cfg); err != nil {
		log.Fatal("Failed to init Redis", zap.Error(err))
	}
	defer db.CloseRedis()

	// 初始化服务
	embeddingService := rag.NewEmbeddingService(cfg, log)

	var retriever *rag.MilvusRetriever
	var err error
	retriever, err = rag.NewMilvusRetriever(cfg, embeddingService, log)
	if err != nil {
		// 记录错误但不退出，允许应用继续运行
		log.Warn("Failed to create retriever, vector search features will be unavailable",
			zap.Error(err),
			zap.String("milvus_address", cfg.MilvusAddress))
		// 设置 retriever 为 nil，后续代码需要检查
		retriever = nil
	} else {
		defer retriever.Close()
	}

	// 初始化文档服务
	docParser := document.NewDocumentParser(log)
	docProcessor := document.NewDocumentProcessor(cfg, log)
	docService := document.NewService(docParser, docProcessor, retriever, cfg, log)

	// 初始化聊天服务
	chatService, err := chat.NewService(docService, cfg, log)
	if err != nil {
		log.Fatal("Failed to create chat service", zap.Error(err))
	}

	// 初始化处理器
	authHandler := handlers.NewAuthHandler(log)
	docHandler := handlers.NewDocumentHandler(docService, log)
	chatHandler := handlers.NewChatHandler(chatService, log)
	kbHandler := handlers.NewKnowledgeBaseHandler(retriever, log)
	sysHandler := handlers.NewSystemHandler(cfg, log)
	userHandler := handlers.NewUserHandler(log)

	// 设置Gin
	gin.SetMode(cfg.GinMode)
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(log))
	router.Use(middleware.CORS())

	// 静态文件
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/**/*.html")

	// 前端路由
	setupFrontendRoutes(router)

	// API路由
	api := router.Group("/api")
	{
		// 健康检查
		api.GET("/health", sysHandler.Health)

		// 认证路由
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)

			// 需要认证的路由
			authRequired := auth.Group("")
			authRequired.Use(middleware.AuthMiddleware())
			{
				authRequired.POST("/logout", authHandler.Logout)
				authRequired.GET("/profile", authHandler.GetProfile)
				authRequired.POST("/refresh", authHandler.RefreshToken)
			}
		}

		// 需要认证的API路由
		authorized := api.Group("")
		authorized.Use(middleware.AuthMiddleware())
		{
			// 知识库管理
			kb := authorized.Group("/knowledge-bases")
			{
				kb.POST("", kbHandler.Create)
				kb.GET("", kbHandler.List)
				kb.GET("/:id", kbHandler.Get)
				kb.PUT("/:id", kbHandler.Update)
				kb.DELETE("/:id", kbHandler.Delete)
				kb.GET("/:id/documents", docHandler.List)
			}

			// 文档管理
			docs := authorized.Group("/documents")
			{
				docs.GET("", docHandler.ListAll) // 获取所有文档
				docs.POST("/upload", docHandler.Upload)
				docs.POST("/search", docHandler.Search)
				docs.DELETE("/:id", docHandler.Delete)
			}

			// 聊天功能
			chat := authorized.Group("/chat")
			{
				chat.POST("", chatHandler.Chat)
				chat.POST("/stream", chatHandler.ChatStream)
				chat.GET("/conversations", chatHandler.ListConversations)
				chat.GET("/conversations/:id", chatHandler.GetConversation)
			}

			// 系统管理（需要管理员权限）
			system := authorized.Group("/system")
			system.Use(middleware.RequireRole("admin"))
			{
				system.GET("/config", sysHandler.GetConfig)
				system.PUT("/config", sysHandler.UpdateConfig)
			}

			// 系统统计（所有登录用户可访问）
			authorized.GET("/system/stats", sysHandler.GetStats)

			// 用户管理（需要管理员权限）
			users := authorized.Group("/users")
			users.Use(middleware.RequireRole("admin"))
			{
				users.GET("", userHandler.ListUsers)
				users.GET("/:id", userHandler.GetUser)
				users.POST("", userHandler.CreateUser)
				users.PUT("/:id", userHandler.UpdateUser)
				users.DELETE("/:id", userHandler.DeleteUser)
				users.PUT("/:id/status", userHandler.UpdateUserStatus)
			}
		}
	}

	// Swagger文档
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 启动服务器
	srv := &http.Server{
		Addr:    cfg.ServerHost + ":" + cfg.ServerPort,
		Handler: router,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	log.Info("Server started",
		zap.String("host", cfg.ServerHost),
		zap.String("port", cfg.ServerPort),
		zap.String("mode", cfg.GinMode))

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 5秒超时关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// setupFrontendRoutes 设置前端路由
func setupFrontendRoutes(router *gin.Engine) {
	// 首页 - 聊天界面
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "chat.html", gin.H{
			"title": "Eino RAG - 智能对话",
		})
	})

	// 登录页
	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "登录 - Eino RAG",
		})
	})

	// 注册页
	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{
			"title": "注册 - Eino RAG",
		})
	})

	// 管理后台
	admin := router.Group("/admin")
	{
		// 仪表板
		admin.GET("/dashboard", func(c *gin.Context) {
			c.HTML(http.StatusOK, "admin/dashboard.html", gin.H{
				"title": "管理后台 - Eino RAG",
			})
		})

		// 默认重定向到仪表板
		admin.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/admin/dashboard")
		})

		// 知识库管理
		admin.GET("/knowledge-bases", func(c *gin.Context) {
			c.HTML(http.StatusOK, "admin/knowledge-bases.html", gin.H{
				"title": "知识库管理 - Eino RAG",
			})
		})

		// 文档管理
		admin.GET("/documents", func(c *gin.Context) {
			c.HTML(http.StatusOK, "admin/documents.html", gin.H{
				"title": "文档管理 - Eino RAG",
			})
		})

		// 用户管理
		admin.GET("/users", func(c *gin.Context) {
			c.HTML(http.StatusOK, "admin/users.html", gin.H{
				"title": "用户管理 - Eino RAG",
			})
		})

		// 系统设置
		admin.GET("/settings", func(c *gin.Context) {
			c.HTML(http.StatusOK, "admin/settings.html", gin.H{
				"title": "系统设置 - Eino RAG",
			})
		})
	}
}

// loadConfigFromDB 从数据库加载配置
func loadConfigFromDB(cfg *config.Config, log *zap.Logger) {
	// 先打印从环境变量加载的配置
	log.Info("Configuration loaded from environment",
		zap.String("milvus_address", cfg.MilvusAddress),
		zap.String("ollama_url", cfg.OllamaBaseURL),
		zap.String("embedding_model", cfg.EmbeddingModel))

	// 获取数据库中的所有配置
	var configs []struct {
		Key   string
		Value string
	}

	database := db.GetDB()
	if err := database.Table("system_configs").Select("key, value").Find(&configs).Error; err != nil {
		log.Error("Failed to load config from database", zap.Error(err))
		return
	}

	if len(configs) == 0 {
		log.Info("No configuration found in database, using environment values")
		return
	}

	// 构建配置映射，只包含非空值
	configMap := make(map[string]string)
	overrideCount := 0
	for _, c := range configs {
		// 只有当数据库中的值非空时才覆盖
		if c.Value != "" {
			configMap[c.Key] = c.Value

			// 记录哪些配置将被覆盖
			switch c.Key {
			case "milvus_address", "ollama_url", "embedding_model", "openai_api_key", "allowed_file_types":
				log.Info("Overriding config from database",
					zap.String("key", c.Key),
					zap.String("value", c.Value))
				overrideCount++
			}
		}
	}

	// 更新配置
	if overrideCount > 0 {
		config.UpdateFromDB(configMap)
		log.Info("Updated configuration from database",
			zap.Int("total_configs", len(configs)),
			zap.Int("overrides", overrideCount))

		// 打印更新后的配置
		log.Info("Final configuration",
			zap.String("milvus_address", cfg.MilvusAddress),
			zap.String("ollama_url", cfg.OllamaBaseURL))
	} else {
		log.Info("No configuration overrides from database, using environment values")
	}
}
