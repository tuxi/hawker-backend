package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Volcengine VolcengineConfig `mapstructure:"volcengine"`
}

type ServerConfig struct {
	Port      int    `mapstructure:"port"`
	StaticDir string `mapstructure:"static_dir"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Dbname   string `mapstructure:"dbname"`
	SslMode  string `mapstructure:"sslmode"`
}

type VolcengineConfig struct {
	AppID       string `mapstructure:"app_id"`
	AccessToken string `mapstructure:"access_token"`
	ClusterID   string `mapstructure:"cluster_id"`
	VoiceType   string `mapstructure:"voice_type"`
}

// LoadConfig 解析配置文件
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &cfg, nil
}
