# messagex 使用指南与场景示例 (Event Bus & Pub/Sub Guide & Demos)

`messagex` 是 Gorig 框架内置的发布/订阅（Pub/Sub）消息总线，支持本地进程内异步消费、Redis 跨节点发布订阅、顺序消费（Seq）与死信队列重放（DLQ）。

---

## 目录
- [1. 泛型强类型事件发布与订阅 (PublishEvent / SubscribeEvent)](#1-泛型强类型事件发布与订阅-publishevent--subscribeevent)
- [2. 顺序消费订阅 (RegisterTopicSeq)](#2-顺序消费订阅-registertopicseq)
- [3. 死信队列重放 (ReplayDLQ)](#3-死信队列重放-replaydlq)

---

## 1. 泛型强类型事件发布与订阅 (PublishEvent / SubscribeEvent)

### 步骤 1：定义事件模型
```go
type OrderPaidEvent struct {
    OrderID string  `json:"order_id"`
    Amount  float64 `json:"amount"`
    UserID  string  `json:"user_id"`
}
```

### 步骤 2：在模块初始化时订阅事件
```go
import "github.com/WnJee/gorig/mid/messagex"

func init() {
    _, _ = messagex.SubscribeEvent("order.paid", func(ctx context.Context, event *OrderPaidEvent) error {
        // 自动将消息解析为 OrderPaidEvent 结构体
        logger.Info(ctx, "Handling order paid event", zap.String("order_id", event.OrderID))

        // 1. 发送通知邮件 / 微信提醒
        // 2. 增加用户积分
        return nil
    })
}
```

### 步骤 3：在业务逻辑中发布事件
```go
func OnPaymentSuccess(ctx context.Context, order *Order) {
    // 异步投递事件，解耦主业务流程
    _ = messagex.PublishEvent(ctx, "order.paid", OrderPaidEvent{
        OrderID: order.ID,
        Amount:  order.TotalAmount,
        UserID:  order.UserID,
    })
}
```

---

## 2. 顺序消费订阅 (RegisterTopicSeq)

针对严格要求按顺序消费的场景（如账户金额连续变动、工单状态流转）：

```go
func init() {
    _, _ = messagex.RegisterTopicSeq("account.flow", func(msg *messagex.Message) *errors.Error {
        // 顺序同步执行，上一条处理完成才会消费下一条
        accountID := msg.GetValueStr("account_id")
        delta := msg.GetValueFloat64("delta")

        return processAccountChange(accountID, delta)
    })
}
```

---

## 3. 死信队列重放 (ReplayDLQ)

当消费者反复重试失败达到上限时，消息会自动转入 DLQ（死信队列），排查并修复问题后可手动重放：

```go
// 将 order.paid 主题下最多 100 条死信消息重新放回就绪队列进行重试消费
err := messagex.ReplayDLQ("order.paid", 100)
```
