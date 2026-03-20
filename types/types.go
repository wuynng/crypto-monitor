package types

import "time"

type Coin struct {
	ID     string `yaml:"id"`
	Symbol string `yaml:"symbol"`
	Name   string `yaml:"name"`
}

type Config struct {
	Dingtalk DingtalkConfig `yaml:"dingtalk"`
	Monitor  MonitorConfig  `yaml:"monitor"`
	Alert    AlertConfig    `yaml:"alert"`
	Report   ReportConfig   `yaml:"report"`
	Coins    []Coin         `yaml:"coins"`
}

type DingtalkConfig struct {
	AppKey    string `yaml:"appKey"`
	AppSecret string `yaml:"appSecret"`
	RobotCode string `yaml:"robotCode"`
	UserId    string `yaml:"userId"`
}

type MonitorConfig struct {
	Interval  int `yaml:"interval"`
	Threshold int `yaml:"threshold"`
}

type AlertConfig struct {
	Cooldown int `yaml:"cooldown"`
}

type ReportConfig struct {
	Times []int `yaml:"times"`
}

type CoinPrice struct {
	Coin
	Price       float64 `json:"usd"`
	Change24h   float64 `json:"usd_24h_change"`
	PriceChange float64
}

type AlertState struct {
	LastAlertTime time.Time
	LastDirection string
}

type AlertEvent struct {
	Coin        CoinPrice
	Direction   string
	ShouldAlert bool
}
