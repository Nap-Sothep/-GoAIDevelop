package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-gateway/internal/handler"
	"go-gateway/internal/middleware"
)

// RouterConfig 路由器配置
type RouterConfig struct {
	AuthEnabled        bool
	AllowedOrigins     []string
	SkipAuthPaths      []string
}

// DefaultRouterConfig 默认路由配置
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		AuthEnabled:   false, // 开发模式默认禁用认证
		AllowedOrigins: []string{}, // 空列表表示允许localhost
		SkipAuthPaths: []string{"/health", "/api/v1/hello"},
	}
}

// SetupRouter 配置所有路由
func SetupRouter(
	httpHandler *handler.HelloHTTPHandler,
	wsHandler *handler.HelloWSHandler,
) *gin.Engine {
	return SetupRouterWithConfig(httpHandler, wsHandler, DefaultRouterConfig())
}

// SetupRouterWithConfig 带配置的路由设置
func SetupRouterWithConfig(
	httpHandler *handler.HelloHTTPHandler,
	wsHandler *handler.HelloWSHandler,
	cfg *RouterConfig,
) *gin.Engine {
	r := gin.New()

	// 全局中间件
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORSWithConfig(&middleware.CORSConfig{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))

	// 认证中间件（可选）
	authCfg := &middleware.AuthConfig{
		Enabled:        cfg.AuthEnabled,
		SkipPaths:      cfg.SkipAuthPaths,
		AllowedOrigins: cfg.AllowedOrigins,
	}
	r.Use(middleware.Auth(authCfg))

	// 健康检查（跳过认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "msg": "ok"})
	})

	// API路由组
	api := r.Group("/api/v1")
	{
		api.POST("/hello", httpHandler.SayHello)
	}

	// WebSocket路由（使用来源验证）
	r.GET("/ws/chat", gin.WrapH(http.HandlerFunc(wsHandler.ChatStream)))

	return r
}
