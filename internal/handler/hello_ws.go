package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	hellov1 "go-gateway/api/hello/v1"
	"go-gateway/internal/client"
	"go-gateway/internal/middleware"
)

// wsUpgrader WebSocket升级器（使用默认来源检查）
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     middleware.CheckWebSocketOrigin(nil), // 默认允许localhost
}

// HelloWSHandler Hello服务的WebSocket处理器
type HelloWSHandler struct {
	client *client.HelloClient
}

// NewHelloWSHandler 创建WebSocket处理器
func NewHelloWSHandler(c *client.HelloClient) *HelloWSHandler {
	return &HelloWSHandler{client: c}
}

// NewHelloWSHandlerWithOrigins 创建带来源白名单的WebSocket处理器
func NewHelloWSHandlerWithOrigins(c *client.HelloClient, allowedOrigins []string) *HelloWSHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     middleware.CheckWebSocketOrigin(allowedOrigins),
	}
	wsUpgrader = upgrader
	return &HelloWSHandler{client: c}
}

// WSChatMessage WebSocket层传输的消息格式
type WSChatMessage struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// ChatStream 处理 WebSocket /ws/chat
// 双向流: WebSocket ↔ gRPC Stream
func (h *HelloWSHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		zap.L().Error("WebSocket升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	ctx := r.Context()
	stream, err := h.client.ChatStream(ctx)
	if err != nil {
		zap.L().Error("创建gRPC流失败", zap.String("method", "ChatStream"), zap.Error(err))
		conn.WriteJSON(map[string]interface{}{"error": "创建gRPC流失败"})
		return
	}

	// goroutine-1: 从gRPC流接收消息 → 写入WebSocket
	recvDone := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("接收goroutine panic", zap.Any("panic", r))
			}
			close(recvDone)
		}()
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code() != codes.Canceled {
					zap.L().Error("接收gRPC消息失败", zap.Error(err))
				}
				return
			}

			// 将gRPC消息转换为WebSocket消息
			wsMsg := WSChatMessage{
				User:      msg.GetUser(),
				Text:      msg.GetText(),
				Timestamp: msg.GetTimestamp(),
			}
			if err := conn.WriteJSON(wsMsg); err != nil {
				zap.L().Error("写入WebSocket失败", zap.Error(err))
				return
			}
		}
	}()

	// goroutine-2: 从WebSocket读取消息 → 发送到gRPC流
	for {
		var wsMsg WSChatMessage
		if err := conn.ReadJSON(&wsMsg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				zap.L().Warn("WebSocket异常关闭", zap.Error(err))
			}
			break
		}

		grpcMsg := &hellov1.ChatMessage{
			User:      wsMsg.User,
			Text:      wsMsg.Text,
			Timestamp: time.Now().Unix(),
		}
		if err := stream.Send(grpcMsg); err != nil {
			if err == io.EOF {
				break
			}
			zap.L().Error("发送gRPC消息失败", zap.Error(err))
			break
		}
	}

	// 关闭发送端，通知gRPC流已结束
	stream.CloseSend()
	// 等待接收goroutine完成
	<-recvDone
}
