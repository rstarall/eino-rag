package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/models"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

// InitRedis 初始化Redis连接
func InitRedis(cfg *config.Config) error {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse redis URL: %w", err)
	}

	if cfg.RedisPassword != "" {
		opt.Password = cfg.RedisPassword
	}
	opt.DB = cfg.RedisDB

	redisClient = redis.NewClient(opt)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	return nil
}

// GetRedis 获取Redis客户端
func GetRedis() *redis.Client {
	return redisClient
}

// CloseRedis 关闭Redis连接
func CloseRedis() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}

// 对话相关的Redis操作

// SaveConversation 保存对话到Redis
func SaveConversation(ctx context.Context, conv *models.Conversation) error {
	data, err := json.Marshal(conv)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	key := fmt.Sprintf("conversation:%s", conv.ID)
	return redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}

// GetConversation 从Redis获取对话
func GetConversation(ctx context.Context, convID string) (*models.Conversation, error) {
	key := fmt.Sprintf("conversation:%s", convID)
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	var conv models.Conversation
	if err := json.Unmarshal([]byte(data), &conv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	return &conv, nil
}

// AddMessageToConversation 添加消息到对话
func AddMessageToConversation(ctx context.Context, convID string, msg *models.ChatMessage) error {
	conv, err := GetConversation(ctx, convID)
	if err != nil {
		return err
	}

	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	conv.Messages = append(conv.Messages, *msg)
	conv.UpdatedAt = time.Now()

	return SaveConversation(ctx, conv)
}

// 缓存相关的Redis操作

// CacheSet 设置缓存
func CacheSet(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	return redisClient.Set(ctx, key, data, expiration).Err()
}

// CacheGet 获取缓存
func CacheGet(ctx context.Context, key string, dest interface{}) error {
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("failed to get cache: %w", err)
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return nil
}

// CacheDelete 删除缓存
func CacheDelete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return redisClient.Del(ctx, keys...).Err()
}

// CacheExists 检查缓存是否存在
func CacheExists(ctx context.Context, key string) (bool, error) {
	result, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// 向量缓存相关

// CacheEmbedding 缓存文本的向量
func CacheEmbedding(ctx context.Context, text string, embedding []float32) error {
	key := fmt.Sprintf("embedding:%x", hashString(text))
	data, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, key, data, 7*24*time.Hour).Err()
}

// GetCachedEmbedding 获取缓存的向量
func GetCachedEmbedding(ctx context.Context, text string) ([]float32, error) {
	key := fmt.Sprintf("embedding:%x", hashString(text))
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var embedding []float32
	if err := json.Unmarshal([]byte(data), &embedding); err != nil {
		return nil, err
	}

	return embedding, nil
}

// hashString 计算字符串的哈希值
func hashString(s string) uint64 {
	h := uint64(0)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint64(s[i])
	}
	return h
}