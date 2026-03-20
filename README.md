# 加密货币价格监控工具

监控多种加密货币价格，当 24 小时波动超过阈值时通过钉钉机器人发送告警。

## 功能特性

- ✅ 支持多种加密货币（BTC、ETH、SOL等），可配置
- ✅ 从 CoinGecko API 获取实时价格
- ✅ 可配置价格波动阈值（默认 ±5%）
- ✅ 钉钉 Stream 模式实时接收用户消息
- ✅ 阿里云 OpenAPI 发送单聊消息
- ✅ 告警冷却机制，防止重复通知
- ✅ 防止重复启动监控进程
- ✅ 定时发送价格日报（9/15/21点）
- ✅ 用户发送 /price 命令获取当前价格

## 快速开始

### 1. 创建钉钉应用

1. 登录钉钉开放平台创建企业内部应用
2. 获取 `AppKey` 和 `AppSecret`
3. 在应用配置中添加机器人能力，获取 `RobotCode`
4. 配置应用可用范围（需要能发消息给目标用户）

### 2. 编辑配置文件

编辑 `config.yaml`：

```yaml
dingtalk:
  appKey: "your_app_key"
  appSecret: "your_app_secret"
  robotCode: "your_robot_code"
  userId: "target_user_id"

monitor:
  interval: 60        # 检查间隔（分钟）
  threshold: 5        # 告警阈值（%）

alert:
  cooldown: 6         # 告警冷却时间（小时）

report:
  times: [9, 15, 21] # 定时发送日报的时间点

coins:
  - id: bitcoin
    symbol: BTC
    name: 比特币
  - id: ethereum
    symbol: ETH
    name: 以太坊
  - id: solana
    symbol: SOL
    name: Solana
```

### 3. 启动监控

```bash
# 启动监控服务
./crypto-monitor start

# 停止监控服务
./crypto-monitor stop

# 手动发送告警
./crypto-monitor alert
```

## 命令说明

| 命令 | 功能 |
|------|------|
| `start` | 启动监控服务（建立钉钉Stream长连接） |
| `stop` | 停止监控服务 |
| `alert` | 手动发送一次告警 |

## 用户交互

在钉钉中向机器人发送消息：

| 消息 | 响应 |
|------|------|
| `/price` | 返回当前所有加密货币价格 |
| 其他消息 | 回复"收到{消息内容}" |

## 告警消息示例

```
## 🚨 加密货币价格告警

- **BTC**  $71,129.00 | **-4.10%** 🟢
- **ETH**  $2,198.29 | **-5.68%** 🔵
- **SOL**  $90.1500 | **-4.58%** 🟢

---
⏰ 更新时间：2026-03-19 10:00:00
```

## Emoji 说明

| 变化幅度 | Emoji | 说明 |
|----------|-------|------|
| ≥ +10% | 🚀 | 暴涨 |
| +5% ~ +10% | 🔴 | 大幅上涨 |
| -5% ~ +5% | 🟢 | 正常波动 |
| -10% ~ -5% | 🔵 | 大幅下跌 |
| ≤ -10% | 💥 | 暴跌 |

## 告警抑制机制

- 同一币种同一方向的告警，在冷却期内只发送一次
- 默认冷却时间：6 小时（可在配置文件中调整）
- 冷却期过后或方向反转时，会再次发送告警
- 程序重启后冷却状态自动重置

## 技术栈

- Go 1.21+
- CoinGecko API（免费）
- 钉钉 Stream 模式（实时消息）
- 阿里云 OpenAPI SDK（发送消息）

## 注意事项

1. `config.yaml` 包含敏感信息，请勿提交到版本库
2. 项目已配置 `.gitignore`，会自动忽略敏感文件
3. CoinGecko API 有速率限制，建议监控间隔不低于 30 分钟
4. PID 文件存储在 `/tmp/crypto-monitor.pid`