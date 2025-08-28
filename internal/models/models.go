package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户表
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"size:100;not null" json:"name"`
	Email        string     `gorm:"size:100;unique;not null" json:"email"`
	Password     string     `gorm:"size:255;not null" json:"-"`
	Token        string     `gorm:"size:500" json:"token,omitempty"`
	RoleID       uint       `json:"role_id"`
	Role         *Role      `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	RoleName     string     `gorm:"-" json:"role_name"` // 计算字段，从Role获取
	Status       string     `gorm:"size:20;default:'active'" json:"status"` // active, inactive
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AfterFind hook to populate RoleName
func (u *User) AfterFind(tx *gorm.DB) error {
	if u.Role != nil {
		u.RoleName = u.Role.Name
	}
	return nil
}

// Role 角色权限表
type Role struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:50;unique;not null" json:"name"`
	Level       int       `gorm:"default:999" json:"level"`         // 权限等级(0最高)
	Permissions string    `gorm:"type:text" json:"permissions"`     // JSON array of permissions
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// KnowledgeBase 知识库表
type KnowledgeBase struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	DocCount    int       `gorm:"default:0" json:"doc_count"`
	Description string    `gorm:"type:text" json:"description"`
	CreatorID   uint      `json:"creator_id"`
	Creator     *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Document 文档表
type Document struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	KnowledgeBaseID uint           `json:"kb_id"`
	KnowledgeBase   *KnowledgeBase `gorm:"foreignKey:KnowledgeBaseID" json:"knowledge_base,omitempty"`
	FileName        string         `gorm:"size:255;not null" json:"file_name"`
	FileSize        int64          `json:"file_size"`
	Hash            string         `gorm:"size:64" json:"hash"`
	CreatorID       uint           `json:"creator_id"`
	Creator         *User          `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ChatHistory Chat对话记录表
type ChatHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `json:"user_id"`
	User         *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ConversationID string  `gorm:"size:36;not null" json:"conversation_id"` // UUID
	Title        string    `gorm:"size:200" json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SystemConfig 系统配置表
type SystemConfig struct {
	Key   string `gorm:"primaryKey;size:100" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

// ChatMessage Redis中存储的聊天消息
type ChatMessage struct {
	Role      string    `json:"role"`      // user/assistant
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation Redis中存储的对话
type Conversation struct {
	ID        string        `json:"id"`
	UserID    uint          `json:"user_id"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// TokenResponse Token响应
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	RoleName string `json:"role_name"`
	Status   string `json:"status"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleName string `json:"role_name"`
	Status   string `json:"status"`
}

// UpdateUserStatusRequest 更新用户状态请求
type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active inactive"`
}

// Migrate 自动迁移数据库表
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Role{},
		&KnowledgeBase{},
		&Document{},
		&ChatHistory{},
		&SystemConfig{},
	)
}

// InitRoles 初始化默认角色
func InitRoles(db *gorm.DB) error {
	roles := []Role{
		{
			Name:        "admin",
			Level:       0,
			Permissions: `["all"]`,
		},
		{
			Name:        "user",
			Level:       10,
			Permissions: `["chat", "view_kb", "upload_doc"]`,
		},
		{
			Name:        "guest",
			Level:       100,
			Permissions: `["chat", "view_kb"]`,
		},
	}

	for _, role := range roles {
		var existing Role
		if err := db.Where("name = ?", role.Name).First(&existing).Error; err == gorm.ErrRecordNotFound {
			if err := db.Create(&role).Error; err != nil {
				return err
			}
		}
	}

	return nil
}