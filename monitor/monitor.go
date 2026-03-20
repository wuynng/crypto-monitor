package monitor

import (
	"context"
	"crypto-monitor/client"
	"crypto-monitor/config"
	"crypto-monitor/types"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const pidFile = "/tmp/crypto-monitor.pid"

type DingTalkSender interface {
	SendAlert(prices []types.CoinPrice, threshold int) error
	SendReport(prices []types.CoinPrice) error
}

type Monitor struct {
	config      *types.Config
	coingecko   *client.CoinGeckoClient
	dingtalk    DingTalkSender
	alertStates map[string]*types.AlertState
	mu          sync.RWMutex
	stopChan    chan struct{}
}

func New(cfg *types.Config) *Monitor {
	coingecko := client.NewCoinGeckoClient()

	dingtalk := client.NewDingTalkClient(
		cfg.Dingtalk.AppKey,
		cfg.Dingtalk.AppSecret,
		cfg.Dingtalk.RobotCode,
		cfg.Dingtalk.UserId,
		cfg,
		coingecko,
	)

	return &Monitor{
		config:      cfg,
		coingecko:   coingecko,
		dingtalk:    dingtalk,
		alertStates: make(map[string]*types.AlertState),
		stopChan:    make(chan struct{}),
	}
}

func (m *Monitor) Start() error {
	if IsRunning() {
		return fmt.Errorf("监控服务已在运行")
	}

	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("写入 PID 文件失败: %w", err)
	}

	dingtalkClient := m.dingtalk.(*client.DingTalkClient)
	if err := dingtalkClient.Start(context.Background()); err != nil {
		return fmt.Errorf("启动钉钉Stream失败: %w", err)
	}

	fmt.Printf("✅ 监控服务已启动 (PID: %d)\n", os.Getpid())
	fmt.Printf("📊 监控间隔: %d 分钟, 阈值: ±%d%%\n", m.config.Monitor.Interval, m.config.Monitor.Threshold)
	fmt.Printf("📅 定时报告时间点: %v\n", m.config.Report.Times)
	fmt.Printf("💰 监控币种: ")
	for i, coin := range m.config.Coins {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(coin.Symbol)
	}
	fmt.Println()

	go m.run()
	go m.handleSignal()

	return nil
}

func (m *Monitor) run() {
	m.checkAndAlert()

	intervalTicker := time.NewTicker(time.Duration(m.config.Monitor.Interval) * time.Minute)
	reportTicker := time.NewTicker(1 * time.Minute)
	defer intervalTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-intervalTicker.C:
			m.checkAndAlert()
		case <-reportTicker.C:
			m.checkAndSendReport()
		}
	}
}

func (m *Monitor) handleSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan
	m.Stop()
	os.Exit(0)
}

func (m *Monitor) checkAndAlert() {
	prices, err := m.coingecko.GetPrices(m.config.Coins)
	if err != nil {
		fmt.Printf("❌ 获取价格失败: %v\n", err)
		return
	}

	alertPrices := m.filterAlertPrices(prices)
	if len(alertPrices) > 0 {
		if err := m.dingtalk.SendAlert(alertPrices, m.config.Monitor.Threshold); err != nil {
			fmt.Printf("❌ 发送告警失败: %v\n", err)
		} else {
			fmt.Printf("✅ 告警发送成功 (%d 个币种)\n", len(alertPrices))
			m.updateAlertStates(alertPrices)
		}
	} else {
		fmt.Println("ℹ️  价格波动未超过阈值")
	}
}

func (m *Monitor) checkAndSendReport() {
	now := time.Now()
	hour := now.Hour()

	for _, reportHour := range m.config.Report.Times {
		if hour == reportHour && now.Minute() == 0 {
			prices, err := m.coingecko.GetPrices(m.config.Coins)
			if err != nil {
				fmt.Printf("❌ 获取价格失败: %v\n", err)
				return
			}

			if err := m.dingtalk.SendReport(prices); err != nil {
				fmt.Printf("❌ 发送日报失败: %v\n", err)
			} else {
				fmt.Printf("✅ 日报发送成功 (%d 个币种)\n", len(prices))
			}
			return
		}
	}
}

func (m *Monitor) filterAlertPrices(prices []types.CoinPrice) []types.CoinPrice {
	var result []types.CoinPrice
	threshold := float64(m.config.Monitor.Threshold)

	for _, p := range prices {
		if !m.shouldAlert(p) {
			continue
		}

		absChange := p.Change24h
		if absChange < 0 {
			absChange = -absChange
		}

		if absChange >= threshold {
			result = append(result, p)
		}
	}

	return result
}

func (m *Monitor) shouldAlert(p types.CoinPrice) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.alertStates[p.ID]
	if !exists {
		return true
	}

	cooldownDuration := time.Duration(m.config.Alert.Cooldown) * time.Hour
	if time.Since(state.LastAlertTime) < cooldownDuration {
		if (state.LastDirection == "up" && p.Change24h > 0) ||
			(state.LastDirection == "down" && p.Change24h < 0) {
			return false
		}
	}

	return true
}

func (m *Monitor) updateAlertStates(prices []types.CoinPrice) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, p := range prices {
		direction := "up"
		if p.Change24h < 0 {
			direction = "down"
		}
		m.alertStates[p.ID] = &types.AlertState{
			LastAlertTime: now,
			LastDirection: direction,
		}
	}
}

func (m *Monitor) Check() error {
	prices, err := m.coingecko.GetPrices(m.config.Coins)
	if err != nil {
		return fmt.Errorf("获取价格失败: %w", err)
	}

	fmt.Println("\n📊 当前价格:")
	fmt.Println("| 名称 | 价格 | 24h 涨跌幅 |")
	fmt.Println("| :---- | ------: | -------: |")
	for _, p := range prices {
		emoji := client.GetEmoji(p.Change24h)
		fmt.Printf("| **%s** | %s | **%s** %s |\n",
			p.Symbol, client.FormatPrice(p.Price), client.FormatChange(p.Change24h), emoji)
	}
	fmt.Println()
	return nil
}

func (m *Monitor) Alert() error {
	prices, err := m.coingecko.GetPrices(m.config.Coins)
	if err != nil {
		return fmt.Errorf("获取价格失败: %w", err)
	}

	if err := m.dingtalk.SendReport(prices); err != nil {
		return fmt.Errorf("发送告警失败: %w", err)
	}

	fmt.Printf("✅ 手动告警发送成功 (%d 个币种)\n", len(prices))
	return nil
}

func (m *Monitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertStates = make(map[string]*types.AlertState)
	fmt.Println("✅ 告警状态已重置")
}

func (m *Monitor) Stop() {
	fmt.Println("\n🛑 正在停止监控服务...")

	if dingtalkClient, ok := m.dingtalk.(*client.DingTalkClient); ok {
		dingtalkClient.Close()
	}

	close(m.stopChan)
	os.Remove(pidFile)
	fmt.Println("✅ 监控服务已停止")
}

func IsRunning() bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func Stop() error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("监控服务未运行")
	}

	var pid int
	if _, err := fmt.Sscan(string(data), &pid); err != nil {
		return fmt.Errorf("解析 PID 失败: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		return fmt.Errorf("找不到进程: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送终止信号失败: %w", err)
	}

	time.Sleep(500 * time.Millisecond)
	os.Remove(pidFile)
	fmt.Println("✅ 监控服务已停止")
	return nil
}

func LoadConfig(cfgPath string) (*types.Config, error) {
	return config.Load(cfgPath)
}
