package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		// 脱敏处理：移除query中的敏感参数
		sanitizedQuery := sanitizeQuery(query)

		zap.L().Info("请求日志",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", sanitizedQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// sanitizeQuery 脱敏查询参数，移除敏感信息
func sanitizeQuery(query string) string {
	if query == "" {
		return ""
	}

	// 需要脱敏的参数名列表
	sensitiveParams := []string{
		"token", "access_token", "api_key", "apikey", "secret",
		"password", "passwd", "pwd", "auth", "session",
		"id_card", "idcard", "phone", "mobile",
	}

	result := query
	for _, param := range sensitiveParams {
		// 匹配 param=value 模式
		pairs := strings.Split(result, "&")
		var cleanPairs []string
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], param) {
				// 替换敏感值为 ***
				cleanPairs = append(cleanPairs, kv[0]+"=***")
			} else {
				cleanPairs = append(cleanPairs, pair)
			}
		}
		result = strings.Join(cleanPairs, "&")
	}

	return result
}
