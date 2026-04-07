# AI Context

面向 AI / agent 的项目快速上下文。

目标是：先用这一份文件快速建立仓库结构、关键业务和当前代码边界，再决定深入读哪些目录。

## 1. 一句话总结

这是一个 `Unity 微信小游戏客户端 + Go 微服务后端` 的联调仓库。

当前真正已经落地并持续演进的业务主线在 `IAAServer/svr_game`：

- 玩家基础数据读写
- 玩家内部数字 ID
- 体力恢复
- 教程事件
- 普通事件 / 事件链
- 互动事件目标记录
- 房间 / 家具独立模块
- 财富等级筛选
- 静态表加载与索引预计算

## 2. 仓库结构

- `IAAClient`
  - Unity 客户端
  - 当前主业务入口在 `Assets/Scripts/UI/MainPageUI.cs` 和 `Assets/Scripts/Game/Player.cs`
- `IAAServer`
  - Go 后端
  - 包含 `svr_gateway`、`svr_login`、`svr_game`、`svr_supervisor`
- `AI_CONTEXT.md`
  - 给 AI 看的一份快速上下文
- `EVENT_SYSTEM_CONTEXT.md`
  - 事件系统的专题说明
- `README.md`
  - 给人读的仓库说明

## 3. 服务端整体理解

### 3.1 服务角色

- `svr_gateway`
  - 统一入口
  - 校验 JWT
  - 维护 login / game 服务注册表
  - 透传 `X-OpenID`
- `svr_login`
  - 微信登录
  - 调 `jscode2session`
  - 签发 JWT
- `svr_game`
  - 游戏业务服务
  - 读写 Mongo
  - 加载静态 CSV
  - 当前最主要的业务代码所在地
- `svr_supervisor`
  - Linux 侧常驻控制平面
  - 管理 release、进程和 `svr_game` 切换发布

### 3.2 当前请求链路

登录链路：

1. 客户端拿微信 `code`
2. 调 gateway `/login`
3. gateway 转发到 `svr_login`
4. `svr_login` 调微信并返回 `openid + JWT`

游戏链路：

1. 客户端带 JWT 调 gateway
2. gateway 校验 JWT
3. gateway 把 `openid` 写到 `X-OpenID`
4. gateway 转发到 `svr_game`
5. `svr_game` 用 `openid` 读写玩家数据

## 4. `svr_game` 当前结构

这部分是当前最值得读的代码。

### 4.1 顶层目录

- `IAAServer/svr_game/main.go`
  - 只负责启动、配置、Mongo、路由和进程生命周期
- `IAAServer/svr_game/httpapi`
  - HTTP transport 层
- `IAAServer/svr_game/staticdata`
  - 静态表 schema、加载、校验、预计算
- `IAAServer/svr_game/game`
  - 游戏业务 service 组装层
- `IAAServer/svr_game/game/model`
  - 共享业务模型
- `IAAServer/svr_game/game/player`
  - 玩家存储、缓存、体力与资源变更
- `IAAServer/svr_game/game/event`
  - 事件引擎、候选池、链推进、教程逻辑
- `IAAServer/svr_game/game/room`
  - 独立的房间 / 家具业务模块

### 4.2 当前代码边界

#### `httpapi`

职责：

- 路由注册
- 请求校验
- 响应编码
- HTTP 状态码映射

当前约定：

- HTTP 错误字段统一使用 `err_msg`
- `err_msg` 类型是 `uint16`
- 服务端错误码定义在 `IAAServer/common/errorcode/`
- 客户端镜像定义在 `IAAClient/Assets/Scripts/Net/Error.cs`
- 两边错误码必须保持一致

不负责：

- Mongo
- 静态表
- 业务规则

#### `staticdata`

职责：

- CSV schema
- 启动期加载
- 必要校验
- 预计算索引

当前重要预计算：

- `Events.RootRows`
- `Events.RootRowsByLevel`
- `Events.TutorialRows`

#### `game/model`

职责：

- 共享类型

当前重要类型：

- `PlayerData`
- `EventID`
- `EventChainState`

当前 `PlayerData` 还承载：

- `player_id`
- `event_history`
- `event_target_player_ids`
- `pirate_incoming_from`

#### `game/player`

职责：

- Mongo 读写
- 写回缓存
- `openid -> player_id` 常驻缓存
- 玩家默认值归一化
- 体力恢复
- 资源变更
- 互动事件目标选择与跨玩家写入
- `insufficient energy` 错误语义

#### `game/event`

职责：

- 教程事件优先逻辑
- 普通候选池构建
- 事件链推进
- `auto_proceed`
- `event_history_delta`
- `target_player_ids`
- 特殊 flag 驱动的互动事件效果

