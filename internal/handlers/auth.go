package handlers

import (
	"net/http"

	"eino-rag/internal/auth"
	"eino-rag/internal/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthHandler struct {
	logger *zap.Logger
}

func NewAuthHandler(logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		logger: logger,
	}
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "注册信息"
// @Success 200 {object} models.User "注册成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 409 {object} ErrorResponse "邮箱已存在"
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid register request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	user, err := auth.Register(&req)
	if err != nil {
		h.logger.Error("Failed to register user", zap.Error(err))
		status := http.StatusInternalServerError
		message := "Failed to register user"
		
		if err.Error() == "email already exists" {
			status = http.StatusConflict
			message = err.Error()
		}
		
		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	h.logger.Info("User registered successfully", zap.String("email", user.Email))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用邮箱和密码登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "登录信息"
// @Success 200 {object} models.TokenResponse "登录成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "邮箱或密码错误"
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid login request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	tokenResp, err := auth.Login(&req)
	if err != nil {
		h.logger.Error("Failed to login", zap.Error(err))
		status := http.StatusInternalServerError
		message := "Failed to login"
		
		if err.Error() == "invalid email or password" {
			status = http.StatusUnauthorized
			message = err.Error()
		}
		
		c.JSON(status, ErrorResponse{
			Success: false,
			Message: message,
		})
		return
	}

	h.logger.Info("User logged in successfully", zap.String("email", req.Email))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tokenResp,
	})
}

// Logout 用户登出
// @Summary 用户登出
// @Description 登出当前用户
// @Tags 认证
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} SuccessResponse "登出成功"
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, _ := c.Get("user_id")
	
	// 清除用户token
	if uid, ok := userID.(uint); ok {
		if err := auth.UpdateUserToken(uid, ""); err != nil {
			h.logger.Error("Failed to clear user token", zap.Error(err))
		}
	}

	h.logger.Info("User logged out", zap.Any("user_id", userID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}

// GetProfile 获取用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} models.User "用户信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Router /api/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found in context",
		})
		return
	}

	user, err := auth.GetUserByID(userID.(uint))
	if err != nil {
		h.logger.Error("Failed to get user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get user profile",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// RefreshToken 刷新Token
// @Summary 刷新Token
// @Description 使用旧Token刷新获取新Token
// @Tags 认证
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} models.TokenResponse "新Token"
// @Failure 401 {object} ErrorResponse "Token无效"
// @Router /api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	
	user, err := auth.GetUserByID(userID.(uint))
	if err != nil {
		h.logger.Error("Failed to get user for token refresh", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to refresh token",
		})
		return
	}

	token, expiresAt, err := auth.GenerateToken(user)
	if err != nil {
		h.logger.Error("Failed to generate new token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to generate new token",
		})
		return
	}

	// 更新用户token
	user.Token = token
	if err := auth.UpdateUserToken(user.ID, token); err != nil {
		h.logger.Error("Failed to update user token", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": models.TokenResponse{
			Token:     token,
			ExpiresAt: expiresAt,
			User:      *user,
		},
	})
}