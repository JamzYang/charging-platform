package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用程序配置结构
type Config struct {
	App           AppConfig          `mapstructure:"app"`
	PodID         string             `mapstructure:"pod_id"`
	Server        ServerConfig       `mapstructure:"server"`
	WebSocket     WebSocketConfig    `mapstructure:"websocket"`
	Redis         RedisConfig        `mapstructure:"redis"`
	Kafka         KafkaConfig        `mapstructure:"kafka"`
	Cache         CacheConfig        `mapstructure:"cache"`
	Log           LogConfig          `mapstructure:"log"`
	EventChannels EventChannelConfig `mapstructure:"event_channels"`
	Monitoring    MonitoringConfig   `mapstructure:"monitoring"`
	OCPP          OCPPConfig         `mapstructure:"ocpp"`
	Security      SecurityConfig     `mapstructure:"security"`
}

// AppConfig 应用程序基本信息
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Profile string `mapstructure:"profile"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	WebSocketPath  string        `mapstructure:"websocket_path"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxConnections int           `mapstructure:"max_connections"`
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	ReadBufferSize    int           `mapstructure:"read_buffer_size"`
	WriteBufferSize   int           `mapstructure:"write_buffer_size"`
	HandshakeTimeout  time.Duration `mapstructure:"handshake_timeout"`
	PingInterval      time.Duration `mapstructure:"ping_interval"`
	PongTimeout       time.Duration `mapstructure:"pong_timeout"`
	MaxMessageSize    int64         `mapstructure:"max_message_size"`
	EnableCompression bool          `mapstructure:"enable_compression"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
	CheckOrigin       bool          `mapstructure:"check_origin"`
	AllowedOrigins    []string      `mapstructure:"allowed_origins"`
	EnableSubprotocol bool          `mapstructure:"enable_subprotocol"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers         []string       `mapstructure:"brokers"`
	UpstreamTopic   string         `mapstructure:"upstream_topic"`
	DownstreamTopic string         `mapstructure:"downstream_topic"`
	ConsumerGroup   string         `mapstructure:"consumer_group"`
	PartitionNum    int            `mapstructure:"partition_num"`
	Producer        ProducerConfig `mapstructure:"producer"`
	Consumer        ConsumerConfig `mapstructure:"consumer"`
}

// ProducerConfig Kafka生产者配置
type ProducerConfig struct {
	RetryMax       int           `mapstructure:"retry_max"`
	ReturnSuccess  bool          `mapstructure:"return_successes"`
	FlushFrequency time.Duration `mapstructure:"flush_frequency"`
}

// ConsumerConfig Kafka消费者配置
type ConsumerConfig struct {
	ReturnErrors   bool   `mapstructure:"return_errors"`
	OffsetsInitial string `mapstructure:"offsets_initial"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize         int           `mapstructure:"max_size"`
	TTL             time.Duration `mapstructure:"ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	MemoryLimitMB   int           `mapstructure:"memory_limit_mb"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	Async  bool   `mapstructure:"async"`
}

// EventChannelConfig 事件通道配置 - 统一管理所有组件的事件通道容量
type EventChannelConfig struct {
	// 统一事件通道容量 - 所有组件使用相同容量，避免瓶颈
	BufferSize int `mapstructure:"buffer_size" json:"buffer_size"`
}

// DefaultEventChannelConfig 默认事件通道配置
func DefaultEventChannelConfig() EventChannelConfig {
	return EventChannelConfig{
		BufferSize: 50000, // 统一事件通道容量，支持高并发场景
	}
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	MetricsAddr     string `mapstructure:"metrics_addr"`
	HealthCheckPort int    `mapstructure:"health_check_port"`
	PprofEnabled    bool   `mapstructure:"pprof_enabled"`
}

// OCPPConfig OCPP协议配置
type OCPPConfig struct {
	SupportedVersions []string      `mapstructure:"supported_versions"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	MessageTimeout    time.Duration `mapstructure:"message_timeout"`
	WorkerCount       int           `mapstructure:"worker_count"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	TLSEnabled bool   `mapstructure:"tls_enabled"`
	CertFile   string `mapstructure:"cert_file"`
	KeyFile    string `mapstructure:"key_file"`
	ClientAuth bool   `mapstructure:"client_auth"`
}

