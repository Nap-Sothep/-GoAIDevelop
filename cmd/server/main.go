package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	hellov1 "go-gateway/api/hello/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// helloServer 实现 HelloServiceServer 接口
type helloServer struct {
	hellov1.UnimplementedHelloServiceServer
}

// SayHello 实现 Unary RPC
func (s *helloServer) SayHello(ctx context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	name := req.GetName()
	if name == "" {
		name = "陌生人"
	}
	return &hellov1.SayHelloResponse{
		Message: fmt.Sprintf("你好, %s! (来自gRPC后端)", name),
	}, nil
}

// ChatStream 实现双向流 RPC
func (s *helloServer) ChatStream(stream hellov1.HelloService_ChatStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 收到消息后，回一条"已收到"的确认
		reply := &hellov1.ChatMessage{
			User:      "系统",
			Text:      fmt.Sprintf("已收到 %s 的消息: %s", msg.GetUser(), msg.GetText()),
			Timestamp: time.Now().Unix(),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
}

func main() {
	port := "50051"
	if p := os.Getenv("GRPC_PORT"); p != "" {
		port = p
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "监听端口失败: %v\n", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	hellov1.RegisterHelloServiceServer(s, &helloServer{})

	// 注册反射服务，方便用 grpcurl 调试
	reflection.Register(s)

	fmt.Printf("gRPC 后端服务已启动，监听端口 %s\n", port)
	if err := s.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "服务启动失败: %v\n", err)
		os.Exit(1)
	}
}
