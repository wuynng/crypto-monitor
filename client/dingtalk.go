package client

import (
	"context"
	"crypto-monitor/types"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dingtalkoauth2_1_0 "github.com/alibabacloud-go/dingtalk/oauth2_1_0"
	dingtalkrobot_1_0 "github.com/alibabacloud-go/dingtalk/robot_1_0"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
)

type DingTalkClient struct {
	streamClient *client.StreamClient
	appKey       string
	appSecret    string
	robotCode    string
	userId       string
	config       *types.Config
	coingecko    *CoinGeckoClient
	mu           sync.Mutex
	sender       *chatbot.ChatbotReplier
}

func NewDingTalkClient(appKey, appSecret, robotCode, userId string, cfg *types.Config, cg *CoinGeckoClient) *DingTalkClient {
	logger.SetLogger(logger.NewStdTestLogger())

	cli := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig(appKey, appSecret)),
	)

	d := &DingTalkClient{
		streamClient: cli,
		appKey:       appKey,
		appSecret:    appSecret,
		robotCode:    robotCode,
		userId:       userId,
		config:       cfg,
		coingecko:    cg,
		sender:       chatbot.NewChatbotReplier(),
	}

	cli.RegisterChatBotCallbackRouter(d.OnChatBotMessageReceived)

	return d
}

func (d *DingTalkClient) Start(ctx context.Context) error {
	return d.streamClient.Start(ctx)
}

func (d *DingTalkClient) Close() {
	d.streamClient.Close()
}

func (d *DingTalkClient) OnChatBotMessageReceived(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	userMessage := strings.TrimSpace(data.Text.Content)

	if userMessage == "/price" {
		prices, err := d.coingecko.GetPrices(d.config.Coins)
		if err != nil {
			replyText := fmt.Sprintf("获取价格失败: %v", err)
			if err := d.sender.SimpleReplyText(ctx, data.SessionWebhook, []byte(replyText)); err != nil {
				logger.GetLogger().Errorf("回复消息失败: %v", err)
			}
			return []byte(""), nil
		}

		msg := d.buildReportMessage(prices)
		replyText := msg["markdown"].(map[string]string)["text"]

		if err := d.sender.SimpleReplyMarkdown(ctx, data.SessionWebhook, []byte("加密货币价格"), []byte(replyText)); err != nil {
			logger.GetLogger().Errorf("回复消息失败: %v", err)
			return nil, err
		}
	} else {
		replyText := fmt.Sprintf("收到 %s", userMessage)
		if err := d.sender.SimpleReplyText(ctx, data.SessionWebhook, []byte(replyText)); err != nil {
			logger.GetLogger().Errorf("回复消息失败: %v", err)
			return nil, err
		}
	}

	return []byte(""), nil
}

func (d *DingTalkClient) SendAlert(prices []types.CoinPrice, threshold int) error {
	if len(prices) == 0 {
		return nil
	}

	msg := d.buildAlertMessage(prices, threshold)
	return d.sendMessageToUser(msg)
}

func (d *DingTalkClient) SendReport(prices []types.CoinPrice) error {
	if len(prices) == 0 {
		return nil
	}

	msg := d.buildReportMessage(prices)
	return d.sendMessageToUser(msg)
}

func (d *DingTalkClient) getAccessToken() (string, error) {
	config := &openapi.Config{}
	config.Protocol = tea.String("https")
	config.RegionId = tea.String("central")

	client, err := dingtalkoauth2_1_0.NewClient(config)
	if err != nil {
		return "", fmt.Errorf("创建oauth客户端失败: %w", err)
	}

	getAccessTokenRequest := &dingtalkoauth2_1_0.GetAccessTokenRequest{
		AppKey:    tea.String(d.appKey),
		AppSecret: tea.String(d.appSecret),
	}

	response, err := client.GetAccessToken(getAccessTokenRequest)
	if err != nil {
		return "", fmt.Errorf("获取access_token失败: %w", err)
	}

	return tea.StringValue(response.Body.AccessToken), nil
}

func (d *DingTalkClient) sendMessageToUser(msg map[string]interface{}) error {
	text := msg["markdown"].(map[string]string)["text"]
	title := msg["markdown"].(map[string]string)["title"]

	accessToken, err := d.getAccessToken()
	if err != nil {
		return err
	}

	robotClient, err := dingtalkrobot_1_0.NewClient(&openapi.Config{
		Protocol: tea.String("https"),
		RegionId: tea.String("central"),
	})
	if err != nil {
		return fmt.Errorf("创建robot客户端失败: %w", err)
	}

	markdownMsg := fmt.Sprintf(`{"text": "%s", "title": "%s"}`, text, title)

	headers := &dingtalkrobot_1_0.BatchSendOTOHeaders{}
	headers.XAcsDingtalkAccessToken = tea.String(accessToken)

	request := &dingtalkrobot_1_0.BatchSendOTORequest{
		RobotCode: tea.String(d.robotCode),
		UserIds:   []*string{tea.String(d.userId)},
		MsgKey:    tea.String("sampleMarkdown"),
		MsgParam:  tea.String(markdownMsg),
	}

	_, err = robotClient.BatchSendOTOWithOptions(request, headers, &util.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	logger.GetLogger().Infof("消息发送成功")
	return nil
}

func (d *DingTalkClient) buildAlertMessage(prices []types.CoinPrice, threshold int) map[string]interface{} {
	var content strings.Builder
	content.WriteString("## 🚨 加密货币价格告警\n\n")

	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Symbol < prices[j].Symbol
	})

	for _, p := range prices {
		emoji := GetEmoji(p.Change24h)
		changeStr := FormatChange(p.Change24h)
		symbol := FormatSymbol(p.Symbol)
		content.WriteString(fmt.Sprintf("- **%s**  $%s | **%s** %s\n",
			symbol, FormatPriceCompact(p.Price), changeStr, emoji))
	}

	content.WriteString(fmt.Sprintf("\n---\n⏰ %s", time.Now().Format("2006-01-02 15:04:05")))

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "加密货币价格告警",
			"text":  content.String(),
		},
	}
}

func (d *DingTalkClient) buildReportMessage(prices []types.CoinPrice) map[string]interface{} {
	var content strings.Builder
	content.WriteString("## 📊 加密货币价格日报\n\n")

	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Symbol < prices[j].Symbol
	})

	for _, p := range prices {
		emoji := GetEmoji(p.Change24h)
		changeStr := FormatChange(p.Change24h)
		symbol := FormatSymbol(p.Symbol)
		content.WriteString(fmt.Sprintf("- **%s**  $%s | **%s** %s\n",
			symbol, FormatPriceCompact(p.Price), changeStr, emoji))
	}

	content.WriteString(fmt.Sprintf("\n---\n⏰ %s", time.Now().Format("2006-01-02 15:04:05")))

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "加密货币价格日报",
			"text":  content.String(),
		},
	}
}
