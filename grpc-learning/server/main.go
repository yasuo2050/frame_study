// server/main.go
package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net/http"
	"strings"

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

//func main() {
//	// 3. 监听一个 TCP 端口
//	lis, err := net.Listen("tcp", ":50051")
//	if err != nil {
//		log.Fatalf("监听端口失败: %v", err)
//	}
//	log.Println("服务正在监听端口 :50051")
//
//	// 4. 创建一个 gRPC 服务器实例
//	s := grpc.NewServer(
//		// --- 变化点：在这里注册我们的拦截器 ---
//		grpc.UnaryInterceptor(authInterceptor), //grpc.UnaryInterceptor() 接收一个拦截器函数
//	)
//
//	// 5. 将我们的服务实现注册到 gRPC 服务器上
//	pb.RegisterGreeterServer(s, &server{})
//
//	// 6. 启动服务，它会阻塞在这里，直到程序被终止
//	if err := s.Serve(lis); err != nil {
//		log.Fatalf("启动服务失败: %v", err)
//	}
//}

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

func main() {
	// --- 1. 启动 gRPC 服务 (和之前类似，但不立即 Serve) ---
	address := "0.0.0.0:50051"
	//grpcServer := grpc.NewServer(grpc.UnaryInterceptor(authInterceptor))
	grpcServer := grpc.NewServer()
	pb.RegisterGreeterServer(grpcServer, &server{})
	log.Println("gRPC 服务已准备...")

	// --- 2. 启动 HTTP Gateway 服务 ---
	ctx := context.Background()
	gwmux := runtime.NewServeMux() // 创建 gateway 的 Mux
	// 从 gRPC endpoint 注册 Greeter 服务的 HTTP handler
	err := pb.RegisterGreeterHandlerFromEndpoint(
		ctx,
		gwmux,
		address, // gRPC 服务的地址
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		log.Fatalf("注册 HTTP Gateway 失败: %v", err)
	}
	log.Println("HTTP Gateway 已准备...")

	// --- 3. 将 gRPC 和 HTTP 服务合并到同一个端口 ---
	// 这是实现端口复用的关键：我们创建一个顶层的 Handler
	// 它会根据请求的类型，决定把请求交给 gRPC Server 还是 HTTP Gateway
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果请求的 Content-Type 是 application/grpc，说明是 gRPC 请求
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			// 否则，就是普通的 HTTP 请求，交给 Gateway 处理
			gwmux.ServeHTTP(w, r)
		}
	})

	log.Printf("服务启动在 %s (同时支持 gRPC 和 HTTP)", address)
	// 使用 h2c 来包裹我们的 handler，使其能同时处理 HTTP/1.1 和 HTTP/2
	err = http.ListenAndServe(address, h2c.NewHandler(handler, &http2.Server{}))
	if err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
