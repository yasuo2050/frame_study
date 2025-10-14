package main

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// ==================== 模拟完整版 serverx ====================
// 这个案例将理解：
// 1. 为什么需要 serverx
// 2. serverx 解决了什么问题
// 3. 如何设计一个框架

// 第一步：定义服务器结构体
type ServerX struct {
	// 配置相关
	address       string
	protobufFiles []string

	// 注册的函数
	grpcRegisters []func(*grpc.Server)
	httpRegisters []func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

	// 拦截器
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor

	// 模块
	jwtModule    *JWTModule
	loggerModule *LoggerModule
}

// 选项类型
type ServerOption func(*ServerX)

// 第二步：实现各种选项函数（这就是 serverx 的核心API）

// WithGrpcRegisters - 注册 gRPC 服务
func WithGrpcRegisters(registers ...func(*grpc.Server)) ServerOption {
	return func(s *ServerX) {
		s.grpcRegisters = append(s.grpcRegisters, registers...)
	}
}

// WithHttpRegisters - 注册 HTTP 服务
func WithHttpRegisters(registers ...func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error) ServerOption {
	return func(s *ServerX) {
		s.httpRegisters = append(s.httpRegisters, registers...)
	}
}

// WithUnaryInterceptors - 添加一元拦截器
func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServerOption {
	return func(s *ServerX) {
		s.unaryInterceptors = append(s.unaryInterceptors, interceptors...)
	}
}

// WithJWTAuth - 启用 JWT 认证（关键理解点！）
func WithJWTAuth(secret string) ServerOption {
	return func(s *ServerX) {
		s.jwtModule = &JWTModule{
			enabled: true,
			secret:  secret,
		}
		// 自动将JWT拦截器添加到拦截器链
		s.unaryInterceptors = append(s.unaryInterceptors, s.jwtModule.Interceptor())
	}
}

// WithLogging - 启用日志
func WithLogging(level string) ServerOption {
	return func(s *ServerX) {
		s.loggerModule = &LoggerModule{
			enabled: true,
			level:   level,
		}
		s.unaryInterceptors = append(s.unaryInterceptors, s.loggerModule.Interceptor())
	}
}

// 第三步：实现各种模块

// JWT 模块
type JWTModule struct {
	enabled bool
	secret  string
}

func (j *JWTModule) Interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !j.enabled {
			return handler(ctx, req)
		}

		fmt.Printf("🔐 [JWT] 验证请求: %s\n", info.FullMethod)

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, fmt.Errorf("缺少认证信息")
		}

		token := md.Get("authorization")[0]
		if token != "Bearer "+j.secret {
			return nil, fmt.Errorf("无效的Token")
		}

		return handler(ctx, req)
	}
}

// 日志模块
type LoggerModule struct {
	enabled bool
	level   string
}

func (l *LoggerModule) Interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !l.enabled {
			return handler(ctx, req)
		}

		start := time.Now()
		fmt.Printf("📝 [Logger] 开始执行: %s (级别: %s)\n", info.FullMethod, l.level)

		resp, err = handler(ctx, req)

		duration := time.Since(start)
		fmt.Printf("📝 [Logger] 完成执行: %s, 耗时: %v\n", info.FullMethod, duration)

		return resp, err
	}
}

// 第四步：实现构造函数
func NewServerX(options ...ServerOption) *ServerX {
	// 创建默认服务器
	server := &ServerX{
		address:            "0.0.0.0:8080",
		unaryInterceptors:  []grpc.UnaryServerInterceptor{},
		streamInterceptors: []grpc.StreamServerInterceptor{},
	}

	// 应用所有选项
	for _, opt := range options {
		opt(server)
	}

	return server
}

// 第五步：实现核心运行逻辑（这是最复杂的部分）
func (s *ServerX) Run() error {
	// 1. 创建 gRPC 服务器（带拦截器）
	var grpcOpts []grpc.ServerOption
	if len(s.unaryInterceptors) > 0 {
		grpcOpts = append(grpcOpts, grpc.ChainUnaryInterceptor(s.unaryInterceptors...))
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// 2. 注册 gRPC 服务
	for _, register := range s.grpcRegisters {
		register(grpcServer)
	}

	// 3. 创建 HTTP Gateway
	gwmux := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// 4. 注册 HTTP 服务
	for _, register := range s.httpRegisters {
		if err := register(context.Background(), gwmux, s.address, dialOpts); err != nil {
			return fmt.Errorf("注册HTTP服务失败: %v", err)
		}
	}

	// 5. 创建双协议处理器（关键！）
	handler := s.createDualProtocolHandler(grpcServer, gwmux)

	// 6. 启动服务器
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("监听端口失败: %v", err)
	}

	log.Printf("🚀 ServerX 启动成功！双协议监听: %s", s.address)
	log.Printf("   ✅ gRPC 服务: grpc://%s", s.address)
	log.Printf("   ✅ HTTP 服务: http://%s", s.address)

	return http.Serve(lis, handler)
}

