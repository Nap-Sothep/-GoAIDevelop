package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS配置
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// DefaultCORSConfig 默认CORS配置
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{"*"}, // 生产环境应配置具体域名
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
}

// CORSWithConfig 带配置的跨域中间件
func CORSWithConfig(cfg *CORSConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultCORSConfig()
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查来源是否在白名单中
		allowOrigin := "*"
		if len(cfg.AllowedOrigins) > 0 && cfg.AllowedOrigins[0] != "*" {
			for _, o := range cfg.AllowedOrigins {
				if o == origin {
					allowOrigin = origin
					break
				}
			}
			if allowOrigin == "*" && origin != "" {
				// 来源不在白名单中，拒绝
				c.AbortWithStatus(403)
				return
			}
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))

		// 允许携带凭证时不能使用通配符
		if allowOrigin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// CORS 跨域中间件（兼容旧接口，使用默认配置）
func CORS() gin.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}
