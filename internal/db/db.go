package db

import (
	"fmt"
	"os"
	"path/filepath"

	"eino-rag/internal/config"
	"eino-rag/internal/models"
	"eino-rag/pkg/logger"

	"github.com/glebarez/sqlite" // 纯Go的SQLite GORM驱动
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	internalLogger "gorm.io/gorm/logger"
)

var db *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.Config) error {
	// 确保数据库目录存在
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// 设置日志级别
	logLevel := internalLogger.Error
	if cfg.GinMode == "debug" {
		logLevel = internalLogger.Info
	}

	// 打开数据库连接
	var err error
	db, err = gorm.Open(sqlite.Open(cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: internalLogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 设置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// SQLite 只支持一个写入者
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	// 自动迁移
	if err := models.Migrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化默认角色
	if err := models.InitRoles(db); err != nil {
		return fmt.Errorf("failed to init roles: %w", err)
	}
	
	// 创建初始管理员账户
	if err := createInitialAdmin(db); err != nil {
		return fmt.Errorf("failed to create initial admin: %w", err)
	}

	// 初始化系统配置
	if err := initSystemConfig(cfg); err != nil {
		return fmt.Errorf("failed to init system config: %w", err)
	}

	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return db
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// initSystemConfig 初始化系统配置
func initSystemConfig(cfg *config.Config) error {
	// 只初始化必要的默认配置，不覆盖环境变量中的配置
	// 这里只初始化一些系统级的配置，不初始化连接相关的配置
	defaultConfigs := map[string]string{
		// 系统基础配置
		"system_name": "Eino RAG",
		"system_desc": "基于Eino框架的企业级RAG系统",
		"admin_email": "admin@example.com",
		
		// 一些默认限制
		"max_kb_per_user": "10",
		"max_doc_per_kb": "100",
		"chat_history_days": "30",
	}

	for key, value := range defaultConfigs {
		var existing models.SystemConfig
		if err := db.Where("key = ?", key).First(&existing).Error; err == gorm.ErrRecordNotFound {
			if err := db.Create(&models.SystemConfig{
				Key:   key,
				Value: value,
			}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// Transaction 执行事务
func Transaction(fn func(*gorm.DB) error) error {
	return db.Transaction(fn)
}

// createInitialAdmin 创建初始管理员账户
func createInitialAdmin(db *gorm.DB) error {
	// 获取管理员角色
	var adminRole models.Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return fmt.Errorf("failed to find admin role: %w", err)
	}
	
	var count int64
	// 检查是否已存在管理员
	if err := db.Model(&models.User{}).Where("role_id = ?", adminRole.ID).Count(&count).Error; err != nil {
		return err
	}
	
	// 如果已存在管理员，则不创建
	if count > 0 {
		return nil
	}
	
	// 创建默认管理员
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte("admin123456"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	hashedPassword := string(hashedPasswordBytes)
	
	admin := &models.User{
		Name:     "系统管理员",
		Email:    "admin@eino-rag.com",
		Password: hashedPassword,
		RoleID:   adminRole.ID,
		Status:   "active",
	}
	
	if err := db.Create(admin).Error; err != nil {
		return err
	}
	
	logger.Get().Info("Created initial admin account", 
		zap.String("email", admin.Email),
		zap.String("name", admin.Name))
	
	return nil
}