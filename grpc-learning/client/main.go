// client/main.go
package main

import (
	"context"
	"google.golang.org/grpc/metadata"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	// 同样引入生成的 Go 代码包
	pb "grpc-learning/proto" // 注意：请替换成你自己的 Go Module 路径
)

func main() {
	// 1. 连接到服务器地址
	// grpc.WithTransportCredentials(insecure.NewCredentials()) 表示使用不安全的连接，学习时使用，生产环境需要证书
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	// defer 保证在函数结束时关闭连接
	defer conn.Close()

	// 2. 创建一个 Greeter 服务的客户端 "存根" (Stub)
	c := pb.NewGreeterClient(conn)

	// 3. 设置一个带超时的 context (好习惯)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// --- 变化点 2: 创建并附加 Metadata ---
	// 1. 创建一个 metadata.MD 对象，它本质上是 map[string][]string
	md := metadata.New(map[string]string{
		"token":        "my-secret-token-12345",
		"x-request-id": "uuid-abc-123-xyz",
	})
	// 2. 使用 metadata.NewOutgoingContext 将 md 附加到 context 中
	//    这会创建一个新的 context，其中包含了要发送的元数据
	mdCtx := metadata.NewOutgoingContext(ctx, md)

	// 4. 调用 SayHello 方法，就像调用一个本地函数一样
	log.Println("正在向服务器发送请求...")
	r, err := c.SayHello(mdCtx, &pb.HelloRequest{Name: "Gemini"})
	if err != nil {
		log.Fatalf("调用 SayHello 失败: %v", err)
	}

	// 5. 打印服务器返回的结果
	log.Printf("从服务器收到的响应: %s", r.GetMessage())
}
