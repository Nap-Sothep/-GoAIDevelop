package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go-gateway/internal/client"
)

// HelloHTTPHandler Hello服务的HTTP处理器
type HelloHTTPHandler struct {
	client *client.HelloClient
}

// NewHelloHTTPHandler 创建HTTP处理器
func NewHelloHTTPHandler(c *client.HelloClient) *HelloHTTPHandler {
	return &HelloHTTPHandler{client: c}
}

// SayHelloRequest HTTP请求参数
type SayHelloRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

// SayHello 处理 POST /api/v1/hello
func (h *HelloHTTPHandler) SayHello(c *gin.Context) {
	var req SayHelloRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	rsp, err := h.client.SayHello(c.Request.Context(), req.Name)
	if err != nil {
		zap.L().Error("gRPC调用失败", zap.String("method", "SayHello"), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "服务调用失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"reply": rsp.GetMessage(),
		},
	})
}
