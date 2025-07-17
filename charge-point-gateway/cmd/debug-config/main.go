package main

import (
	"fmt"
	"os"

	"github.com/charging-platform/charge-point-gateway/internal/config"
)

// 配置调试工具
// 用于验证和调试配置加载，支持多环境配置测试
func main() {
	fmt.Println("=== Charge Point Gateway Configuration Test ===")

	// 显示环境变量
	fmt.Println("\n--- Environment Variables ---")
	envVars := []string{
		"APP_PROFILE",
		"REDIS_ADDR",
		"KAFKA_BROKERS",
		"SERVER_PORT",
		"LOG_LEVEL",
		"MONITORING_HEALTH_CHECK_PORT",
	}

	for _, env := range envVars {
		value := os.Getenv(env)
		if value != "" {
			fmt.Printf("%s = %s\n", env, value)
		} else {
			fmt.Printf("%s = (not set)\n", env)
		}
	}

	// 加载配置
	fmt.Println("\n--- Loading Configuration ---")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// 显示最终配置
	fmt.Println("\n--- Final Configuration ---")
	fmt.Printf("App Name: %s\n", cfg.App.Name)
	fmt.Printf("App Version: %s\n", cfg.App.Version)
	fmt.Printf("App Profile: %s\n", cfg.App.Profile)
	fmt.Printf("Server Address: %s\n", cfg.GetServerAddr())
	fmt.Printf("Redis Address: %s\n", cfg.Redis.Addr)
	fmt.Printf("Kafka Brokers: %v\n", cfg.Kafka.Brokers)
	fmt.Printf("Log Level: %s\n", cfg.Log.Level)
	fmt.Printf("Metrics Address: %s\n", cfg.GetMetricsAddr())
	fmt.Printf("Health Check Address: %s\n", cfg.GetHealthCheckAddr())

	// 环境检查
	fmt.Println("\n--- Environment Check ---")
	fmt.Printf("Is Development: %v\n", cfg.IsDevelopment())
	fmt.Printf("Is Test: %v\n", cfg.IsTest())
	fmt.Printf("Is Production: %v\n", cfg.IsProduction())

	fmt.Println("\n=== Configuration Test Complete ===")
}