#### `game`

职责：

- 组装 `player.Store`、`event.Engine` 和 `room.Store`
- 对外提供稳定的 service API

现在根 `game` 包已经不是业务实现堆放地，而是 service 入口层。

## 5. 当前业务能力

### 5.1 现有接口

- `GET /debug_val`
- `POST /debug_val_inc`
- `POST /player_data`
- `POST /room_data`
- `POST /trigger_event`
- `POST /upgrade_furniture`

### 5.2 房间 / 家具模块

当前房间系统已经独立于 `PlayerData`：

- 独立模块：`IAAServer/svr_game/game/room`
- 独立存储：`main_collection + "_rooms"`
- 静态表入口：
  - `Data_rooms.csv`
  - `Data_furnitures.csv`
- 对外接口：
  - `POST /room_data`
  - `POST /upgrade_furniture`

如果后续继续开发这一块，优先阅读：

1. `IAAServer/svr_game/game/room/`
2. `IAAServer/svr_game/game/room_service.go`
3. `IAAServer/svr_game/httpapi/handler.go`
4. `IAAServer/svr_game/staticdata/`
### 5.3 玩家状态

当前 `PlayerData` 重要字段：

- `cash`
- `asset`
- `energy`
- `energy_recover_at`
- `shield`
- `tutorial_index`
- `event_history`
- `active_event_chains`

### 5.4 体力系统

由 `params` 中这些字段驱动：

- `start_energy`
- `energy_recover_time`
- `energy_max`

当前行为：

- 新号按 `start_energy` 初始化
- 每次读写玩家时做懒恢复
- 满体时恢复计时暂停
- 扣体力后恢复计时重启

### 5.5 事件系统

当前事件系统是一个 CSV 驱动的加权事件链状态机。

#### 教程事件

- `ForTutorial=true` 的事件按 `ID` 升序触发
- 教程未完成前，不允许触发普通事件
- 教程事件忽略事件链规则
- 教程完成后，教程事件永久不再触发

#### 财富等级限制

- 玩家财富等级由 `assetLevels.csv` 和 `PlayerData.Asset` 动态匹配
- 普通 root 事件只会从满足 `min_level <= 玩家等级 <= max_level` 的桶里选
- 已经进入事件链的 pending 子事件不受等级限制

#### 普通事件选择规则

候选池由两部分组成：

1. 当前财富等级可用的 root 事件
2. 所有 active chain 里的 pending 子事件

补充规则：

- 已经有活跃链的 root 事件，不再进入 root 候选池
- `auto_proceed` 会立刻继续选子事件
- 非 `auto_proceed` 会把子事件写入 `pending_pool`

#### `/trigger_event` 返回

当前成功返回字段：

- `cash`
- `asset`
- `energy`
- `shield`
- `event_history_delta`
- `target_player_ids`

注意：

- `/player_data` 返回完整玩家快照
- `/trigger_event` 返回资源最新值和本次历史增量
- `target_player_ids` 与 `event_history_delta` 一一对应
- 当某一项为 `0` 时，表示这一条事件没有命中合法目标玩家

### 5.6 数字玩家 ID

当前已经引入内部数字玩家 ID：

- `player_id`
  - `uint64`
  - 由 `IAAServer/common/idgen` 的雪花生成器生成
  - 通过 `svr_game/config.json` 里的 `generator_id` 区分生成器实例
- `openid`
  - 继续作为微信登录身份
  - 仍然由 gateway 写入 `X-OpenID`
  - 不再建议作为跨玩家业务引用字段

当前服务端内部约定是：

- 认证链路继续用 `openid`
- 跨玩家引用、事件目标、接口返回优先用 `player_id`
- `player` 模块维护一个不过期的进程内 `openid -> player_id` 映射缓存

### 5.7 互动事件范例：`pirate_incoming`

这是当前“事件节点触发后，对其他玩家产生影响”的最小参考实现。

静态配置前提：

- `Data_events.csv` 的 `flags` 支持 `pirate_incoming`
- `Data_params.csv` 提供 `pirate_target_min_level`
- `Data_params.csv` 提供 `pirate_blocked_event_id`
- `Data_params.csv` 提供 `pirate_hit_event_id`
- `Data_params.csv` 提供 `pirate_hit_cash_loss_percent`

行为约定：

- 触发时从其他玩家里随机选目标
- 目标玩家财富等级必须严格大于 `pirate_target_min_level`
- 财富等级由 `assetLevels.csv` 和 `PlayerData.Asset` 反推
- 如果命中目标：
  - 把触发者 `player_id` 追加到目标玩家的 `pirate_incoming_from`
  - 本次事件对应的目标槽位写入目标 `player_id`
