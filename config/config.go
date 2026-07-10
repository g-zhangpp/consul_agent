package config

import (
	"flag"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	confPath string
	instance *Config
	once     sync.Once
)

func init() {
	flag.StringVar(&confPath, "confpath", "conf/config.yaml", "配置文件路径，默认为conf/config.yaml")
}

type LogConfig struct {
	LogFilePath string `yaml:"LogFilePath"`
	LogLevel    string `yaml:"LogLevel"`
}

type ConsulConfig struct {
	Token                                   string `yaml:"Token"`
	Address                                 string `yaml:"Address"`
	CheckTimeout                            string `yaml:"CheckTimeout"`
	CheckInterval                           string `yaml:"CheckInterval"`
	CheckDeregisterCriticalServiceAfter     bool   `yaml:"CheckDeregisterCriticalServiceAfter"`
	CheckDeregisterCriticalServiceAfterTime string `yaml:"CheckDeregisterCriticalServiceAfterTime"`
}

type SystemConfig struct {
	ServiceName   string `yaml:"ServiceName"`
	ListenAddress string `yaml:"ListenAddress"`
	Port          string `yaml:"Port"`
	FindAddress   string `yaml:"FindAddress"`
}

type ServiceConfig struct {
	Name    string            `yaml:"Name"`
	Port    string            `yaml:"Port"`
	Address string            `yaml:"Address"`
	Tags    []string          `yaml:"Tags"`
	Meta    map[string]string `yaml:"Meta"`
}

type Config struct {
	System   SystemConfig    `yaml:"System"`
	Logs     LogConfig       `yaml:"Logs"`
	Consul   ConsulConfig    `yaml:"Consul"`
	Services []ServiceConfig `yaml:"Services"`
}

// GetConf 返回单例配置
func GetConf() *Config {
	once.Do(func() {
		flag.Parse()
		data, err := os.ReadFile(confPath)
		if err != nil {
			logrus.Fatalf("读取配置文件失败: %v", err)
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logrus.Fatalf("解析配置文件失败: %v", err)
		}
		if len(cfg.Services) == 0 {
			logrus.Fatal("配置文件中至少需要配置一个服务")
		}
		for i, svc := range cfg.Services {
			if svc.Address == "" {
				logrus.Fatalf("Services[%d] (%s) 的 Address 为必填项", i, svc.Name)
			}
			if svc.Meta == nil || svc.Meta["custom_instance_id"] == "" {
				logrus.Fatalf("Services[%d] (%s) 的 Meta.custom_instance_id 为必填项", i, svc.Name)
			}
		}
		instance = &cfg
	})
	return instance
}
