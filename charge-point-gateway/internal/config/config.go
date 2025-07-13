package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用程序配置结构
type Config struct {
	PodID      string           `mapstructure:"pod_id"`
	Server     ServerConfig     `mapstructure:"server"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Log        LogConfig        `mapstructure:"log"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	OCPP       OCPPConfig       `mapstructure:"ocpp"`
	Security   SecurityConfig   `mapstructure:"security"`
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

// MetricsConfig 监控指标配置
type MetricsConfig struct {
	Addr string `mapstructure:"addr"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	HealthCheckPort int  `mapstructure:"health_check_port"`
	PprofEnabled    bool `mapstructure:"pprof_enabled"`
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

// Load 加载配置
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetServerAddr 获取服务器地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetMetricsAddr 获取监控地址
func (c *Config) GetMetricsAddr() string {
	return c.Metrics.Addr
}

// GetHealthCheckAddr 获取健康检查地址
func (c *Config) GetHealthCheckAddr() string {
	return fmt.Sprintf(":%d", c.Monitoring.HealthCheckPort)
}
