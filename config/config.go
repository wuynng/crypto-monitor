package config

import (
	"fmt"
	"os"

	"crypto-monitor/types"
	"gopkg.in/yaml.v3"
)

var DefaultConfigPath = "config.yaml"

func Load(path string) (*types.Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg types.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	setDefaults(&cfg)
	return &cfg, nil
}

func validate(cfg *types.Config) error {
	if cfg.Dingtalk.AppKey == "" || cfg.Dingtalk.AppSecret == "" {
		return fmt.Errorf("请配置钉钉 appKey+appSecret")
	}
	if cfg.Dingtalk.RobotCode == "" {
		return fmt.Errorf("请配置钉钉 robotCode")
	}
	if cfg.Dingtalk.UserId == "" {
		return fmt.Errorf("请配置钉钉 userId")
	}
	if cfg.Monitor.Interval <= 0 {
		cfg.Monitor.Interval = 60
	}
	if cfg.Monitor.Threshold <= 0 {
		cfg.Monitor.Threshold = 5
	}
	if cfg.Alert.Cooldown <= 0 {
		cfg.Alert.Cooldown = 6
	}
	if len(cfg.Coins) == 0 {
		return fmt.Errorf("未配置监控币种")
	}
	return nil
}

func setDefaults(cfg *types.Config) {
	if cfg.Monitor.Interval == 0 {
		cfg.Monitor.Interval = 60
	}
	if cfg.Monitor.Threshold == 0 {
		cfg.Monitor.Threshold = 5
	}
	if cfg.Alert.Cooldown == 0 {
		cfg.Alert.Cooldown = 6
	}
	if len(cfg.Report.Times) == 0 {
		cfg.Report.Times = []int{9, 15, 21}
	}
}
