package handlers

import (
	"net/http"
	"strconv"

	"eino-rag/internal/auth"
	"eino-rag/internal/db"
	"eino-rag/internal/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserHandler struct {
	logger *zap.Logger
}

func NewUserHandler(logger *zap.Logger) *UserHandler {
	return &UserHandler{
		logger: logger,
	}
}

// ListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 获取系统中的所有用户（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} map[string]interface{} "用户列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Router /api/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	
	offset := (page - 1) * pageSize
	
	// 查询用户列表
	var users []models.User
	var total int64
	
	query := db.GetDB().Model(&models.User{})
	
	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("Failed to count users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to count users",
		})
		return
	}
	
	// 获取用户列表
	if err := query.Preload("Role").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		h.logger.Error("Failed to get users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get users",
		})
		return
	}
	
	// 清理敏感信息
	for i := range users {
		users[i].Password = ""
		users[i].Token = ""
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"users":   users,
		"total":   total,
		"page":    page,
		"page_size": pageSize,
	})
}

// GetUser 获取用户详情
// @Summary 获取用户详情
// @Description 根据ID获取用户详细信息（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "用户ID"
// @Success 200 {object} models.User "用户信息"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /api/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid user ID",
		})
		return
	}
	
	var user models.User
	if err := db.GetDB().Preload("Role").First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Message: "User not found",
			})
			return
		}
		
		h.logger.Error("Failed to get user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get user",
		})
		return
	}
	
	// 清理敏感信息
	user.Password = ""
	user.Token = ""
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// CreateUser 创建用户
// @Summary 创建用户
// @Description 创建新用户（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body models.CreateUserRequest true "用户信息"
// @Success 200 {object} models.User "创建的用户"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 409 {object} ErrorResponse "用户已存在"
// @Router /api/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid create user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}
	
	// 检查邮箱是否已存在
	var existingUser models.User
	if err := db.GetDB().Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Success: false,
			Message: "Email already exists",
		})
		return
	}
	
	// 获取角色
	var role models.Role
	if req.RoleName != "" {
		if err := db.GetDB().Where("name = ?", req.RoleName).First(&role).Error; err != nil {
			h.logger.Error("Failed to find role", zap.Error(err), zap.String("role", req.RoleName))
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Success: false,
				Message: "Invalid role",
			})
			return
		}
	} else {
		// 默认角色为user
		if err := db.GetDB().Where("name = ?", "user").First(&role).Error; err != nil {
			h.logger.Error("Failed to find default role", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Success: false,
				Message: "Failed to find default role",
			})
			return
		}
	}
	
	// 创建用户
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to process password",
		})
		return
	}
	
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		RoleID:   role.ID,
		Status:   req.Status,
	}
	
	if user.Status == "" {
		user.Status = "active"
	}
	
	if err := db.GetDB().Create(&user).Error; err != nil {
		h.logger.Error("Failed to create user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to create user",
		})
		return
	}
	
	// 重新加载用户信息（包含角色）
	if err := db.GetDB().Preload("Role").First(&user, user.ID).Error; err != nil {
		h.logger.Error("Failed to reload user", zap.Error(err))
	}
	
	// 清理敏感信息
	user.Password = ""
	
	h.logger.Info("User created successfully", zap.String("email", user.Email))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// UpdateUser 更新用户
// @Summary 更新用户信息
// @Description 更新用户信息（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "用户ID"
// @Param request body models.UpdateUserRequest true "更新信息"
// @Success 200 {object} models.User "更新后的用户"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /api/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid user ID",
		})
		return
	}
	
	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid update user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}
	
	// 获取用户
	var user models.User
	if err := db.GetDB().First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Success: false,
				Message: "User not found",
			})
			return
		}
		
		h.logger.Error("Failed to get user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get user",
		})
		return
	}
	
	// 更新字段
	updates := make(map[string]interface{})
	
	if req.Name != "" {
		updates["name"] = req.Name
	}
	
	if req.Email != "" && req.Email != user.Email {
		// 检查邮箱是否已被使用
		var existingUser models.User
		if err := db.GetDB().Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, ErrorResponse{
				Success: false,
				Message: "Email already exists",
			})
			return
		}
		updates["email"] = req.Email
	}
	
	if req.Password != "" {
		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			h.logger.Error("Failed to hash password", zap.Error(err))
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Success: false,
				Message: "Failed to process password",
			})
			return
		}
		updates["password"] = hashedPassword
	}
	
	if req.RoleName != "" {
		var role models.Role
		if err := db.GetDB().Where("name = ?", req.RoleName).First(&role).Error; err != nil {
			h.logger.Error("Failed to find role", zap.Error(err), zap.String("role", req.RoleName))
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Success: false,
				Message: "Invalid role",
			})
			return
		}
		updates["role_id"] = role.ID
	}
	
	if req.Status != "" {
		updates["status"] = req.Status
	}
	
	// 执行更新
	if err := db.GetDB().Model(&user).Updates(updates).Error; err != nil {
		h.logger.Error("Failed to update user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to update user",
		})
		return
	}
	
	// 重新加载用户信息
	if err := db.GetDB().Preload("Role").First(&user, user.ID).Error; err != nil {
		h.logger.Error("Failed to reload user", zap.Error(err))
	}
	
	// 清理敏感信息
	user.Password = ""
	user.Token = ""
	
	h.logger.Info("User updated successfully", zap.Uint("user_id", uint(userID)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 删除指定用户（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "用户ID"
// @Success 200 {object} SuccessResponse "删除成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /api/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid user ID",
		})
		return
	}
	
	// 不允许删除ID为1的管理员
	if userID == 1 {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Success: false,
			Message: "Cannot delete primary admin user",
		})
		return
	}
	
	// 不允许用户删除自己
	currentUserID, _ := c.Get("user_id")
	if uint(userID) == currentUserID.(uint) {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Success: false,
			Message: "Cannot delete your own account",
		})
		return
	}
	
	// 执行删除
	result := db.GetDB().Delete(&models.User{}, userID)
	if result.Error != nil {
		h.logger.Error("Failed to delete user", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to delete user",
		})
		return
	}
	
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}
	
	h.logger.Info("User deleted successfully", zap.Uint("user_id", uint(userID)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// UpdateUserStatus 更新用户状态
// @Summary 更新用户状态
// @Description 激活或禁用用户（需要管理员权限）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "用户ID"
// @Param request body models.UpdateUserStatusRequest true "状态信息"
// @Success 200 {object} SuccessResponse "更新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /api/users/{id}/status [put]
func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid user ID",
		})
		return
	}
	
	var req models.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid update status request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}
	
	if req.Status != "active" && req.Status != "inactive" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid status value",
		})
		return
	}
	
	// 更新状态
	result := db.GetDB().Model(&models.User{}).Where("id = ?", userID).Update("status", req.Status)
	if result.Error != nil {
		h.logger.Error("Failed to update user status", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to update user status",
		})
		return
	}
	
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}
	
	h.logger.Info("User status updated successfully", 
		zap.Uint("user_id", uint(userID)),
		zap.String("status", req.Status))
		
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User status updated successfully",
	})
}