// Load 加载配置 - Spring Boot风格：多环境配置
func Load() (*Config, error) {
	// 1. 设置默认值
	setDefaults()

	// 2. 确定运行环境
	profile := getProfile()
	fmt.Printf("Loading configuration for profile: %s\n", profile)

	// 3. 加载默认配置文件 application.yaml
	if err := loadConfigFile("application"); err != nil {
		fmt.Printf("Warning: Could not load default config file: %v\n", err)
	}

	// 4. 加载环境特定配置文件 application-{profile}.yaml
	if profile != "" {
		configName := fmt.Sprintf("application-%s", profile)
		if err := loadConfigFile(configName); err != nil {
			fmt.Printf("Warning: Could not load profile config file %s: %v\n", configName, err)
		}
	}

	// 6. 环境变量覆盖配置文件（最高优先级）
	setupEnvironmentVariables()

	// 7. 解析最终配置
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 8. 设置运行时环境信息
	cfg.App.Profile = profile

	// 9. 打印配置加载信息（调试用）
	printConfigInfo(&cfg)

	return &cfg, nil
}

// getProfile 获取运行环境配置
func getProfile() string {
	// 优先级：环境变量 > 配置文件默认值
	// 先检查环境变量
	if profile := os.Getenv("APP_PROFILE"); profile != "" {
		return profile
	}
	// 再检查viper中的配置
	if profile := viper.GetString("app.profile"); profile != "" {
		return profile
	}
	return "local" // 默认开发环境
}

// loadConfigFile 加载指定的配置文件
func loadConfigFile(configName string) error {
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	return viper.MergeInConfig()
}

// setupEnvironmentVariables 设置环境变量映射
func setupEnvironmentVariables() {
	// 启用自动环境变量读取
	viper.AutomaticEnv()

	// 设置键名替换规则：将配置中的点号替换为下划线
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 手动绑定关键环境变量（确保映射正确）
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("log.level", "LOG_LEVEL")
	viper.BindEnv("monitoring.health_check_port", "MONITORING_HEALTH_CHECK_PORT")
	viper.BindEnv("app.profile", "APP_PROFILE")

	// 特殊处理Kafka brokers数组
	if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); kafkaBrokers != "" {
		// 支持逗号分隔的多个broker地址
		brokers := strings.Split(kafkaBrokers, ",")
		for i, broker := range brokers {
			brokers[i] = strings.TrimSpace(broker)
		}
		viper.Set("kafka.brokers", brokers)
	}
}

