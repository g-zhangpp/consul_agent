# Consul Agent - 多服务注册工具

## 简介

Consul Agent 是一个轻量级的服务注册工具，用于将本机运行的多个服务（如 Prometheus Exporter）批量注册到 Consul 集群中。支持自定义 Tag、元数据（Meta）和健康检查，适用于需要统一管理服务发现的场景。

### 核心功能

- **多服务注册**：一次运行即可注册多个不同的服务（如 node-exporter、dcgm-exporter 等）
- **自定义 Tag**：每个服务可配置独立的标签列表，便于分类和过滤
- **自定义元数据（Meta）**：每个服务可配置独立的键值对元数据
- **健康检查**：自动为每个注册的服务配置 HTTP 健康检查
- **优雅退出**：监听 SIGINT/SIGTERM 信号，退出时自动从 Consul 注销所有服务
- **日志轮转**：支持按天轮转，自动清理过期日志

## 编译环境

- **Go 版本**：>= 1.21
- **操作系统**：Linux / Windows / macOS

### 编译步骤

```bash
cd consul_agent
go build -o consul_agent main.go
```

## 部署使用

### 1. 准备配置文件

编辑 `conf/config.yaml`，根据实际环境修改以下配置：

```yaml
System:
  ServiceName: consul-agent
  ListenAddress: 0.0.0.0
  Port: 9985
  FindAddress: 8.8.8.8:80

Logs:
  LogFilePath: ../logs/consul-agent.log
  LogLevel: info

Consul:
  Address: 1.2.3.4:8500,1.2.3.5:8500,1.2.3.6:8500
  Token: your-consul-token
  CheckTimeout: 5s
  CheckInterval: 5s
  CheckDeregisterCriticalServiceAfter: false
  CheckDeregisterCriticalServiceAfterTime: 30s

Services:
  - Name: node-exporter
    Port: 9100
    Address: "10.192.161.100"
    Tags:
      - "env:production"
      - "team:ops"
    Meta:
      version: "1.6.1"
      description: "Node metrics exporter"
      custom_instance_id: "6a494ad3-b071-4be3-b174-aca6e807928e"

  - Name: dcgm-exporter
    Port: 9400
    Address: "10.192.161.100"
    Tags:
      - "env:production"
      - "team:gpu"
    Meta:
      version: "3.3.5"
      description: "NVIDIA DCGM GPU metrics exporter"
      custom_instance_id: "6a494ad3-b071-4be3-b174-aca6e807928e"
```

### 配置说明

| 配置项 | 说明 |
|--------|------|
| `System.Port` | 本工具自身的 HTTP 健康检查端口 |
| `System.FindAddress` | 用于自动获取本机出口 IP 的远程地址（UDP，不会真正发送数据） |
| `Consul.Address` | Consul 集群地址，多个用逗号分隔，程序会随机选择一个连接 |
| `Consul.Token` | Consul ACL Token，未启用 ACL 可留空 |
| `Services[].Name` | 服务名称，对应 Consul 中的 Service Name |
| `Services[].Port` | 服务端口 |
| `Services[].Address` | **必填**，本机 IP 地址，用于注册和健康检查 |
| `Services[].Tags` | 自定义标签列表 |
| `Services[].Meta` | 自定义元数据键值对 |
| `Services[].Meta.custom_instance_id` | **必填**，实例唯一标识，用于生成全局唯一的 Service ID，避免跨区域 IP 冲突 |

### 2. 运行

```bash
# 使用默认配置路径
./consul_agent

# 指定配置文件路径
./consul_agent -confpath /path/to/config.yaml
```

### 3. 验证注册结果

在 Consul UI 中可以看到：

```
node-exporter (Service)
  └── node-exporter-<custom_instance_id>-<本机IP>-9100 (Instance)
      Tags: [env:production, team:ops]
      Meta: {version: 1.6.1, description: Node metrics exporter, custom_instance_id: 6a494ad3-...}

dcgm-exporter (Service)
  └── dcgm-exporter-<custom_instance_id>-<本机IP>-9400 (Instance)
      Tags: [env:production, team:gpu]
      Meta: {version: 3.3.5, description: NVIDIA DCGM GPU metrics exporter, custom_instance_id: 6a494ad3-...}
```

### 4. 配合 Prometheus 使用

在 Prometheus 配置中使用 Consul 服务发现：

```yaml
scrape_configs:
  - job_name: 'node-exporter'
    consul_sd_configs:
      - server: 'consul-server:8500'
        services: ['node-exporter']
        tags: ['env:production']

  - job_name: 'dcgm-exporter'
    consul_sd_configs:
      - server: 'consul-server:8500'
        services: ['dcgm-exporter']
        tags: ['env:production']
```

### 5. 使用 Systemd 管理（Linux）

```ini
[Unit]
Description=Consul Agent - Multi Service Registration
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/consul_agent
ExecStart=/opt/consul_agent/consul_agent -confpath /opt/consul_agent/conf/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable consul_agent
sudo systemctl start consul_agent
```

## 注意事项

- **启动前必须配置 `Address` 和 `custom_instance_id`**：在启动本服务前，务必在 `config.yaml` 中为每个服务填入本机的真实 IP 地址（`Address`）和唯一的实例标识（`Meta.custom_instance_id`）。这两个字段为必填项，缺失将导致程序启动失败。`custom_instance_id` 建议使用 UUID，确保跨区域、跨实例全局唯一。
- 确保本机对应的服务（如 node-exporter、dcgm-exporter）已启动，否则 Consul 健康检查会标记为不健康
- 如果开启了 `CheckDeregisterCriticalServiceAfter`，不健康的服务实例会在指定时间后被 Consul 自动注销
- 程序退出时会自动注销所有已注册的服务，建议通过 Systemd 管理以确保异常重启时重新注册
- Service ID 格式为 `<Name>-<custom_instance_id>-<Address>-<Port>`，通过 `custom_instance_id` 保证不同区域、不同实例之间的唯一性，避免 IP 冲突导致的服务覆盖

## 特别感谢

感谢 [consul-registy-service](https://github.com/FrankenFuncc/consul-registy-service) 项目给本工具带来的启发。
