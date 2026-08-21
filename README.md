# cloudpvp-matcher

`cloudpvp-matcher` 是 cloudpvp 对战平台的匹配服务。服务从 RabbitMQ 接收 lobby 的开始/取消匹配命令，将等待大厅写入 Redis 队列，并由后台扫描器自然组成比赛。组队成功后，Matcher 发布完整 `Match` 到 `match.create`，由服务器分配服务补充服务器信息并通过 `match.update` 更新业务侧。

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
│       └── main.go              # 解析 flag，调用 app.Run
├── internal/
│   ├── app/
│   │   └── app.go               # 应用装配：配置、连接、仓储、用例、消费者、定时任务
│   ├── domain/
│   │   ├── config/              # GameMode、MatchConfig、配置仓储端口
│   │   ├── lobby/               # Lobby 与原始 lobby 仓储端口
│   │   ├── match/               # Matchmaker 端口和 CSGO 5v5 自然组队实现
│   │   └── matchmaking/         # LobbyEvent、完整 Match 契约和发布端口
│   ├── usecase/
│   │   └── matchmaking/         # lobby 入队/取消、匹配扫描、锁和 match.create 发布编排
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
3. 从 Apollo 读取 Redis 和 RabbitMQ 配置。
4. 初始化 Redis、RabbitMQ，并声明 RabbitMQ 拓扑。
5. 装配 Redis lobby/匹配队列仓储和 RabbitMQ 事件发布者。
6. 注册领域匹配器，目前为 `CSGO5v5Matchmaker`。
7. 创建 Redis 分布式锁和 `matchmaking.UseCase`。
8. 注册并启动匹配请求与取消消费者。
9. 启动每秒执行一次的自然组队扫描器；CSGO 5v5 队列凑满两支 5 人队伍后发布 `match.create`。

## RabbitMQ 拓扑

交换器名称来自 Apollo 配置项 `rabbitmq.exchange_name`，类型为 `topic`。

服务会在同一个 topic 交换器上幂等声明以下队列和绑定：

| 队列 | 绑定路由键 | 当前服务角色 |
|---|---|---|
| `matcher.lobby.enqueue` | `lobby.matchmaking.enqueue` | 本服务消费 |
| `matcher.lobby.cancel` | `matchmaking.cancel` | 本服务消费 |
| `lobby.lobby.update` | `lobby.update` | 本服务发布，Lobby 消费 |
| `lobby.match.update` | `match.create`、`match.update` | Matcher/Allocator 发布，业务服务消费完整 Match |
| `allocator.match.create` | `match.create` | Matcher 发布，服务器分配服务消费 |

比赛消息流只有以下两步：

1. Matcher 组队成功，发布 `status=WAITING_FOR_SERVER` 且 `server=null` 的完整 Match 到 `match.create`。`lobby.match.update` 和 `allocator.match.create` 都会收到该消息。
2. 服务器分配服务消费 `match.create`，在同一个完整 Match 上补充 `server.ip`、将状态更新为 `IN_PROGRESS`，再发布到 `match.update`。只有 `lobby.match.update` 绑定该路由键。

## 当前注册的消费者路由

当前运行时注册以下消费者：

| 消费队列 | 路由键 | 入站模型 |
|---|---|---|
| `matcher.lobby.enqueue` | `lobby.matchmaking.enqueue` | `domain/lobby.Lobby` |
| `matcher.lobby.cancel` | `matchmaking.cancel` | `domain/lobby.Lobby` |

入站消息直接反序列化为领域 `lobby.Lobby`，再交给 `matchmaking.UseCase` 路由到对应游戏模式的 `Matchmaker`。

### `Lobby`

```json
{
  "lobby_id": "lobby-001",
  "game_mode": "CS2/5v5/competitive",
  "players": [76561198000000001],
  "created_at": "2026-05-30T12:00:00Z"
}
```

字段来源于 `internal/domain/lobby.Lobby`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `lobby_id` | string | 队伍或房间 ID，必填 |
| `game_mode` | string | 游戏模式，必填 |
| `players` | int64[] | 参与匹配的玩家 ID 列表 |
| `created_at` | time | 请求创建时间 |

## 本服务发布的路由和模型

服务通过 `infra/mq.Publisher` 发布大厅状态和新建比赛两类消息。`match.update` 由服务器分配服务发布，Matcher 仅声明其到业务队列的绑定。

| 路由键 | 模型 | 触发条件 | 说明 |
|---|---|---|---|
| `lobby.update` | `domain/matchmaking.LobbyEvent` | 大厅状态变化 | 更新单个业务大厅状态 |
| `match.create` | `domain/matchmaking.Match` | 自然组队成功 | 同时通知业务服务和服务器分配服务 |

### `LobbyEvent`

```json
{
  "lobby_id": "lobby-001",
  "status": "MATCHING",
  "match_id": "match-001",
  "reason": ""
}
```

匹配完成时，Matcher 会先为每个参与比赛的 Lobby 发布 `status=MATCHING` 和对应的 `match_id`，待这些更新发布成功后，再发布 `match.create`。Lobby 保持 `MATCHING` 状态，供后续确认比赛流程使用。

### `Match`

```json
{
  "match_id": "match-001",
  "game_mode": "CS2/5v5/competitive",
  "status": "WAITING_FOR_SERVER",
  "teams": [
    {
      "lobby_ids": ["lobby-001", "lobby-002"],
      "members": []
    },
    {
      "lobby_ids": ["lobby-003", "lobby-004"],
      "members": []
    }
  ],
  "server": null,
  "created_at": "2026-05-30T12:00:00Z",
  "updated_at": "2026-05-30T12:00:00Z"
}
```

Matcher 发布的 `match.create` 必须满足：

- `status` 固定为 `WAITING_FOR_SERVER`。
- `server` 固定为 `null`。
- `teams[].lobby_ids` 保留每支队伍由哪些 lobby 组成。
- `teams[].members` 暂保留为空数组，以兼容现有下游消息模型。
- 顶层字段固定为 `match_id`、`game_mode`、`status`、`teams`、`server`、`created_at`、`updated_at`。

当前测试用服务器分配服务发布 `match.update` 时保留上述完整字段，只将状态改为 `IN_PROGRESS`，并固定写入 `{"ip":"127.0.0.1"}`。

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
