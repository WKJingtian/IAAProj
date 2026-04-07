# IAA 项目说明

## 项目概览

这是一个 `Unity 微信小游戏客户端 + Go 微服务后端` 的联调仓库。

仓库当前已经不是单纯的基础设施样板，而是包含一条可运行的轻量业务链路：

1. 微信登录
2. gateway 鉴权与转发
3. `svr_game` 按 `openid` 读写 Mongo
4. 客户端拉取玩家数据
5. 客户端触发事件并获得资源变化

目录上主要分成两部分：

- `IAAClient`
  - Unity 客户端
  - 当前重点是登录、网络层、资源加载和轻量玩法验证
- `IAAServer`
  - Go 服务端
  - 包含 `svr_gateway`、`svr_login`、`svr_game`、`svr_supervisor`

## 仓库结构

### 客户端

- `IAAClient/Assets/Scripts`
  - 主要业务代码
- `IAAClient/Assets/Scripts/Net`
  - 登录、会话、网关访问、游戏接口访问
- `IAAClient/Assets/Scripts/Game`
  - 当前玩家状态封装
- `IAAClient/Assets/Scripts/UI`
  - 当前主界面和联调用 UI
- `IAAClient/Assets/Scripts/Utils/YooAssetManager.cs`
  - 资源加载主入口

### 服务端

- `IAAServer/common`
  - 公共库
  - 包括日志、配置、Mongo、服务注册相关逻辑
- `IAAServer/svr_gateway`
  - 统一入口
  - 校验 JWT
  - 转发 login / game 请求
- `IAAServer/svr_login`
  - 微信登录服务
  - 调 `jscode2session`
  - 生成 JWT
- `IAAServer/svr_game`
  - 当前核心业务服务
  - 玩家数据、体力、事件系统都在这里
- `IAAServer/svr_supervisor`
  - Linux 侧常驻控制平面
  - 管理进程、状态和 `svr_game` 切换发布

## 当前请求链路

### 登录链路

1. 客户端调用微信 SDK 获取一次性 `code`
2. 客户端请求 gateway `/login`
3. gateway 转发给 `svr_login`
4. `svr_login` 调微信换取 `openid`
5. `svr_login` 返回 `openid + JWT`

### 游戏链路

1. 客户端后续请求都带 JWT
2. gateway 校验 JWT
3. gateway 将 `openid` 写入 `X-OpenID`
4. gateway 转发给 `svr_game`
5. `svr_game` 按 `openid` 读写玩家数据

## `svr_game` 当前结构

这部分是当前最值得读的代码。

### 顶层目录

- `IAAServer/svr_game/main.go`
  - 负责启动、配置、Mongo、路由注册和进程生命周期
- `IAAServer/svr_game/httpapi`
  - HTTP transport 层
- `IAAServer/svr_game/staticdata`
  - 静态表 schema、加载、校验和预计算
- `IAAServer/svr_game/game`
  - 游戏业务 service 组装层

### `game` 内部分层

- `IAAServer/svr_game/game/model`
  - 共享模型
  - 当前包括 `PlayerData`、`EventID`、`EventChainState`
- `IAAServer/svr_game/game/player`
  - 玩家存储、写回缓存、体力恢复、资源变更
- `IAAServer/svr_game/game/event`
  - 事件引擎、教程事件、候选池、事件链推进
- `IAAServer/svr_game/game/room`
  - 房间与家具的独立业务模块

当前边界已经明确：

- `httpapi` 只负责 HTTP 协议和状态码
- `staticdata` 只负责表和索引
- `player` 只负责玩家状态
- `event` 只负责事件业务
- 根 `game` 包只负责组装 `Service`

## 当前业务接口

`svr_game` 当前对外提供：

- `GET /debug_val`
- `POST /debug_val_inc`
- `POST /player_data`
- `POST /room_data`
- `POST /trigger_event`
- `POST /upgrade_furniture`

其中常用入口：

- `/player_data`
  - 返回完整玩家快照
  - 不存在则创建默认玩家
- `/trigger_event`
  - 消耗体力
  - 触发事件
  - 支持请求体字段 `multiplier`
  - 返回资源最新值、`event_history_delta` 和 `target_player_ids`
- `/room_data`
  - 房间模块入口
- `/upgrade_furniture`
  - 家具升级入口

## 数字玩家 ID 与互动事件范例

