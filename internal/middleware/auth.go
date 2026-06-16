package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthConfig 认证中间件配置
type AuthConfig struct {
	Enabled        bool     // 是否启用认证
	SkipPaths      []string // 跳过认证的路径（如健康检查）
	AllowedOrigins []string // WebSocket允许的源列表
}

// DefaultAuthConfig 默认认证配置（开发模式：认证禁用）
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:   false, // 开发阶段默认禁用，生产环境应设为true
		SkipPaths: []string{"/health", "/api/v1/hello"},
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		},
	}
}

// Auth 认证中间件（JWT占位实现）
func Auth(cfg *AuthConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultAuthConfig()
	}

	// 构建跳过路径映射
	skipMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		// 跳过指定路径
		if skipMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// 如果认证未启用，直接放行（开发模式）
		if !cfg.Enabled {
			c.Set("auth_enabled", false)
			c.Next()
			return
		}

		// 提取并验证JWT Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "缺少认证令牌",
			})
			c.Abort()
			return
		}

		// 提取Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "无效的认证格式，应为: Bearer <token>",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// TODO: 实际的JWT验证逻辑
		// claims, err := jwt.Parse(token, secretKey)
		// if err != nil { ... }

		// 占位：简单验证token非空
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "无效的认证令牌",
			})
			c.Abort()
			return
		}

		// 将用户信息存入上下文（占位）
		c.Set("user_id", "demo_user")
		c.Set("auth_enabled", true)

		zap.L().Debug("认证成功", zap.String("path", c.Request.URL.Path))
		c.Next()
	}
}

// CheckWebSocketOrigin WebSocket来源验证函数
func CheckWebSocketOrigin(allowedOrigins []string) func(r *http.Request) bool {
	if len(allowedOrigins) == 0 {
		// 如果没有配置允许的源，默认允许localhost（开发友好）
		return func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1")
		}
	}

	// 构建允许的来源映射
	originMap := make(map[string]bool)
	for _, o := range allowedOrigins {
		originMap[o] = true
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// 允许非浏览器客户端（如curl、grpcurl）
			return true
		}
		if originMap[origin] {
			return true
		}
		zap.L().Warn("WebSocket来源被拒绝", zap.String("origin", origin))
		return false
	}
}