// 双协议处理器 - 这是 serverx 的核心魔法
func (s *ServerX) createDualProtocolHandler(grpcServer *grpc.Server, gwMux *runtime.ServeMux) http.Handler {
	return h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 判断请求类型
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			// gRPC 请求
			fmt.Printf("🔥 路由到 gRPC 服务: %s %s\n", r.Method, r.URL.Path)
			grpcServer.ServeHTTP(w, r)
		} else {
			// HTTP 请求
			fmt.Printf("🌐 路由到 HTTP Gateway: %s %s\n", r.Method, r.URL.Path)
			gwMux.ServeHTTP(w, r)
		}
	}), &http2.Server{})
}

// ==================== 模拟 PB 代码 ====================

// 模拟生成的 gRPC 服务接口
type GreeterServer interface {
	SayHello(context.Context, *HelloRequest) (*HelloReply, error)
}

// 模拟请求和响应结构
type HelloRequest struct {
	Name string
}

type HelloReply struct {
	Message string
}

// 模拟服务实现
type greeterServer struct{}

func (g *greeterServer) SayHello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
	fmt.Printf("💼 [业务] 收到请求: %+v\n", req)
	return &HelloReply{Message: "你好, " + req.Name}, nil
}

// ==================== 演示对比 ====================

// 方式1：原生模式（复杂）
func originalImplementation() {
	fmt.Println("\n=== 原生实现方式（复杂）===")
	// 这里需要手动处理所有 serverx 帮你做的事情：
	// 1. 创建 gRPC server
	// 2. 创建 HTTP Gateway
	// 3. 设置 h2c 处理器
	// 4. 注册拦截器
	// 5. 监听端口
	// ... 大量重复代码
	fmt.Println("❌ 需要写 100+ 行重复代码")
}

// 方式2：ServerX 模式（简洁）
func serverXImplementation() {
	fmt.Println("\n=== ServerX 实现方式（简洁）===")

	// 使用我们的 ServerX
	NewServerX(
		WithGrpcRegisters(func(gs *grpc.Server) {
			// 注册服务（框架会自动处理）
			fmt.Println("✅ 注册 Greeter 服务")
		}),
		WithHttpRegisters(func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
			// 注册 HTTP 处理器
			fmt.Println("✅ 注册 HTTP 处理器")
			return nil
		}),
		WithJWTAuth("my-secret-key"),
		WithLogging("debug"),
	)

	fmt.Printf("✅ 只需要 10 行代码就完成了完整的服务器配置！\n")
	fmt.Printf("✅ 包含了：双协议 + JWT认证 + 日志 + 拦截器链\n")

	// server.Run() // 实际启动（这里演示，不真正运行）
}

func main() {
	fmt.Println("=== 🎯 ServerX 框架设计原理演示 ===")

	// 展示两种方式的对比
	originalImplementation()
	serverXImplementation()

	fmt.Println("\n=== 💡 理解框架开发的核心思想 ===")
	fmt.Println("1. 📦 封装复杂性：将复杂的基础设施代码封装起来")
	fmt.Println("2. 🎭 约定优于配置：提供合理的默认值和最佳实践")
	fmt.Println("3. 🔧 组合优于继承：通过选项模式实现灵活的功能组合")
	fmt.Println("4. 🎪 关注点分离：业务代码与基础设施代码分离")
	fmt.Println("5. 🚀 提升开发效率：让开发者专注于业务逻辑")

	fmt.Println("\n=== 📚 从业务开发者到框架开发者的进阶之路 ===")
	fmt.Println("✅ 第一阶段：理解 gRPC 原理（已完成）")
	fmt.Println("✅ 第二阶段：理解拦截器设计模式（通过 mini-framework）")
	fmt.Println("✅ 第三阶段：理解选项模式配置（通过 options-demo）")
	fmt.Println("✅ 第四阶段：理解整体框架设计（通过 serverx-simplified）")
	fmt.Println("🎯 下一步：阅读真实的 serverx 源码，理解生产级实现")
}