当前 `svr_game` 已经引入内部数字玩家 ID：

- `player_id`
  - `uint64`
  - 由 `IAAServer/common/idgen` 的雪花生成器生成
  - 用于跨玩家引用、事件目标记录和接口返回
- `openid`
  - 继续保留为微信登录身份
  - 主要用于 gateway 鉴权和服务端内部读写路径

`svr_game/config.json` 当前还需要提供：

- `generator_id`
  - 雪花生成器实例 ID
  - 语义上独立于 `server_id`
  - 热更新时允许同一个 `server_id` 的新老实例短暂并存，但不应共用同一个 `generator_id`

`player` 模块当前维护一个进程内常驻映射：

- `openid -> player_id`

这个映射没有过期时间，用于减少服务端内部从 `openid` 反查数字 ID 的重复开销。

当前可以把 `pirate_incoming` 当成“互动事件”的最小实现范例。

静态配置前提：

- 在 `Data_events.csv` 的 `flags` 中配置 `pirate_incoming`
- 在 `Data_params.csv` 中提供 `pirate_target_min_level`
- 在 `Data_params.csv` 中提供 `pirate_blocked_event_id`
- 在 `Data_params.csv` 中提供 `pirate_hit_event_id`
- 在 `Data_params.csv` 中提供 `pirate_hit_cash_loss_percent`

模块分工：

- `event`
  - 识别事件节点上的特殊 flag
  - 在事件真正触发后收集这一条事件对应的目标玩家 ID
- `player`
  - 查询符合条件的目标玩家
  - 通过统一的玩家 mutate 路径写回目标玩家数据
- `httpapi`
  - 只负责把这次触发的目标 ID 增量编码到响应里

当前通用互动事件约定：

- `/trigger_event.target_player_ids`
  - 与 `event_history_delta` 一一对应
  - 每条事件历史增量对应一个目标槽位
  - 没有目标时写 `0`
- `PlayerData.event_target_player_ids`
  - 与 `event_history` 始终等长
  - 没有目标时同样记录 `0`
- `0` 是保留值
  - 不表示合法玩家
  - 只表示“这一条事件没有合适目标”

`pirate_incoming` 当前行为：

- 触发时，从“其他玩家”里随机选一个目标
- 目标玩家财富等级必须严格大于 `pirate_target_min_level`
- 财富等级不是单独存库字段，而是由 `assetLevels.csv` 通过 `asset` 反推
- 如果找到目标：
  - 把触发者的 `player_id` 追加到目标玩家的 `pirate_incoming_from`
  - 当前事件对应的目标槽位写入目标 `player_id`
- 如果找不到目标：
  - 不报错
  - 不回滚事件
  - 当前事件对应的目标槽位写 `0`
- 当目标玩家下一次调用 `/player_data` 时：
  - 按 `pirate_incoming_from` 的顺序一次性结算全部海盗标记
  - 有 `shield` 就消耗 1 点 `shield`，并在自己的 `event_history` 里追加 `pirate_blocked_event_id`
  - 没有 `shield` 就按 `pirate_hit_cash_loss_percent` 扣减当前 `cash` 的对应比例，`0.01` 表示 1%，向下取整，最小钳制到 `0`，并在自己的 `event_history` 里追加 `pirate_hit_event_id`
  - 这两种自动事件都只写被攻击者自己的历史，不给原触发者奖励，也不触发额外事件逻辑
  - 自动结算同样要维护 `event_target_player_ids`，目标填原触发者 `player_id`
  - 自动结算同样受 `max_event_history` 限制

## 如何新增业务模块

如果要在当前项目里新增一个业务模块，建议沿用 `svr_game` 现在这套边界：

### 1. 先定协议，再写业务

建议先确定三件事：

1. 客户端请求要带什么
2. 服务端返回什么
3. 这次业务属于 `player`、`event`，还是一个新的独立模块

当前约定：

- JSON 字段使用 `snake_case`
- 错误字段统一使用 `err_msg`
  - 类型是 `uint16`
  - 服务端 `IAAServer/common/errorcode/errorcode.go` 和客户端 `IAAClient/Assets/Scripts/Net/Error.cs` 必须保持一致
- 认证仍然走 gateway，业务服务从 `X-OpenID` 取玩家身份

### 2. 客户端怎么定义请求携带的数据结构

当前客户端现状是：

