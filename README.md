# cloudpvp-matcher

`cloudpvp-matcher` 是 cloudpvp 对战平台的匹配服务。服务从 RabbitMQ 接收 lobby 匹配请求，由用例层路由到对应游戏模式的 Matchmaker，并在匹配结果或确认请求产生后通过 RabbitMQ 发布领域消息。

## 架构

项目按 Clean Architecture 分层，依赖方向如下：

```text
infra → handler → usecase → domain
基础设施   适配层     用例层      领域层
```

约束：

- `domain` 不依赖任何其他 `internal` 包。
- `usecase` 只依赖 `domain`。
- RabbitMQ 入站/出站消息直接使用领域消息模型，不再保留独立 DTO 层。
- `infra` 负责 Apollo、Redis、RabbitMQ 等外部系统接入。
- `internal/app` 是运行时装配层，负责配置加载、依赖初始化、路由注册和后台任务启动。
- `cmd/matcher` 只解析命令行参数并调用 `app.Run`。

## 目录结构

```text
cloudpvp-matcher/
├── cmd/
│   └── matcher/
│       ├── main.go              # 解析 flag，调用 app.Run
│       └── exit.go              # 统一处理启动错误和退出码
├── internal/
│   ├── app/
│   │   └── app.go               # 应用装配：配置、连接、仓储、用例、消费者、定时任务
│   ├── domain/
│   │   ├── config/              # GameMode、MatchConfig、配置仓储端口
│   │   ├── lobby/               # Lobby、PlayerInfo、原始 lobby 仓储端口
│   │   ├── match/               # Match、Team、Matchmaker、事件/锁端口、CSGO 5v5 匹配器
│   │   └── matchmaking/         # 匹配发布端口、MatchResult、ConfirmRequest
│   ├── usecase/
│   │   └── matchmaking/         # lobby 请求路由、Matchmaker 注册、确认和结果发布编排
│   └── infra/
│       ├── cache/               # Redis 客户端和仓储实现
│       ├── config/              # 本地 Apollo 启动配置加载、本地配置仓储
│       └── mq/                  # RabbitMQ 连接、拓扑声明、消费者、发布者、路由常量
├── config.yaml                  # 本地 Apollo 启动配置
├── go.mod
└── README.md
```

## 启动流程

`cmd/matcher/main.go` 支持一个参数：

```bash
go run ./cmd/matcher -config ./config.yaml
```

如果不传 `-config`，默认读取当前目录下的 `config.yaml`，并支持 `MATCHER_APOLLO_*` 与旧版 `APOLLO_*` 环境变量覆盖。

启动后 `internal/app.Run` 会完成：

1. 读取本地 Apollo 启动配置。
2. 初始化 Apollo 客户端。
3. 从 Apollo 读取 Redis、RabbitMQ 和匹配模式配置。
4. 初始化 Redis、RabbitMQ，并声明 RabbitMQ 拓扑。
5. 装配 Redis 票据仓储、本地配置仓储、RabbitMQ 事件发布者。
6. 注册领域匹配器，目前为 `CSGO5v5Matchmaker`。
7. 创建 Redis 分布式锁、`matchmaking.UseCase` 和 `MatchHandler`。
8. 注册并启动匹配请求消费者。
9. 启动后台匹配扫描器，每秒按游戏模式尝试组装对局。
10. 启动票据过期清理定时任务，每 30 秒清理超过 5 分钟仍在队列中的票据。

## RabbitMQ 拓扑

交换器名称来自 Apollo 配置项 `rabbitmq.exchange_name`，类型为 `topic`。

服务启动时会声明以下队列和绑定：

| 队列 | 绑定路由键 | 当前服务角色 |
|---|---|---|
| `matchmaking.request.queue` | `matchmaking.request` | 本服务消费 |
| `matchmaking.cancel.queue` | `matchmaking.cancel` | 本服务消费 |
| `match.result.queue` | `match.result` | 本服务发布，外部服务消费 |
| `match.confirm.queue` | `match.confirm.*` | 预留确认相关队列，当前服务发布 `match.confirm.request` |

