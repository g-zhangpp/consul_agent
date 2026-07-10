package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"consul_agent/config"
	"consul_agent/consul"
	"consul_agent/logs"

	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.GetConf()

	// 初始化日志
	logs.InitLog(cfg.Logs.LogFilePath)

	// 创建 Consul 客户端
	client, err := consul.NewClient()
	if err != nil {
		logrus.Fatalf("Consul连接失败: %v", err)
	}

	// 注册所有服务
	if err := client.RegisterAll(); err != nil {
		logrus.Fatalf("服务注册失败: %v", err)
	}
	logrus.Info("所有服务注册完成")

	// 监听退出信号，优雅注销
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logrus.Infof("收到信号 %v，开始注销服务...", sig)
		client.DeregisterAll()
		os.Exit(0)
	}()

	// 启动 HTTP 健康检查服务
	listenAddr := cfg.System.ListenAddress + ":" + cfg.System.Port
	http.HandleFunc("/", consul.HealthHandler)
	logrus.Infof("健康检查服务启动: %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		logrus.Fatalf("HTTP服务启动失败: %v", err)
	}
}