- 现有 `GameApiClient` 已经同时支持“无请求体”和“带 JSON body”的授权请求
- 身份信息不放在 body 里，而是由 gateway 从 JWT 解析后写入 `X-OpenID`

如果新接口需要客户端主动提交业务参数，建议这样做：

1. 在 `IAAClient/Assets/Scripts/Net/NetType.cs` 新增请求 / 响应结构
2. 在 `IAAClient/Assets/Scripts/Configs/NetAPIs.cs` 新增 API 路径定义
3. 如果接口需要业务参数，就在 `IAAClient/Assets/Scripts/Net/Services/` 下新增对应 service，负责拼请求、发请求、处理回调
4. service 内部通过 `GameApiClient.SendAuthorizedRequestAsync<TResponse>(api, token, requestBody)` 发送 JSON body
5. 如果接口没有业务参数，可以继续传 `null` 或空请求体

推荐约定：

- 请求和响应结构都集中放在 `NetType.cs`
- 业务 service 一接口一文件，和现有 `PlayerDataService.cs`、`TriggerEventService.cs` 保持一致
- 不要把 `openid` 放进请求体，继续走 JWT -> gateway -> `X-OpenID`
- 如果请求体里只有业务参数，就只定义业务字段；身份字段不要重复塞进 body
- JSON 字段名和服务端结构体 tag 保持一致，例如 `multiplier`、`debug_val`

### 3. 服务端怎么新增 HTTP 方法

当前 `svr_game` 的 HTTP 接入顺序建议固定成下面这样：

1. 先在 `IAAServer/svr_game/httpapi/reply.go` 定义返回结构
2. 如果需要公共请求解析逻辑，再补到 `IAAServer/svr_game/httpapi/helper.go`
3. 在 `IAAServer/svr_game/game/` 暴露一个 service 方法，作为 transport 和业务实现之间的稳定入口
4. 具体业务实现放到合适模块：
   - 玩家状态相关放 `game/player`
   - 事件相关放 `game/event`
   - 公共状态模型放 `game/model`
   - 静态配置相关放 `staticdata`
5. 在 `IAAServer/svr_game/httpapi/handler.go` 新增 handler 并注册路由

推荐判断标准：

- 这是“协议层”问题，放 `httpapi`
- 这是“状态持久化 / 缓存 / 资源变更”问题，放 `player`
- 这是“事件规则 / 状态机 / 候选池”问题，放 `event`
- 这是“CSV 表 / 参数 / 索引”问题，放 `staticdata`

一个最小的 HTTP 新增链路，建议按下面这个文件顺序落：

1. 在 `httpapi/reply.go` 增加响应结构；如果需要请求体，也先想清楚请求字段
2. 在 `game/` 根包增加一个对外 service 方法，例如 `func (s *Service) DoSomething(...)`
3. 把具体逻辑落到已有模块：
   - 读写玩家、扣资源、更新缓存，放 `game/player`
   - 事件选择、链推进、奖励结算，放 `game/event`
   - 共享结构，放 `game/model`
   - 读静态表、建索引，放 `staticdata`
4. 回到 `httpapi/helper.go` 写请求解析 helper
5. 回到 `httpapi/handler.go` 写 handler，并在 `RegisterRoutes` 里注册路由

### 4. 服务端怎么新增业务模块

当需求已经明显不属于现有 `player` / `event` 两个域时，再考虑新建模块。判断标准可以用这三条：

1. 这套规则有自己的状态和生命周期，不只是给 `player` 或 `event` 补一个辅助函数
2. 这套规则后续还会继续长，不是一次性的两三个方法
3. 这套规则对外能形成稳定入口，例如未来会有自己的一组 service 方法

当前更推荐的新模块位置是 `IAAServer/svr_game/game/<module_name>/`，而不是继续把实现堆回 `game/` 根目录。

一个最小的新模块可以按下面的方式起：

1. 在 `game/<module_name>/` 下先放真正实现文件，命名按职责拆开，例如 `engine.go`、`store.go`、`state.go`
2. 如果这个模块需要对外暴露状态结构，把跨模块共享的类型放到 `game/model`
3. 在 `game/netHandler.go` 里把新模块组装进 `Service`
4. 在 `game/` 根包新增一层薄的 service 方法，把 `httpapi` 需要的调用收口到这里
5. 如果模块需要静态表支持，把配置结构和预计算索引落到 `staticdata`

