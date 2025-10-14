package main

import (
	"fmt"
	"time"
)

// ==================== 选项模式：理解 serverx 的配置哲学 ====================

// 问题：为什么 serverx 不这样做？
// ❌ 错误的配置方式（难以扩展）
type ServerConfigBad struct {
	EnableJWT     bool
	EnableLog     bool
	EnableMetrics bool
	EnableTrace   bool
	JWTSecret     string
	LogLevel      string
	// 新功能？-> 添加越来越多字段...
}

func NewServerBad(config ServerConfigBad) *ServerBad {
	// 参数验证变得复杂，返回值难以处理
	return &ServerBad{config: config}
}

// 为了演示错误方式，需要定义对应的ServerBad结构体
type ServerBad struct {
	config ServerConfigBad
}

// ✅ 正确的配置方式：选项模式

// 第一步：定义配置选项结构体
type ServerConfig struct {
	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	EnableLogging bool
	LogLevel      string
	Interceptors  []string // 模拟拦截器列表
	// Redis 配置
	redisConfig *RedisConfig // 改为指针，可以为空
}

// 第二步：定义选项类型（核心！）
type Option func(*ServerConfig)

// 第三步：创建各种 With... 函数
func WithJWTAuth(secret string) Option {
	return func(config *ServerConfig) {
		config.JWTSecret = secret
		config.AccessTTL = 24 * time.Hour // 默认值
		config.RefreshTTL = 7 * 24 * time.Hour
	}
}

func WithJWTAuthAdvanced(secret string, accessTTL, refreshTTL time.Duration) Option {
	return func(config *ServerConfig) {
		config.JWTSecret = secret
		config.AccessTTL = accessTTL
		config.RefreshTTL = refreshTTL
	}
}

func WithLogging(level string) Option {
	return func(config *ServerConfig) {
		config.EnableLogging = true
		config.LogLevel = level
	}
}

func WithInterceptors(interceptors ...string) Option {
	return func(config *ServerConfig) {
		config.Interceptors = append(config.Interceptors, interceptors...)
	}
}

// 第四步：实现接受选项的构造函数
type Server struct {
	config ServerConfig
}

func NewServer(options ...Option) *Server {
	// 创建默认配置
	defaultConfig := ServerConfig{
		AccessTTL:     24 * time.Hour,
		RefreshTTL:    7 * 24 * time.Hour,
		EnableLogging: false,
		LogLevel:      "info",
		Interceptors:  []string{},
	}

	// 应用所有选项
	config := &defaultConfig
	for _, opt := range options {
		opt(config)
	}

	// 验证配置
	if config.JWTSecret == "" && len(config.Interceptors) > 0 {
		fmt.Println("⚠️  警告：没有设置JWT密钥但启用了需要JWT的拦截器")
	}

	return &Server{config: *config}
}

func (s *Server) PrintConfig() {
	fmt.Printf("📋 服务器配置:\n")
	fmt.Printf("   JWT密钥: %s\n", maskSecret(s.config.JWTSecret))
	fmt.Printf("   访问令牌TTL: %v\n", s.config.AccessTTL)
	fmt.Printf("   刷新令牌TTL: %v\n", s.config.RefreshTTL)
	fmt.Printf("   启用日志: %v\n", s.config.EnableLogging)
	fmt.Printf("   日志级别: %s\n", s.config.LogLevel)
	fmt.Printf("   拦截器: %v\n", s.config.Interceptors)

	// 安全打印Redis配置
	if redisConfig := s.config.GetRedis(); redisConfig != nil {
		fmt.Printf("   Redis地址: %s\n", redisConfig.Address)
		fmt.Printf("   Redis密码: %s\n", maskSecret(redisConfig.Password))
		fmt.Printf("   Redis数据库: %d\n", redisConfig.DB)
	} else {
		fmt.Printf("   Redis配置: 未启用\n")
	}
	fmt.Println()
}

func maskSecret(secret string) string {
	if secret == "" {
		return "未设置"
	}
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

// ==================== 扩展性演示：添加新功能 ====================

// 假设我们要添加一个Redis配置（新功能）
type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

// 扩展现有配置
func (c *ServerConfig) SetRedis(redisConfig RedisConfig) {
	c.redisConfig = &redisConfig
}

func (c *ServerConfig) GetRedis() *RedisConfig {
	return c.redisConfig
}

// 添加Redis选项，完全不影响现有代码！
func WithRedis(address string, password string, db int) Option {
	return func(config *ServerConfig) {
		config.SetRedis(RedisConfig{
			Address:  address,
			Password: password,
			DB:       db,
		})
	}
}

// ==================== 实战对比 ====================

func main() {
	fmt.Println("=== 🛠️  选项模式实战：理解 serverx 的配置哲学 ===\n")

	// 场景1：最简单的服务器
	fmt.Println("1️⃣ 场景1：最简单的服务器")
	server1 := NewServer()
	server1.PrintConfig()

	// 场景2：启用JWT认证
	fmt.Println("2️⃣ 场景2：启用JWT认证")
	server2 := NewServer(
		WithJWTAuth("my-super-secret-key-123456"),
	)
	server2.PrintConfig()

	// 场景3：完整的微服务配置
	fmt.Println("3️⃣ 场景3：完整的微服务配置")
	server3 := NewServer(
		WithJWTAuthAdvanced("prod-secret-key", 2*time.Hour, 24*time.Hour),
		WithLogging("debug"),
		WithInterceptors("auth", "logging", "rate-limit", "metrics"),
		WithRedis("localhost:6379", "", 0), // 新功能！
	)
	server3.PrintConfig()

	// 场景4：展示扩展性 - 动态配置
	fmt.Println("4️⃣ 场景4：运行时动态配置")
	options := []Option{}

	// 根据环境动态添加配置
	env := "production" // 可以从环境变量读取
	if env == "production" {
		options = append(options,
			WithJWTAuth("prod-secret"),
			WithLogging("error"),
			WithRedis("redis.prod.com:6379", "strong-password", 1),
		)
	} else {
		options = append(options,
			WithJWTAuth("dev-secret"),
			WithLogging("debug"),
			WithRedis("localhost:6379", "", 0),
		)
	}

	options = append(options,
		WithInterceptors("auth", "logging", "metrics"),
	)

	server4 := NewServer(options...)
	server4.PrintConfig()

	fmt.Println("=== 💡 选项模式的核心优势 ===")
	fmt.Println("✅ 灵活性：按需组合，不想用的功能不配置")
	fmt.Println("✅ 可扩展性：新增功能不影响现有代码")
	fmt.Println("✅ 可读性：With...函数名清晰表达意图")
	fmt.Println("✅ 默认值：提供合理的默认配置")
	fmt.Println("✅ 验证：在构造时统一验证配置")

	fmt.Println("\n=== 🤔 对比其他框架的配置方式 ===")
	fmt.Println("Go-Zero: 依赖配置文件 (config.yaml)")
	fmt.Println("Kratos:  依赖注入模式")
	fmt.Println("ServerX: 选项模式 (函数式)")
	fmt.Println("各有优劣，选项模式最适合Go的简洁哲学！")
}
