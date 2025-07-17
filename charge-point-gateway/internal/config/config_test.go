package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		cleanup  func()
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "load default config",
			setup: func() {
				viper.Reset()
				setTestDefaults()
			},
			cleanup: func() {
				viper.Reset()
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "0.0.0.0", cfg.Server.Host)
				assert.Equal(t, 8080, cfg.Server.Port)
				assert.Equal(t, "/ocpp", cfg.Server.WebSocketPath)
				assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
				assert.Equal(t, []string{"localhost:9092"}, cfg.Kafka.Brokers)
			},
		},
		{
			name: "load config with environment variables",
			setup: func() {
				viper.Reset()
				setTestDefaults()
				os.Setenv("GATEWAY_SERVER_PORT", "9090")
				os.Setenv("GATEWAY_REDIS_ADDR", "redis:6379")
				viper.SetEnvPrefix("GATEWAY")
				viper.AutomaticEnv()
				// 绑定环境变量
				viper.BindEnv("server.port", "GATEWAY_SERVER_PORT")
				viper.BindEnv("redis.addr", "GATEWAY_REDIS_ADDR")
			},
			cleanup: func() {
				os.Unsetenv("GATEWAY_SERVER_PORT")
				os.Unsetenv("GATEWAY_REDIS_ADDR")
				viper.Reset()
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 9090, cfg.Server.Port)
				assert.Equal(t, "redis:6379", cfg.Redis.Addr)
			},
		},
		{
			name: "load config with custom values",
			setup: func() {
				viper.Reset()
				setTestDefaults()
				viper.Set("server.host", "127.0.0.1")
				viper.Set("server.port", 8888)
				viper.Set("cache.max_size", 5000)
				viper.Set("ocpp.heartbeat_interval", "600s")
			},
			cleanup: func() {
				viper.Reset()
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "127.0.0.1", cfg.Server.Host)
				assert.Equal(t, 8888, cfg.Server.Port)
				assert.Equal(t, 5000, cfg.Cache.MaxSize)
				assert.Equal(t, 600*time.Second, cfg.OCPP.HeartbeatInterval)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			cfg, err := Load()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestConfig_GetServerAddr(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	addr := cfg.GetServerAddr()
	assert.Equal(t, "localhost:8080", addr)
}

func TestConfig_GetMetricsAddr(t *testing.T) {
	cfg := &Config{
		Monitoring: MonitoringConfig{
			MetricsAddr: ":9090",
		},
	}

	addr := cfg.GetMetricsAddr()
	assert.Equal(t, ":9090", addr)
}

func TestConfig_GetHealthCheckAddr(t *testing.T) {
	cfg := &Config{
		Monitoring: MonitoringConfig{
			HealthCheckPort: 8081,
		},
	}

	addr := cfg.GetHealthCheckAddr()
	assert.Equal(t, ":8081", addr)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		validate func(*testing.T, *Config)
	}{
		{
			name: "validate server config",
			setup: func() {
				viper.Reset()
				setTestDefaults()
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.NotEmpty(t, cfg.Server.Host)
				assert.Greater(t, cfg.Server.Port, 0)
				assert.NotEmpty(t, cfg.Server.WebSocketPath)
				assert.Greater(t, cfg.Server.MaxConnections, 0)
			},
		},
		{
			name: "validate redis config",
			setup: func() {
				viper.Reset()
				setTestDefaults()
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.NotEmpty(t, cfg.Redis.Addr)
				assert.GreaterOrEqual(t, cfg.Redis.DB, 0)
				assert.Greater(t, cfg.Redis.PoolSize, 0)
			},
		},
		{
			name: "validate kafka config",
			setup: func() {
				viper.Reset()
				setTestDefaults()
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.NotEmpty(t, cfg.Kafka.Brokers)
				assert.NotEmpty(t, cfg.Kafka.UpstreamTopic)
				assert.NotEmpty(t, cfg.Kafka.DownstreamTopic)
				assert.NotEmpty(t, cfg.Kafka.ConsumerGroup)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer viper.Reset()

			cfg, err := Load()
			require.NoError(t, err)
			tt.validate(t, cfg)
		})
	}
}

// setTestDefaults 设置测试用的默认配置
func setTestDefaults() {
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
	viper.SetDefault("monitoring.metrics_port", 9090)
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