建议避免的做法：

- 不要让 `httpapi` 直接调用 `game/player` 或 `game/event` 子包实现细节
- 不要把 Mongo、HTTP、静态表解析揉进同一个文件
- 不要为了“先跑起来”把新业务先塞进 `game/netHandler.go`，后面再拆；现在这层应该只负责组装

### 5. 一个最小新增流程

如果你要加一个新的业务接口，最稳的顺序是：

1. 先补客户端请求 / 响应 DTO
2. 再补 `NetAPIs.cs`
3. 再补客户端 service
4. 然后补服务端 service 方法
5. 再补 `httpapi` handler 和 route
6. 最后再接 UI

这样做的好处是：

- 协议先稳定
- 模块边界不容易写乱
- 后面做 review 时更容易看出“协议层”和“业务层”有没有混在一起

## 房间模块入口

房间 / 家具系统已经接入，但这里不单独展开业务规则。快速定位时看这几个入口就够：

- 服务端模块：`IAAServer/svr_game/game/room/`
- 服务端对外接口：
  - `POST /room_data`
  - `POST /upgrade_furniture`
- 服务端静态表：
  - `Data_rooms.csv`
  - `Data_furnitures.csv`
- 客户端协议与 service：
  - `IAAClient/Assets/Scripts/Net/NetType.cs`
  - `IAAClient/Assets/Scripts/Net/Services/RoomDataService.cs`
  - `IAAClient/Assets/Scripts/Net/Error.cs`

## 当前玩家数据

`PlayerData` 目前重要字段包括：

- `player_id`
- `openid`
- `debug_val`
- `cash`
- `asset`
- `energy`
- `energy_recover_at`
- `shield`
- `tutorial_index`
- `event_history`
- `event_target_player_ids`
- `pirate_incoming_from`
- `active_event_chains`
- `created_at`
- `updated_at`

### 体力系统

当前体力系统由 `params` 中这些字段驱动：

- `start_energy`
- `energy_recover_time`
- `energy_max`

实现方式是懒恢复：

- 新号按 `start_energy` 初始化
- 每次读写玩家数据时结算恢复
- 满体时恢复计时暂停
- 扣体力后恢复计时重启

## 当前事件系统

事件系统是 `svr_game` 里当前最完整的业务模块。

### 事件静态表

主要来自：

- `Data_events.csv`
- `Data_assetLevels.csv`
- `Data_params.csv`

### 当前支持的能力

- 按权重选择事件
- 事件奖励结算
- `event_history` 持久化
- `event_target_player_ids` 持久化
- `active_event_chains` 持久化
- `children_event`
- `options_or_weights`
- `auto_proceed`
- 教程事件优先
- 财富等级过滤 root 事件
- 特殊 flag 驱动的互动事件

### 教程事件规则

- `ForTutorial=true` 的事件按 `ID` 升序触发
- 教程未完成前，不允许触发普通事件
- 教程事件忽略事件链规则
- 教程完成后，教程事件永久不再触发

### 财富等级规则

- 玩家财富等级由 `assetLevels.csv` 和 `PlayerData.Asset` 推导
- 普通 root 事件必须满足：
  - `min_level <= 玩家等级 <= max_level`
- 已进入事件链的 pending 子事件不受等级限制

### 普通事件触发流程

1. 先看玩家是否还有未完成教程
2. 如果有，按教程顺序直接触发教程事件
3. 如果没有，从统一候选池选择普通事件
4. 候选池由两部分组成：
   - 当前财富等级可用的 root 事件
   - 所有 active chain 里的 pending 子事件
5. 消耗 `event_cost`
6. 发奖励
7. 更新 `event_history`
8. 根据 `children_event` 和 `auto_proceed` 推进链状态

### `/trigger_event` 返回

当前成功返回字段：

- `cash`
- `asset`
- `energy`
- `shield`
- `event_history_delta`
- `target_player_ids`

语义上：

- `/player_data` 返回完整快照
- `/trigger_event` 返回本次触发带来的增量结果
- `target_player_ids[i]` 对应 `event_history_delta[i]`
- 当 `target_player_ids[i] == 0` 时，表示这一条事件没有合适目标

当前请求体支持：

- `multiplier`
  - 默认 `1`
  - 当传 `2` 时，体力消耗和奖励数值都会乘 `2`

## 静态表和参数

`svr_game` 启动时会加载并校验：