- 如果没命中目标：
  - 不报错
  - 不回滚事件
  - 本次事件对应的目标槽位写 `0`
- 当目标玩家下一次调用 `/player_data` 时：
  - 按 `pirate_incoming_from` 顺序一次性结算全部海盗标记
  - 有 `shield` 就消耗 1 点 `shield`，并把 `pirate_blocked_event_id` 直接写进自己的 `event_history`
  - 没有 `shield` 就按 `pirate_hit_cash_loss_percent` 扣减当前 `cash` 的对应比例，`0.01` 表示 1%，向下取整，最小钳制到 `0`，并把 `pirate_hit_event_id` 直接写进自己的 `event_history`
  - 这些自动事件不走事件引擎，不给原触发者奖励，也不触发额外 flag 或链式逻辑
  - 自动事件的 `event_target_player_ids` 仍然写原触发者 `player_id`
  - 自动结算同样受 `max_event_history` 裁剪

当前通用互动事件数据约定：

- `PlayerData.EventTargetPlayerIDs`
  - 与 `EventHistory` 始终等长
  - 每个事件历史项有一个对应目标槽位
  - 没有目标时记录 `0`
- `/trigger_event.target_player_ids`
  - 与 `event_history_delta` 始终等长
  - 语义和持久化侧一致

## 6. 当前静态表

`svr_game` 启动时会加载：

- `Data_params.csv`
- `Data_items.csv`
- `Data_events.csv`
- `Data_assetLevels.csv`
- `Data_options.csv`
- `Data_rooms.csv`
- `Data_furnitures.csv`

当前重要参数：

- `event_cost`
- `max_event_history`
- `start_energy`
- `energy_recover_time`
- `energy_max`
- `pirate_target_min_level`
- `pirate_blocked_event_id`
- `pirate_hit_event_id`
- `pirate_hit_cash_loss_percent`

## 7. 当前测试状态

`svr_game` 原有的 3 个测试文件已经移除：

- 事件模块测试
- 写回缓存测试
- 静态表加载测试

当前 `svr_game` 目录下已不再保留 `_test.go` 文件。

## 8. 建议阅读顺序

如果要快速接手当前后端主线，建议按这个顺序读：

1. `IAAServer/svr_game/main.go`
2. `IAAServer/svr_game/httpapi/handler.go`
3. `IAAServer/svr_game/game/netHandler.go`
4. `IAAServer/svr_game/game/player_service.go`
5. `IAAServer/svr_game/game/event.go`
6. `IAAServer/svr_game/game/model/types.go`
7. `IAAServer/svr_game/game/player/`
8. `IAAServer/svr_game/game/event/`
9. `IAAServer/svr_game/game/room/`
10. `IAAServer/svr_game/staticdata/`

如果只想看事件系统：

1. `EVENT_SYSTEM_CONTEXT.md`
2. `IAAServer/svr_game/game/event/`
3. `IAAServer/svr_game/game/model/types.go`
4. `IAAServer/svr_game/game/player/mutation.go`
5. `IAAServer/svr_game/staticdata/`

## 9. 当前最重要的事实

- `svr_game` 已经完成一轮明确的职责拆分
- `httpapi` 和 `staticdata` 已经移出 `game` 业务目录
- `game` 内部已经按 `model / player / event` 分层
- 新增的房间 / 家具能力也已按独立 `room` 模块接入
- 根 `game` 包现在只保留 service 组装层
- 事件系统已经支持教程优先、体力恢复、财富等级过滤、事件链和特殊 flag 驱动的互动事件
- 跨玩家业务引用已经开始统一切到 `player_id`
- 当前最重的业务模块仍然是 `event`
# Local Cache / Redis 替代方案

- 当前还没有引入真实 Redis，服务端缓存能力先由 `IAAServer/common/localcache` 提供。
- `localcache.TimedCache`
  - 进程内 TTL KV。
  - 当前用于 `svr_gateway` 的 sticky binding，例如 `openid -> game server id`。
- `localcache.WriteBackCache[T]`
  - 进程内写回缓存。
  - 支持 `Get / StoreLoaded / MutateIfPresent / MutateWithLoaded / FlushNow`。
  - 当前用于 `svr_game/game/player`，缓存玩家数据并定时刷回 Mongo。
- 当前设计前提是：gateway 对 game 走粘滞路由，所以同一玩家通常会命中同一个 `svr_game` 实例，玩家数据缓存不需要分布式化。
- 未来如果引入 Redis，优先迁移的是 gateway 的粘滞路由绑定；`svr_game` 的玩家写回缓存仍然优先保留本地内存设计，而不是直接把 Redis 当成玩家状态主缓存。
