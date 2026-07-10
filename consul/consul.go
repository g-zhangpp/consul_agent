package consul

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"consul_agent/config"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/sirupsen/logrus"
)

// Client 封装 Consul 客户端连接
type Client struct {
	client *consulapi.Client
}

// NewClient 创建 Consul 客户端连接
func NewClient() (*Client, error) {
	cfg := config.GetConf()
	config := consulapi.DefaultConfig()
	config.Address = getConsulAddr()
	config.Token = cfg.Consul.Token

	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建Consul客户端失败: %v", err)
	}
	logrus.Infof("已连接到Consul节点: %s", config.Address)
	return &Client{client: client}, nil
}

// RegisterAll 注册所有配置的服务
func (c *Client) RegisterAll() error {
	cfg := config.GetConf()

	for _, svc := range cfg.Services {
		if err := c.registerService(svc); err != nil {
			return fmt.Errorf("注册服务 %s 失败: %v", svc.Name, err)
		}
	}
	return nil
}

// DeregisterAll 注销所有配置的服务
func (c *Client) DeregisterAll() {
	cfg := config.GetConf()

	for _, svc := range cfg.Services {
		instanceID := svc.Meta["custom_instance_id"]
		serviceID := buildServiceID(svc.Name, instanceID, svc.Address, svc.Port)
		if err := c.client.Agent().ServiceDeregister(serviceID); err != nil {
			logrus.Errorf("注销服务 %s 失败: %v", serviceID, err)
		} else {
			logrus.Infof("已注销服务: %s", serviceID)
		}
	}
}

func (c *Client) registerService(svc config.ServiceConfig) error {
	cfg := config.GetConf()

	port, err := strconv.Atoi(svc.Port)
	if err != nil {
		return fmt.Errorf("端口转换失败: %v", err)
	}

	addr := svc.Address
	instanceID := svc.Meta["custom_instance_id"]
	serviceID := buildServiceID(svc.Name, instanceID, addr, svc.Port)

	// 构建健康检查地址
	checkHTTP := fmt.Sprintf("http://%s:%d/", addr, port)
	if cfg.Consul.Token != "" {
		checkHTTP += "?token=" + cfg.Consul.Token
	}

	registration := &consulapi.AgentServiceRegistration{
		ID:      serviceID,
		Name:    svc.Name,
		Port:    port,
		Address: addr,
		Tags:    svc.Tags,
		Meta:    svc.Meta,
		Check: &consulapi.AgentServiceCheck{
			HTTP:     checkHTTP,
			Timeout:  cfg.Consul.CheckTimeout,
			Interval: cfg.Consul.CheckInterval,
		},
	}

	// 配置自动注销不健康服务
	if cfg.Consul.CheckDeregisterCriticalServiceAfter && cfg.Consul.CheckDeregisterCriticalServiceAfterTime != "" {
		registration.Check.DeregisterCriticalServiceAfter = cfg.Consul.CheckDeregisterCriticalServiceAfterTime
		logrus.Infof("已开启自动注销不健康服务，超时时间: %s", cfg.Consul.CheckDeregisterCriticalServiceAfterTime)
	}

	// 先尝试注销同名旧注册，避免重复
	_ = c.client.Agent().ServiceDeregister(serviceID)

	if err := c.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("注册失败: %v", err)
	}

	logrus.Infof("服务注册成功: %s (ID: %s, 地址: %s:%d, Tags: %v, Meta: %v)",
		svc.Name, serviceID, addr, port, svc.Tags, svc.Meta)
	return nil
}

func buildServiceID(name, instanceID, addr, port string) string {
	return name + "-" + instanceID + "-" + addr + "-" + port
}

// getConsulAddr 从配置的 Consul 地址列表中随机选择一个
func getConsulAddr() string {
	cfg := config.GetConf()
	addrs := strings.Split(cfg.Consul.Address, ",")
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return addrs[r.Intn(len(addrs))]
}

// HealthHandler HTTP 健康检查处理函数
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
