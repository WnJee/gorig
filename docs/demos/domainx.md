# domainx / dx 使用指南与场景示例 (Domain & Data Access Guide & Demos)

`domainx` 及 `dx` 是 Gorig 框架提供的流式领域数据访问层（支持 MySQL / MongoDB / SQLite），提供类型安全的链式查询、自动主键生成、事务管理与分页。

---

## 目录
- [1. 实体模型定义 (Model)](#1-实体模型定义-model)
- [2. 链式查询 (Query)](#2-链式查询-query)
- [3. 增删改查 (CRUD)](#3-增删改查-crud)
- [4. 分页查询 (Page / PageData)](#4-分页查询-page--pagedata)
- [5. 事务管理 (Transaction)](#5-事务管理-transaction)
- [6. 批量遍历 (AllEach / FindEach)](#6-批量遍历-alleach--findeach)

---

## 1. 实体模型定义 (Model)

实现 `DConfig()` 即可绑定存储引擎类型、数据库名称及表名：

```go
package model

import "github.com/WnJee/gorig/domainx"

type User struct {
    ID       domainx.ID `json:"id" gorm:"primaryKey"`
    Username string     `json:"username" gorm:"size:64;index"`
    Email    string     `json:"email" gorm:"size:128"`
    Age      int        `json:"age"`
    Status   int        `json:"status" gorm:"default:1"`
    Tags     []string   `json:"tags" gorm:"serializer:json"`
}

// DConfig 声明使用的数据库类型（Mysql / Mongo / Sqlite）、库名和表名
func (u *User) DConfig() (domainx.ConType, string, string) {
    return domainx.Mysql, "main", "users"
}
```

---

## 2. 链式查询 (Query)

使用 `dx.On[T](ctx)` 开启链式查询：

```go
import "github.com/WnJee/gorig/domainx/dx"

// 1. 查询单条记录数据
user, err := dx.On[User](ctx).
    Eq("status", 1).
    Like("username", "john%").
    FirstData()

// 2. 复杂条件与排序查询列表
users, err := dx.On[User](ctx).
    Gte("age", 18).
    In("status", []int{1, 2}).
    Sort("id", false). // false 为倒序 DESC，true 为升序 ASC
    Limit(50).
    FindData()

// 3. 数组/标签字段包含匹配 (Has / HasAny / HasAll)
admins, err := dx.On[User](ctx).
    Has("tags", "admin").
    FindData()

// 4. 统计与聚合
count, _ := dx.On[User](ctx).Eq("status", 1).Count()
exists, _ := dx.On[User](ctx).Eq("email", "test@example.com").Exists()
totalAge, _ := dx.On[User](ctx).Sum("age")
```

---

## 3. 增删改查 (CRUD)

```go
// 1. 新增记录（自动生成雪花算法 ID）
newUser := &User{
    Username: "alice",
    Email:    "alice@example.com",
    Age:      26,
}
id, err := dx.On[User](ctx, newUser).Save()

// 2. 根据主键更新单个字段
err = dx.On[User](ctx).WithID(id).Update("age", 27)

// 3. 批量更新多个字段
err = dx.On[User](ctx).WithID(id).Updates(map[string]any{
    "status": 2,
    "email":  "new_alice@example.com",
})

// 4. 按条件批量更新
err = dx.On[User](ctx).Eq("status", 0).Update("status", 1)

// 5. 删除记录
err = dx.On[User](ctx).WithID(id).Delete()
```

---

## 4. 分页查询 (Page / PageData)

直接返回强类型的 `*load.PageRespT[*T]` 分页响应：

```go
func ListUsers(ctx *gin.Context) {
    defer apix.HandlePanic(ctx)

    pageReq, _ := apix.GetPageReq(ctx)
    keyword, _ := apix.Param[string](ctx, "keyword")

    q := dx.On[User](ctx).Eq("status", 1)
    if keyword != "" {
        q.Like("username", keyword+"%")
    }

    // PageData 直接返回 []*User 切片分页
    resp, err := q.Sort("id", false).PageData(pageReq.Page, pageReq.Size)
    apix.HandlePage(ctx, consts.CurdSelectFailCode, resp, err)
}
```

---

## 5. 事务管理 (Transaction)

```go
err := domainx.Transaction(ctx, func(txCtx context.Context) error {
    // 传入 txCtx，dx 自动使用当前事务上下文
    _, err := dx.On[User](txCtx, user1).Save()
    if err != nil {
        return err
    }

    err = dx.On[Account](txCtx).WithID(user1.ID.Int64()).Update("balance", 1000)
    if err != nil {
        return err
    }

    return nil // 返回 nil 自动提交，返回 error 自动回滚
})
```

---

## 6. 批量遍历 (AllEach / FindEach)

处理超大表时，`AllEach` 自动基于游标分页流式遍历，防止内存溢出（OOM）：

```go
// 流式遍历所有记录，每次读取 1000 条并执行回调
err := dx.On[User](ctx).Eq("status", 1).AllEach(func(item *domainx.Complex[User]) *errors.Error {
    user := item.Data
    fmt.Println("Processing user:", user.ID, user.Username)
    return nil
})
```