// printConfigInfo 打印配置加载信息（调试用）
func printConfigInfo(cfg *Config) {
	fmt.Printf("=== Configuration Loaded ===\n")

	// 应用信息
	fmt.Printf("App:\n")
	fmt.Printf("  Name: %s\n", cfg.App.Name)
	fmt.Printf("  Version: %s\n", cfg.App.Version)
	fmt.Printf("  Profile: %s\n", cfg.App.Profile)
	fmt.Printf("  Pod ID: %s\n", cfg.PodID)

	// 服务器配置
	fmt.Printf("Server:\n")
	fmt.Printf("  Address: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("  WebSocket Path: %s\n", cfg.Server.WebSocketPath)
	fmt.Printf("  Read Timeout: %v\n", cfg.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %v\n", cfg.Server.WriteTimeout)
	fmt.Printf("  Max Connections: %d\n", cfg.Server.MaxConnections)

	// WebSocket配置
	fmt.Printf("WebSocket:\n")
	fmt.Printf("  Read Buffer Size: %d\n", cfg.WebSocket.ReadBufferSize)
	fmt.Printf("  Write Buffer Size: %d\n", cfg.WebSocket.WriteBufferSize)
	fmt.Printf("  Handshake Timeout: %v\n", cfg.WebSocket.HandshakeTimeout)
	fmt.Printf("  Ping Interval: %v\n", cfg.WebSocket.PingInterval)
	fmt.Printf("  Pong Timeout: %v\n", cfg.WebSocket.PongTimeout)
	fmt.Printf("  Max Message Size: %d\n", cfg.WebSocket.MaxMessageSize)
	fmt.Printf("  Enable Compression: %v\n", cfg.WebSocket.EnableCompression)
	fmt.Printf("  Idle Timeout: %v\n", cfg.WebSocket.IdleTimeout)
	fmt.Printf("  Cleanup Interval: %v\n", cfg.WebSocket.CleanupInterval)
	fmt.Printf("  Check Origin: %v\n", cfg.WebSocket.CheckOrigin)
	fmt.Printf("  Enable Subprotocol: %v\n", cfg.WebSocket.EnableSubprotocol)

	// Redis配置
	fmt.Printf("Redis:\n")
	fmt.Printf("  Address: %s\n", cfg.Redis.Addr)
	fmt.Printf("  Database: %d\n", cfg.Redis.DB)
	fmt.Printf("  Pool Size: %d\n", cfg.Redis.PoolSize)
	fmt.Printf("  Min Idle Conns: %d\n", cfg.Redis.MinIdleConns)
	fmt.Printf("  Dial Timeout: %v\n", cfg.Redis.DialTimeout)

	// Kafka配置
	fmt.Printf("Kafka:\n")
	fmt.Printf("  Brokers: %v\n", cfg.Kafka.Brokers)
	fmt.Printf("  Consumer Group: %s\n", cfg.Kafka.ConsumerGroup)
	fmt.Printf("  Upstream Topic: %s\n", cfg.Kafka.UpstreamTopic)
	fmt.Printf("  Downstream Topic: %s\n", cfg.Kafka.DownstreamTopic)
	fmt.Printf("  Partition Num: %d\n", cfg.Kafka.PartitionNum)
	fmt.Printf("  Producer Retry Max: %d\n", cfg.Kafka.Producer.RetryMax)
	fmt.Printf("  Producer Return Success: %v\n", cfg.Kafka.Producer.ReturnSuccess)
	fmt.Printf("  Producer Flush Frequency: %v\n", cfg.Kafka.Producer.FlushFrequency)

	// 缓存配置
	fmt.Printf("Cache:\n")
	fmt.Printf("  Max Size: %d\n", cfg.Cache.MaxSize)
	fmt.Printf("  TTL: %v\n", cfg.Cache.TTL)
	fmt.Printf("  Cleanup Interval: %v\n", cfg.Cache.CleanupInterval)
	fmt.Printf("  Memory Limit: %d MB\n", cfg.Cache.MemoryLimitMB)

	// 日志配置
	fmt.Printf("Log:\n")
	fmt.Printf("  Level: %s\n", cfg.Log.Level)
	fmt.Printf("  Format: %s\n", cfg.Log.Format)
	fmt.Printf("  Output: %s\n", cfg.Log.Output)
	fmt.Printf("  async: %s\n", cfg.Log.Async)

	// 监控配置
	fmt.Printf("Monitoring:\n")
	fmt.Printf("  Metrics Address: %s\n", cfg.Monitoring.MetricsAddr)
	fmt.Printf("  Health Check Port: %d\n", cfg.Monitoring.HealthCheckPort)
	fmt.Printf("  Pprof Enabled: %v\n", cfg.Monitoring.PprofEnabled)

	// OCPP配置
	fmt.Printf("OCPP:\n")
	fmt.Printf("  Supported Versions: %v\n", cfg.OCPP.SupportedVersions)
	fmt.Printf("  Heartbeat Interval: %v\n", cfg.OCPP.HeartbeatInterval)
	fmt.Printf("  Connection Timeout: %v\n", cfg.OCPP.ConnectionTimeout)
	fmt.Printf("  Message Timeout: %v\n", cfg.OCPP.MessageTimeout)
	fmt.Printf("  Worker Count: %d\n", cfg.OCPP.WorkerCount)

	// 安全配置
	fmt.Printf("Security:\n")
	fmt.Printf("  TLS Enabled: %v\n", cfg.Security.TLSEnabled)
	if cfg.Security.TLSEnabled {
		fmt.Printf("  Cert File: %s\n", cfg.Security.CertFile)
		fmt.Printf("  Key File: %s\n", cfg.Security.KeyFile)
		fmt.Printf("  Client Auth: %v\n", cfg.Security.ClientAuth)
	}

	fmt.Printf("============================\n")
}

// setDefaults 设置默认配置
func setDefaults() {
	// 应用信息
	viper.SetDefault("app.name", "charge-point-gateway")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.profile", "local")

	// 服务器配置
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.websocket_path", "/ocpp")
	viper.SetDefault("server.read_timeout", "60s")
	viper.SetDefault("server.write_timeout", "60s")
	viper.SetDefault("server.max_connections", 100000)

	// WebSocket配置
	viper.SetDefault("websocket.read_buffer_size", 4096)
	viper.SetDefault("websocket.write_buffer_size", 4096)
	viper.SetDefault("websocket.handshake_timeout", "10s")
	viper.SetDefault("websocket.ping_interval", "30s")
	viper.SetDefault("websocket.pong_timeout", "10s")
	viper.SetDefault("websocket.max_message_size", 1048576) // 1MB
	viper.SetDefault("websocket.enable_compression", false)
	viper.SetDefault("websocket.idle_timeout", "15m")
	viper.SetDefault("websocket.cleanup_interval", "10m")
	viper.SetDefault("websocket.check_origin", false)
	viper.SetDefault("websocket.allowed_origins", []string{})
	viper.SetDefault("websocket.enable_subprotocol", true)

	// Redis配置
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.min_idle_conns", 10)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")

	// Kafka配置
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.upstream_topic", "ocpp-events-up")
	viper.SetDefault("kafka.downstream_topic", "commands-down")
	viper.SetDefault("kafka.consumer_group", "gateway-consumer")
	viper.SetDefault("kafka.partition_num", 3) // 默认3个分区

	// 缓存配置
	viper.SetDefault("cache.max_size", 10000)
	viper.SetDefault("cache.ttl", "1h")
	viper.SetDefault("cache.cleanup_interval", "10m")
	viper.SetDefault("cache.memory_limit_mb", 512)

	// 日志配置
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "console")
	viper.SetDefault("log.output", "stdout")

	// 事件通道配置
	viper.SetDefault("event_channels.buffer_size", 50000)

	// 监控配置
	viper.SetDefault("monitoring.metrics_addr", ":9090")
	viper.SetDefault("monitoring.health_check_port", 8081)
	viper.SetDefault("monitoring.pprof_enabled", false)

	// OCPP配置
	viper.SetDefault("ocpp.supported_versions", []string{"1.6"})
	viper.SetDefault("ocpp.heartbeat_interval", "300s")
	viper.SetDefault("ocpp.connection_timeout", "60s")
	viper.SetDefault("ocpp.message_timeout", "30s")
	viper.SetDefault("ocpp.worker_count", 100) // 默认100个worker，支持高并发

	// 安全配置
	viper.SetDefault("security.tls_enabled", false)
	viper.SetDefault("security.cert_file", "")
	viper.SetDefault("security.key_file", "")
	viper.SetDefault("security.client_auth", false)
}

// GetServerAddr 获取服务器地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetMetricsAddr 获取监控地址
func (c *Config) GetMetricsAddr() string {
	return c.Monitoring.MetricsAddr
}

// GetHealthCheckAddr 获取健康检查地址
func (c *Config) GetHealthCheckAddr() string {
	return fmt.Sprintf(":%d", c.Monitoring.HealthCheckPort)
}

// IsProduction 判断是否为生产环境
func (c *Config) IsProduction() bool {
	return c.App.Profile == "prod"
}

// IsDevelopment 判断是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.App.Profile == "dev"
}

// IsTest 判断是否为测试环境
func (c *Config) IsTest() bool {
	return c.App.Profile == "test" || c.App.Profile == "local"
}
