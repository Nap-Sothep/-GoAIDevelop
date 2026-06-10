package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-gateway/internal/handler"
	"go-gateway/internal/middleware"
)

// SetupRouter 配置所有路由
func SetupRouter(
	httpHandler *handler.HelloHTTPHandler,
	wsHandler *handler.HelloWSHandler,
) *gin.Engine {
	r := gin.New()

	// 全局中间件
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "msg": "ok"})
	})

	// API路由组
	api := r.Group("/api/v1")
	{
		api.POST("/hello", httpHandler.SayHello)
	}

	// WebSocket路由
	r.GET("/ws/chat", gin.WrapH(http.HandlerFunc(wsHandler.ChatStream)))

	return r
}
