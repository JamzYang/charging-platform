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
	App        AppConfig        `mapstructure:"app"`
	PodID      string           `mapstructure:"pod_id"`
	Server     ServerConfig     `mapstructure:"server"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Log        LogConfig        `mapstructure:"log"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	OCPP       OCPPConfig       `mapstructure:"ocpp"`
	Security   SecurityConfig   `mapstructure:"security"`
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

	// 5. 兼容旧的config.yaml文件
	if err := loadConfigFile("config"); err != nil {
		fmt.Printf("Info: Legacy config.yaml not found (this is normal): %v\n", err)
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
	return "dev" // 默认开发环境
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
	fmt.Printf("Profile: %s\n", cfg.App.Profile)
	fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Redis: %s\n", cfg.Redis.Addr)
	fmt.Printf("Kafka: %v\n", cfg.Kafka.Brokers)
	fmt.Printf("Log Level: %s\n", cfg.Log.Level)
	fmt.Printf("============================\n")
}

// setDefaults 设置默认配置
func setDefaults() {
	// 应用信息
	viper.SetDefault("app.name", "charge-point-gateway")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.profile", "dev")

	// 服务器配置
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.websocket_path", "/ocpp")
	viper.SetDefault("server.read_timeout", "60s")
	viper.SetDefault("server.write_timeout", "60s")
	viper.SetDefault("server.max_connections", 100000)

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

	// 监控配置
	viper.SetDefault("monitoring.metrics_addr", ":9090")
	viper.SetDefault("monitoring.health_check_port", 8081)
	viper.SetDefault("monitoring.pprof_enabled", false)

	// OCPP配置
	viper.SetDefault("ocpp.supported_versions", []string{"1.6"})
	viper.SetDefault("ocpp.heartbeat_interval", "300s")
	viper.SetDefault("ocpp.connection_timeout", "60s")
	viper.SetDefault("ocpp.message_timeout", "30s")

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
