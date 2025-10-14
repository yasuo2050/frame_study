package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"time"
)

// ==================== 第一步：理解拦截器设计模式 ====================

// 业务处理函数类型
type Handler func(ctx context.Context, req interface{}) (interface{}, error)

// 拦截器类型
type Interceptor func(ctx context.Context, req interface{}, handler Handler) (interface{}, error)

// ==================== 第二步：实现各种中间件 ====================

// 1. 认证中间件
func AuthInterceptor() Interceptor {
	return func(ctx context.Context, req interface{}, handler Handler) (interface{}, error) {
		fmt.Println("🔐 [Auth] 开始验证...")

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "缺少认证信息")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "缺少authorization头")
		}

		token := tokens[0]
		if token != "valid-token-123" {
			return nil, status.Errorf(codes.Unauthenticated, "无效的Token")
		}

		fmt.Println("✅ [Auth] 认证通过")
		return handler(ctx, req)
	}
}

// 2. 日志中间件
func LoggingInterceptor() Interceptor {
	return func(ctx context.Context, req interface{}, handler Handler) (interface{}, error) {
		start := time.Now()
		fmt.Printf("📝 [Log] 请求开始: %T\n", req)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		fmt.Printf("📝 [Log] 请求完成，耗时: %v，错误: %v\n", duration, err)

		return resp, err
	}
}

// 3. 限流中间件
func RateLimitInterceptor() Interceptor {
	return func(ctx context.Context, req interface{}, handler Handler) (interface{}, error) {
		fmt.Println("🚦 [RateLimit] 开始限流检查...")

		// 模拟限流逻辑
		time.Sleep(10 * time.Millisecond)

		fmt.Println("✅ [RateLimit] 限流检查通过")
		return handler(ctx, req)
	}
}

// ==================== 第三步：实现拦截器链 ====================

// Chain 创建拦截器链（关键理解点！）
func Chain(interceptors ...Interceptor) Interceptor {
	return func(ctx context.Context, req interface{}, handler Handler) (interface{}, error) {
		// 递归调用，形成责任链
		return func(currentHandler Handler) Handler {
			for i := len(interceptors) - 1; i >= 0; i-- {
				currentHandler = func(h Handler) Handler {
					return func(ctx context.Context, req interface{}) (interface{}, error) {
						return interceptors[i](ctx, req, h)
					}
				}(currentHandler)
			}
			return currentHandler
		}(handler)(ctx, req)
	}
}

// ==================== 第四步：框架核心（模拟 grpc.NewServer） ====================

type MiniServer struct {
	interceptors []Interceptor
	handlers     map[string]Handler
}

func NewMiniServer() *MiniServer {
	return &MiniServer{
		handlers: make(map[string]Handler),
	}
}

// 添加拦截器（类似 grpc.UnaryInterceptor）
func (s *MiniServer) Use(interceptors ...Interceptor) {
	s.interceptors = append(s.interceptors, interceptors...)
}

// 注册处理器
func (s *MiniServer) Handle(name string, handler Handler) {
	s.handlers[name] = handler
}

// 执行请求（模拟真实的 gRPC 调用）
func (s *MiniServer) Call(ctx context.Context, method string, req interface{}) (interface{}, error) {
	handler, ok := s.handlers[method]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "方法不存在: %s", method)
	}

	// 应用拦截器链
	chain := Chain(s.interceptors...)
	return chain(ctx, req, handler)
}

// ==================== 第五步：业务代码（现在变得很干净） ====================

// SayHello 业务逻辑 - 不需要关心认证、日志、限流！
func SayHello业务逻辑(ctx context.Context, req interface{}) (interface{}, error) {
	fmt.Println("💼 [Business] 执行业务逻辑...")
	return map[string]string{"message": "Hello, World!"}, nil
}

// CreateUser 业务逻辑 - 同样很干净！
func CreateUser业务逻辑(ctx context.Context, req interface{}) (interface{}, error) {
	fmt.Println("💼 [Business] 创建用户业务逻辑...")
	return map[string]string{"user_id": "12345"}, nil
}

// ==================== 第六步：对比框架 vs 原始方法 ====================

// ❌ 原始方法（业务开发者常犯的错误）
type 原始Server struct{}

func (s *原始Server) SayHello(ctx context.Context, req interface{}) (interface{}, error) {
	// 🔗 每个方法都要重复这些代码！
	// 认证检查
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "缺少认证信息")
	}

	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "缺少authorization头")
	}

	if token := tokens[0]; token != "valid-token-123" {
		return nil, status.Errorf(codes.Unauthenticated, "无效Token")
	}

	// 日志记录
	fmt.Printf("请求: %+v\n", req)

	// 限流检查
	time.Sleep(10 * time.Millisecond)

	// 真正的业务逻辑
	fmt.Println("执行业务...")
	return map[string]string{"message": "Hello, World!"}, nil
}

func main() {
	fmt.Println("=== 🚀 欢迎来到框架开发的世界 ===\n")

	// ========== 使用框架版本 ==========
	fmt.Println("✨ 框架版本（推荐）：")
	server := NewMiniServer()

	// 声明式配置中间件
	server.Use(
		LoggingInterceptor(),
		AuthInterceptor(),
		RateLimitInterceptor(),
	)

	// 注册业务逻辑
	server.Handle("SayHello", SayHello业务逻辑)
	server.Handle("CreateUser", CreateUser业务逻辑)

	// 模拟请求（带认证信息）
	// 注意：这里使用 NewIncomingContext，因为我们在模拟服务端接收请求
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.New(map[string]string{"authorization": "valid-token-123"}))

	// 调用服务
	resp, err := server.Call(ctx, "SayHello", map[string]string{"name": "张三"})
	if err != nil {
		log.Printf("请求失败: %v", err)
	} else {
		fmt.Printf("响应: %+v\n\n", resp)
	}

	// ========== 对比原始版本 ==========
	fmt.Println("❌ 原始版本（不推荐）：")
	original := &原始Server{}
	ctx2 := metadata.NewIncomingContext(context.Background(),
		metadata.New(map[string]string{"authorization": "valid-token-123"}))

	resp2, err2 := original.SayHello(ctx2, map[string]string{"name": "张三"})
	if err2 != nil {
		log.Printf("请求失败: %v", err2)
	} else {
		fmt.Printf("响应: %+v\n", resp2)
	}

	fmt.Println("\n=== 💡 核心思想总结 ===")
	fmt.Println("1. 🎯 关注点分离：业务逻辑与基础设施分离")
	fmt.Println("2. 🔧 可组合性：中间件可以任意组合")
	fmt.Println("3. 📈 可维护性：修改中间件不影响业务代码")
	fmt.Println("4. 🚀 高复用性：中间件可以被多个服务复用")
	fmt.Println("\n这就是为什么需要 serverx.NewServer 的真正原因！")
}
