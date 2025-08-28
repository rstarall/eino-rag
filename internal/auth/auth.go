package auth

import (
	"errors"
	"fmt"
	"time"

	"eino-rag/internal/db"
	"eino-rag/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// HashPassword 加密密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Register 用户注册
func Register(req *models.RegisterRequest) (*models.User, error) {
	database := db.GetDB()

	// 检查邮箱是否已存在
	var existingUser models.User
	if err := database.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("email already exists")
	}

	// 加密密码
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 获取默认角色
	var role models.Role
	if err := database.Where("name = ?", "user").First(&role).Error; err != nil {
		return nil, fmt.Errorf("failed to find default role: %w", err)
	}

	// 创建用户
	user := &models.User{
		Name:      req.Name,
		Email:     req.Email,
		Password:  hashedPassword,
		RoleID:    role.ID,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := database.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 重新加载用户信息（包含角色）
	if err := database.Preload("Role").First(user, user.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload user: %w", err)
	}

	return user, nil
}

// Login 用户登录
func Login(req *models.LoginRequest) (*models.TokenResponse, error) {
	database := db.GetDB()

	// 查找用户（包含角色信息）
	var user models.User
	if err := database.Preload("Role").Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 检查用户状态
	if user.Status != "active" {
		return nil, errors.New("user account is disabled")
	}

	// 验证密码
	if !CheckPassword(req.Password, user.Password) {
		return nil, errors.New("invalid email or password")
	}

	// 生成Token
	token, expiresAt, err := GenerateToken(&user)
	if err != nil {
		return nil, err
	}

	// 更新登录时间和Token
	now := time.Now()
	user.LastLoginAt = &now
	user.Token = token
	if err := database.Save(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &models.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
	}, nil
}

// GetUserByID 根据ID获取用户
func GetUserByID(userID uint) (*models.User, error) {
	database := db.GetDB()

	var user models.User
	if err := database.Preload("Role").First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail 根据邮箱获取用户
func GetUserByEmail(email string) (*models.User, error) {
	database := db.GetDB()

	var user models.User
	if err := database.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateUserToken 更新用户Token
func UpdateUserToken(userID uint, token string) error {
	database := db.GetDB()

	return database.Model(&models.User{}).
		Where("id = ?", userID).
		Update("token", token).Error
}

// CheckPermission 检查用户权限
func CheckPermission(user *models.User, permission string) (bool, error) {
	// 如果用户已经预加载了角色
	if user.Role != nil {
		// 管理员拥有所有权限
		if user.Role.Level == 0 {
			return true, nil
		}
		// TODO: 实现更复杂的权限检查逻辑
		return true, nil
	}

	// 如果没有预加载角色，则加载
	database := db.GetDB()
	var role models.Role
	if err := database.First(&role, user.RoleID).Error; err != nil {
		return false, fmt.Errorf("failed to get role: %w", err)
	}

	// 管理员拥有所有权限
	if role.Level == 0 {
		return true, nil
	}

	// 检查具体权限
	// 这里简化处理，实际应解析JSON
	// TODO: 实现更复杂的权限检查逻辑
	return true, nil
}