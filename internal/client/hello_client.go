package client

import (
	"context"
	"fmt"
	"time"

	hellov1 "go-gateway/api/hello/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// HelloClient Hello服务的gRPC客户端封装
type HelloClient struct {
	conn   *grpc.ClientConn
	client hellov1.HelloServiceClient
}

// NewHelloClient 创建Hello客户端并建立连接
func NewHelloClient(target string) (*HelloClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("连接Hello gRPC服务失败 [target=%s]: %w", target, err)
	}

	return &HelloClient{
		conn:   conn,
		client: hellov1.NewHelloServiceClient(conn),
	}, nil
}

// SayHello 调用SayHello Unary RPC
func (c *HelloClient) SayHello(ctx context.Context, name string) (*hellov1.SayHelloResponse, error) {
	resp, err := c.client.SayHello(ctx, &hellov1.SayHelloRequest{
		Name: name,
	})
	if err != nil {
		return nil, fmt.Errorf("SayHello RPC调用失败: %w", err)
	}
	return resp, nil
}

// ChatStream 调用ChatStream双向流RPC
func (c *HelloClient) ChatStream(ctx context.Context) (hellov1.HelloService_ChatStreamClient, error) {
	stream, err := c.client.ChatStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("ChatStream RPC调用失败: %w", err)
	}
	return stream, nil
}

// Close 关闭gRPC连接
func (c *HelloClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