- `Data_params.csv`
- `Data_items.csv`
- `Data_events.csv`
- `Data_assetLevels.csv`
- `Data_options.csv`
- `Data_rooms.csv`
- `Data_furnitures.csv`

当前重要参数包括：

- `event_cost`
- `max_event_history`
- `start_energy`
- `energy_recover_time`
- `energy_max`
- `pirate_target_min_level`
- `pirate_blocked_event_id`
- `pirate_hit_event_id`
- `pirate_hit_cash_loss_percent`

启动时会预计算这些索引：

- `Events.RootRows`
- `Events.RootRowsByLevel`
- `Events.TutorialRows`

## 客户端现状

客户端现在仍然是轻量玩法验证形态，不是完整正式游戏 UI。

当前主入口：

- `IAAClient/Assets/Scripts/UI/MainPageUI.cs`
- `IAAClient/Assets/Scripts/Game/Player.cs`

目前客户端已经能完成：

- 登录
- 拉取 `player_data`
- 触发 `trigger_event`
- 展示 `cash / asset / energy / shield`
- 展示事件结果文本

## 配置位置

### 客户端

主要写在：

- `IAAClient/Assets/Scripts/Configs/GlobalConfigs.cs`

当前包括：

- 微信 AppID
- gateway 地址
- YooAsset CDN 地址

### 服务端

各服务独立配置：

- `IAAServer/svr_gateway/config.json`
- `IAAServer/svr_login/config.json`
- `IAAServer/svr_game/config.json`
- `IAAServer/svr_game/mongo_config.json`
- `IAAServer/svr_supervisor/config.json`

## 构建与部署

### Windows 构建

入口：

- `IAAServer/build_all_linux.bat`

产物输出到：

- `IAAServer/linux_build/`

### Linux 运维入口

入口脚本：

- `IAAServer/run_all_linux.sh`

支持操作：

- 导入 release
- 启动整套服务
- 发布新的 `svr_game`
- 查看状态
- 停止服务

### 当前发布模型

- `svr_game` 支持 supervisor 管理下的切换发布
- `svr_gateway` 和 `svr_login` 目前仍按整套重启处理

## 当前测试状态

`svr_game` 原本有 3 个测试文件：

- `game/event/event_test.go`
- `game/player/writeback_cache_test.go`
- `staticdata/loader_test.go`

这三份测试现在已经按当前取舍移除，`svr_game` 目录下不再保留 `_test.go` 文件。

## 阅读建议

如果要快速理解当前后端主线，建议按这个顺序读：

1. `AI_CONTEXT.md`
2. `EVENT_SYSTEM_CONTEXT.md`
3. `IAAServer/svr_game/main.go`
4. `IAAServer/svr_game/httpapi/`
5. `IAAServer/svr_game/game/netHandler.go`
6. `IAAServer/svr_game/game/player_service.go`
7. `IAAServer/svr_game/game/event.go`
8. `IAAServer/svr_game/game/model/`
9. `IAAServer/svr_game/game/player/`
10. `IAAServer/svr_game/game/event/`
11. `IAAServer/svr_game/game/room/`
12. `IAAServer/svr_game/staticdata/`

如果只关心当前最重的业务模块，优先看 `svr_game`，不要先钻 `gateway` 或 `supervisor`。
## 通用服务器组件

`IAAServer/common` 当前承载几类可复用的服务器基础组件：

- `common/applog`
  - 统一日志初始化、输出和 panic 捕获
- `common/config`
  - JSON / CSV 配置加载
- `common/mongo`
  - Mongo 客户端初始化、共享连接与断开
- `common/service`
  - 服务注册、心跳、lease 与实例发现
- `common/localcache`
  - 进程内缓存能力
  - `TimedCache` 用于简单 TTL KV 场景，当前用于 `svr_gateway` 的 sticky binding
  - `WriteBackCache[T]` 用于本地写回缓存，当前用于 `svr_game` 的玩家数据缓存

当前阶段还没有引入真实 Redis。现在的取舍是：

- `svr_gateway` 用本地 TTL cache 保存粘滞路由绑定
- `svr_game` 用本地 write-back cache 缓存玩家数据并定时刷回 Mongo
- 等后续需要多 gateway 共享粘滞绑定时，再把路由绑定这一层替换成 Redis；玩家数据缓存不需要因此一起改成分布式缓存
