# storage 使用指南与场景示例 (Unified Storage Guide & Demos)

`storage` 是 Gorig 框架的对象存储统一抽象层，一套代码无缝兼容本地磁盘（Local）、AWS S3、阿里云 OSS 与 MinIO，并支持分片上传、断点续传、文件下载与临时预签名 URL（Presigned URL）。

> [!TIP] **轻量零冗余依赖设计 (Zero Heavy Vendor SDK Bloat)**
> Gorig `storage` 采用纯标准库（`net/http` + `crypto` + `encoding/xml`）基于标准 REST API 与签名规范原生实现 OSS 与 S3/MinIO 协议，**完全不引入庞大的第三方厂商 SDK**（如 AWS SDK、Aliyun OSS SDK、MinIO SDK），避免了引入数十个间接依赖包，极大减小了二进制构建体积并提升编译速度。

---

## 目录
- [1. 快速上传 (PutBytes / SaveString / PutFile)](#1-快速上传-putbytes--savestring--putfile)
- [2. 文件下载与读取 (GetBytes / ReadString / DownloadToFile)](#2-文件下载与读取-getbytes--readstring--downloadtofile)
- [3. 生成临时访问与上传 URL (GetURL)](#3-生成临时访问与上传-url-geturl)
- [4. 断点续传大文件 (UploadFileResumable)](#4-断点续传大文件-uploadfileresumable)
- [5. 对象元数据与删除 (Stat / Delete / Exists)](#5-对象元数据与删除-stat--delete--exists)

---

## 1. 快速上传 (PutBytes / SaveString / PutFile)

```go
import "github.com/WnJee/gorig/storage"

// 1. 保存文本内容或 JSON 字符串
result, err := storage.SaveString(ctx, "logs/2026-09-24.log", "App started successfully")

// 2. 上传字节切片（如图片数据）
result, err = storage.PutBytes(ctx, "avatars/u1001.png", imgBytes, "image/png")

// 3. 上传本地文件
result, err = storage.PutFile(ctx, "backup/db.tar.gz", "/tmp/backup.tar.gz")
```

---

## 2. 文件下载与读取 (GetBytes / ReadString / DownloadToFile)

```go
// 1. 读取文本内容
content, err := storage.ReadString(ctx, "configs/banner.json")

// 2. 读取二进制字节
data, err := storage.GetBytes(ctx, "avatars/u1001.png")

// 3. 直接下载并写入本地文件
err = storage.DownloadToFile(ctx, "backup/db.tar.gz", "/var/data/restore.tar.gz")
```

---

## 3. 生成临时访问与上传 URL (GetURL)

对于私有 Bucket，可生成带时效签名的安全下载链接：

```go
// 生成 1 小时内有效的临时访问链接
downloadURL, err := storage.GetURL(ctx, "private/reports/q3.xlsx", 1*time.Hour)
```

---

## 4. 断点续传大文件 (UploadFileResumable)

上传 GB 级超大文件时，支持多协程分片并发上传与断点恢复：

```go
res, err := storage.UploadFileResumable(
    ctx,
    "videos/course-01.mp4",
    "/data/large_video.mp4",
    &storage.ResumableOptions{
        PartSize:    10 * 1024 * 1024, // 10MB 分片
        Concurrency: 5,                // 5 协程并发
        Progress: func(uploaded, total int64) {
            fmt.Printf("Upload progress: %.2f%%\n", float64(uploaded)/float64(total)*100)
        },
    },
)
```

---

## 5. 对象元数据与删除 (Stat / Delete / Exists)

```go
// 检查对象是否存在
exists, _ := storage.Exists(ctx, "avatars/u1001.png")

// 获取文件大小与元信息
info, err := storage.Stat(ctx, "videos/course-01.mp4")
if err == nil {
    fmt.Printf("File size: %d, ETag: %s\n", info.Size, info.ETag)
}

// 删除单个文件
_ = storage.Delete(ctx, "avatars/u1001.png")

// 批量删除多个文件
_ = storage.DeleteMulti(ctx, []string{"tmp/1.txt", "tmp/2.txt"})
```
