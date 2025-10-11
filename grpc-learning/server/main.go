// server/main.go
package main

import (
	"context"
	"google.golang.org/grpc/metadata"
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
	
	// --- 变化点 2: 从传入的 context 中提取 Metadata ---
	// 1. 使用 metadata.FromIncomingContext 来获取元数据
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("没有找到 metadata")
	} else {
		// 2. Metadata 的值是一个字符串切片，即使只有一个值也是切片
		//    我们通常取第一个元素
		var token, reqID string
		if len(md.Get("token")) > 0 {
			token = md.Get("token")[0]
		}
		if len(md.Get("x-request-id")) > 0 {
			reqID = md.Get("x-request-id")[0]
		}
		log.Printf("收到了来自客户端的 Metadata - Token: %s, RequestID: %s", token, reqID)
	}

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
	s := grpc.NewServer()

	// 5. 将我们的服务实现注册到 gRPC 服务器上
	pb.RegisterGreeterServer(s, &server{})

	// 6. 启动服务，它会阻塞在这里，直到程序被终止
	if err := s.Serve(lis); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