## 当前注册的消费者路由

当前运行时注册以下消费者：

| 消费队列 | 路由键 | 入站模型 |
|---|---|---|
| `matchmaking.request.queue` | `matchmaking.request` | `domain/lobby.Lobby` |
| `matchmaking.cancel.queue` | `matchmaking.cancel` | `domain/lobby.Lobby` |

入站消息直接反序列化为领域 `lobby.Lobby`，再交给 `matchmaking.UseCase` 路由到对应游戏模式的 `Matchmaker`。

### `Lobby`

```json
{
  "lobby_id": "lobby-001",
  "game_mode": "matchmaker/5v5/competitive",
  "members": [
    {
      "player_id": "player-001"
    }
  ],
  "created_at": "2026-05-30T12:00:00Z"
}
```

字段来源于 `internal/domain/lobby.Lobby`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `lobby_id` | string | 队伍或房间 ID，必填 |
| `game_mode` | string | 游戏模式，必填 |
| `members` | `PlayerInfo[]` | 参与匹配的玩家 |
| `created_at` | time | 请求创建时间 |

`PlayerInfo`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `player_id` | string | 玩家 ID |

## 本服务发布的路由和模型

匹配完成后，服务通过 `infra/mq.Publisher` 发布以下消息。

| 路由键 | 模型 | 触发条件 | 说明 |
|---|---|---|---|
| `match.result` | `domain/matchmaking.MatchResult` | 匹配结果已确定 | 通知业务服务匹配结果 |
| `match.confirm.request` | `domain/matchmaking.ConfirmRequest` | 需要玩家确认 | 请求业务服务确认指定 lobby |

### `MatchResult`

```json
{
  "message_id": "",
  "match_id": "match-001",
  "game_mode": "matchmaker/5v5/competitive",
  "teams": [
    {
      "lobby_id": "lobby-001",
      "lobby_ids": ["lobby-001"],
      "members": [
        {
          "player_id": "player-001"
        }
      ]
    }
  ],
  "matched_at": "2026-05-30T12:00:00Z"
}
```

### `ConfirmRequest`

```json
{
  "lobby_ids": ["lobby-001", "lobby-002"]
}
```

注意：

- 当前发布者里 `message_id` 仍为空字符串，代码中标记为待上层注入。
- `TeamInfo.lobby_ids` 表示该队伍由哪些 lobby 拼成；单 lobby 队伍会同时填充兼容字段 `lobby_id`。

## 配置项

本地 `config.yaml` 只用于启动 Apollo 客户端；运行时业务配置从 Apollo 获取。

本服务当前读取的 Apollo 配置键：

| 配置键 | 说明 |
|---|---|
| `redis.addr` | Redis 地址，必填 |
| `redis.password` | Redis 密码 |
| `redis.db` | Redis DB 编号 |
| `rabbitmq.url` | RabbitMQ 连接地址，必填 |
| `rabbitmq.exchange_name` | RabbitMQ topic 交换器名称，必填 |
| `match_modes` | 匹配模式配置 JSON 数组 |

`match_modes` 示例：

```json
[
  {
    "game_mode": "matchmaker/5v5/competitive",
    "team_size": 5,
    "team_count": 2,
    "need_confirm": false,
    "confirm_timeout": "30s",
    "match_timeout": "5m"
  }
]
```

## 本地开发

```bash
# 运行测试
go test ./...

# 启动服务，需要可访问的 Apollo、Redis、RabbitMQ
go run ./cmd/matcher -config ./config.yaml
```

## 新增游戏模式

1. 在 `internal/domain/config/game_mode.go` 添加 `GameMode` 常量。
2. 在 `internal/domain/match/` 实现 `Matchmaker`。
3. 在 `internal/app/app.go` 注册新的匹配器。
4. 在 Apollo 的 `match_modes` 中添加对应游戏模式配置。
