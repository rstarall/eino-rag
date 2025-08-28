package auth

import (
	"errors"
	"fmt"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT claims结构
type Claims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	RoleName string `json:"role_name"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT token
func GenerateToken(user *models.User) (string, time.Time, error) {
	cfg := config.Get()
	expiresAt := time.Now().Add(time.Duration(cfg.JWTExpireHours) * time.Hour)

	claims := &Claims{
		UserID:   user.ID,
		Email:    user.Email,
		RoleName: user.RoleName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "eino-rag",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken 验证JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// RefreshToken 刷新Token
func RefreshToken(oldToken string) (string, time.Time, error) {
	claims, err := ValidateToken(oldToken)
	if err != nil {
		return "", time.Time{}, err
	}

	// 创建新的token
	user := &models.User{
		ID:       claims.UserID,
		Email:    claims.Email,
		RoleName: claims.RoleName,
	}

	return GenerateToken(user)
}