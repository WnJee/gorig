# cronx 使用指南与场景示例 (Cron & Scheduled Tasks Guide & Demos)

`cronx` 是 Gorig 框架提供的分布式与内存级定时任务引擎，支持标准 Cron 表达式、固定间隔任务、一次性延迟任务以及基于 Redis 的持久化分布式延迟任务（支持服务重启自动恢复与故障租约续期）。

---

## 目录
- [1. 周期性 Cron 任务 (AddCronTask / AddEveryTask)](#1-周期性-cron-任务-addcrontask--addeverytask)
- [2. 一次性延迟任务 (AddDelayTask / AddOnceTask)](#2-一次性延迟任务-adddelaytask--addoncetask)
- [3. 分布式持久化延迟任务 (RegisterPersistTask / AddPersistDelayTask)](#3-分布式持久化延迟任务-registerpersisttask--addpersistdelaytask)
- [4. 任务管理 (ListTasks / RemoveTask)](#4-任务管理-listtasks--removetask)

---

## 1. 周期性 Cron 任务 (AddCronTask / AddEveryTask)

在模块 `init()` 中注册任务即可自动纳入服务生命周期管理：

```go
import "github.com/WnJee/gorig/cronx"

func init() {
    // 1. 标准 Cron 表达式（支持秒级 6 段，超时自动取消 context）
    cronx.AddCronTask("0 0 2 * * ?", func(ctx context.Context) {
        // 每天凌晨 2 点执行数据统计，最长运行 30 分钟
        statsService.DailyAggregate(ctx)
    }, 30*time.Minute)

    // 2. 固定时间间隔周期任务
    cronx.AddEveryTask(5*time.Minute, func(ctx context.Context) {
        // 每 5 分钟刷新一次节点心跳
        nodeService.Heartbeat(ctx)
    }, 10*time.Second)
}
```

---

## 2. 一次性延迟任务 (AddDelayTask / AddOnceTask)

适用于不需要跨进程持久化的高性能内存延迟调度：

```go
func OnUserRegistered(userID int64) {
    // 30 分钟后检查新用户是否完成首日签到
    cronx.AddDelayTask(30*time.Minute, func(ctx context.Context) {
        checkUserFirstSign(ctx, userID)
    })
}
```

---

## 3. 分布式持久化延迟任务 (RegisterPersistTask / AddPersistDelayTask)

针对关键业务（如**未支付订单 15 分钟超时关单**、**会员到期前 3 天提醒**），任务 Payload 会被序列化存入 Redis ZSet。服务重启或多实例部署时，节点会自动抢占到期任务并保证可靠执行。

### 步骤 1：定义 Payload 并启动注册
```go
type OrderTimeoutPayload struct {
    OrderID string `json:"order_id"`
}

func (OrderTimeoutPayload) PersistPayload() {}

func init() {
    // 启动时必须注册任务处理函数（映射函数名与 Payload 类型）
    _ = cronx.RegisterPersistTask(HandleOrderTimeout)
}

func HandleOrderTimeout(ctx context.Context, p OrderTimeoutPayload) error {
    return orderService.CloseIfUnpaid(ctx, p.OrderID)
}
```

### 步骤 2：投递延迟任务
```go
func CreateOrder(order *Order) {
    // 15 分钟后触发关单检查
    taskID, err := cronx.AddPersistDelayTask(
        15*time.Minute,
        HandleOrderTimeout,
        OrderTimeoutPayload{OrderID: order.ID},
    )
}
```

---

## 4. 任务管理 (ListTasks / RemoveTask)

```go
// 1. 获取所有已注册任务信息（下次执行时间、上次执行时间等）
tasks := cronx.ListTasks()
for _, t := range tasks {
    fmt.Printf("Task %s [%s] Next: %v\n", t.Name, t.Spec, t.NextRun)
}

// 2. 按 EntryID 移除任务
cronx.RemoveTask(entryID)

// 3. 按函数名移除任务
cronx.RemoveTaskByName("main.statsService.DailyAggregate")
```
