package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type ServerConfig struct {
	Port        string `mapstructure:"port"`
	GRPCPort    string `mapstructure:"grpc_port"`
	StoragePath string `mapstructure:"storage_path"`
	MaxSize     int64  `mapstructure:"max_file_size"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RabbitMQConfig struct {
	Host      string `mapstructure:"host"`
	Port      string `mapstructure:"port"`
	User      string `mapstructure:"user"`
	Password  string `mapstructure:"password"`
	QueueName string `mapstructure:"queue_name"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// 定义全局变量 (直接定义为值类型，防止空指针 panic)
var GlobalConfig Config

func LoadConfig() {
	// 默认配置
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.grpc_port", "9090") // 默认 gRPC 端口
	viper.SetDefault("server.storage_path", "./uploads")
	viper.SetDefault("server.max_file_size", 104857600)

	// 配置文件设置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config") // 优先找 ./config/config.yaml
	viper.AddConfigPath(".")        // 其次找 ./config.yaml

	// 环境变量设置
	viper.SetEnvPrefix("GCC")                              // 改成 GCC (Go Cloud Compute) 避免和 Storage 冲突
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // server.port -> GCC_SERVER_PORT
	viper.AutomaticEnv()

	// 读取配置
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Warning: Config file not found, using default values or environment variables.")
		} else {
			log.Fatalf("Error reading config file: %s", err)
		}
	}

	// 解析到全局变量
	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	log.Println("Config loaded successfully")
}
