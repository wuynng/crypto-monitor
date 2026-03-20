package client

import (
	"crypto-monitor/types"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const (
	baseURL    = "https://api.coingecko.com/api/v3"
	maxRetries = 3
)

var retryDelays = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

type CoinGeckoClient struct {
	httpClient *http.Client
}

func NewCoinGeckoClient() *CoinGeckoClient {
	return &CoinGeckoClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CoinGeckoClient) GetPrices(coins []types.Coin) ([]types.CoinPrice, error) {
	if len(coins) == 0 {
		return nil, fmt.Errorf("币种列表为空")
	}

	coinIDs := make([]string, len(coins))
	for i, coin := range coins {
		coinIDs[i] = coin.ID
	}

	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true",
		baseURL, joinIDs(coinIDs))

	var priceMap map[string]struct {
		USD          float64 `json:"usd"`
		USD24hChange float64 `json:"usd_24h_change"`
	}

	err := c.doRequestWithRetry(url, &priceMap)
	if err != nil {
		return nil, err
	}

	prices := make([]types.CoinPrice, 0, len(coins))
	for _, coin := range coins {
		if data, ok := priceMap[coin.ID]; ok {
			prices = append(prices, types.CoinPrice{
				Coin:        coin,
				Price:       data.USD,
				Change24h:   data.USD24hChange,
				PriceChange: data.USD24hChange,
			})
		}
	}

	return prices, nil
}

func (c *CoinGeckoClient) doRequestWithRetry(url string, result interface{}) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("请求失败: %w", err)
			time.Sleep(retryDelays[i])
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("服务器错误: %d", resp.StatusCode)
			time.Sleep(retryDelays[i])
			continue
		}

		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("API 限流: %d", resp.StatusCode)
			time.Sleep(retryDelays[i])
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("请求失败: %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			lastErr = fmt.Errorf("解析响应失败: %w", err)
			time.Sleep(retryDelays[i])
			continue
		}

		return nil
	}

	return fmt.Errorf("重试 %d 次后仍失败: %w", maxRetries, lastErr)
}

func joinIDs(ids []string) string {
	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += id
	}
	return result
}

func FormatPrice(price float64) string {
	if price >= 1000 {
		return fmt.Sprintf("$%.2f", price)
	} else if price >= 1 {
		return fmt.Sprintf("$%.4f", price)
	}
	return fmt.Sprintf("$%.6f", price)
}

func FormatChange(change float64) string {
	sign := "+"
	if change < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f%%", sign, change)
}

func GetEmoji(change float64) string {
	absChange := math.Abs(change)
	switch {
	case absChange >= 10:
		return "🚀"
	case change >= 5:
		return "🔴"
	case change >= 0:
		return "🟢"
	case change >= -5:
		return "🟢"
	case change >= -10:
		return "🔵"
	default:
		return "💥"
	}
}
