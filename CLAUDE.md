# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

GSCoWork 是一个协作办公日历 Web 应用，团队成员可查看彼此日程并标记每天的工作状态（默认/休息/🐮🐴）。

## 构建与运行

```bash
# 构建
go build -o gscowork .

# 运行（默认 :8080，数据库 data.db）
./gscowork

# 自定义参数
./gscowork -port 9090 -db /path/to/data.db

# 后台运行（守护进程模式）
sudo ./gscowork start              # 后台启动
sudo ./gscowork stop               # 停止服务
sudo ./gscowork restart            # 重启服务
sudo ./gscowork status             # 查看运行状态

# 自定义 PID 文件和端口
sudo ./gscowork -port 9090 -pid /var/run/gscowork.pid start

# 交叉编译到 Linux
GOOS=linux GOARCH=amd64 go build -o gscowork .
```

默认管理员账号：`admin` / `admin123`（首次启动自动创建）

## 技术栈

- Go 标准库 `net/http` + `html/template`（无第三方 Web 框架）
- SQLite via `modernc.org/sqlite`（纯 Go，无 CGO 依赖）
- 密码哈希：`golang.org/x/crypto/bcrypt`
- 前端：原生 HTML/CSS/JS，模板渲染

## 架构

单 package `main`，按职责分文件：

| 文件 | 职责 |
|------|------|
| `main.go` | 入口、路由注册、静态资源 |
| `auth.go` | Session 管理（内存 map）、`requireLogin`/`requireAdmin` 中间件、密码校验 |
| `handler.go` | 所有 HTTP handler + 模板初始化 + 日历网格构建逻辑 |
| `db.go` | SQLite 初始化、建表、CRUD 操作 |
| `model.go` | `User`、`Schedule` 结构体及状态常量 |

关键设计：
- 认证基于内存 Session map（重启清除，可接受），cookie 设置 HttpOnly + SameSite=Strict
- 日程状态通过 fetch POST `/schedule` 循环切换，返回 JSON
- 月份切换通过 URL 参数 `?month=2026-02`
- `schedules` 表对 `(user_id, date)` 有唯一索引，用 `ON CONFLICT DO UPDATE` 实现 upsert

## 路由

| 方法 | 路径 | 权限 |
|------|------|------|
| GET/POST | `/login` | 公开 |
| GET | `/logout` | 登录用户 |
| GET | `/` | 登录用户（主页日历） |
| POST | `/schedule` | 登录用户（仅操作自己） |
| GET | `/admin` | 仅 admin |
| POST | `/admin/user` | 仅 admin |

## 数据库

SQLite，两张表：`users` 和 `schedules`。首次启动自动建表。状态值：1=默认，2=休息，3=🐮🐴（见 `model.go` 常量）。

