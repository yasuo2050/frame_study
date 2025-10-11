// server/main.go
package main

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net"

	"google.golang.org/grpc"
	// 引入我们刚刚生成的 Go 代码包
	pb "grpc-learning/proto" // 注意：请替换成你自己的 Go Module 路径
)

// 1. 定义一个 struct，用来实现 .proto 文件中定义的 GreeterServer 接口
type server struct {
	// 必须嵌入这个类型，以保证向前兼容性
	pb.UnimplementedGreeterServer
}

// 2. 实现 SayHello 方法
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("收到了来自客户端的消息: %v", in.GetName())
	// 业务逻辑：返回一个拼接后的字符串
	return &pb.HelloReply{Message: "你好, " + in.GetName()}, nil
}

func main() {
	// 3. 监听一个 TCP 端口
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}
	log.Println("服务正在监听端口 :50051")

	// 4. 创建一个 gRPC 服务器实例
	s := grpc.NewServer(
		// --- 变化点：在这里注册我们的拦截器 ---
		grpc.UnaryInterceptor(authInterceptor), //grpc.UnaryInterceptor() 接收一个拦截器函数
	)

	// 5. 将我们的服务实现注册到 gRPC 服务器上
	pb.RegisterGreeterServer(s, &server{})

	// 6. 启动服务，它会阻塞在这里，直到程序被终止
	if err := s.Serve(lis); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}

// authInterceptor 一元服务拦截器
func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.Println("--- [拦截器]: 接待到新请求 ---")

	// 1. 从 context 中提取 metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		// 如果没有 metadata，就拒绝请求
		return nil, status.Errorf(codes.Unauthenticated, "请求中缺少凭证信息(metadata)")
	}

	// 2. 检查凭证是否有效
	var token, reqID string
	if len(md.Get("token")) > 0 {
		token = md.Get("token")[0]
	}
	// Metadata 的值是一个字符串切片，即使只有一个值也是切片。我们通常取第一个元素
	if len(md.Get("x-request-id")) > 0 {
		reqID = md.Get("x-request-id")[0]
	}
	log.Printf("收到了来自客户端的 Metadata - Token: %s, RequestID: %s", token, reqID)

	// 简单的验证逻辑：token 必须是 "my-secret-token-12345"
	if token != "my-secret-token-12345" {
		log.Printf("[拦截器]: 认证失败, 无效的 Token: %s", token)
		return nil, status.Errorf(codes.Unauthenticated, "无效的 Token")
	}

	log.Printf("[拦截器]: 认证成功! Token: %s", token)
	// 3. 认证通过，保安放行，调用 handler 让请求继续
	return handler(ctx, req)
}
