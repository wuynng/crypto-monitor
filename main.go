package main

import (
	"crypto-monitor/monitor"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfgPath := "config.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := monitor.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 配置加载失败: %v\n", err)
		os.Exit(1)
	}

	m := monitor.New(cfg)

	cmd := os.Args[1]
	switch cmd {
	case "start":
		if err := m.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		select {}

	case "stop":
		if err := monitor.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "alert":
		if err := m.Alert(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("❌ 未知命令: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: crypto-monitor <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start  - 启动监控服务")
	fmt.Println("  stop   - 停止监控服务")
	fmt.Println("  alert  - 手动发送告警")
}
