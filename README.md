# GSCoWork

协作办公日历，团队成员可查看彼此日程并标记每天的工作状态。

## 功能

- 账号登录，admin 后台创建用户
- 主页展示所有用户的月历
- 每人可编辑自己日历中的日期状态：默认 / 休息 / 鸡血
- 点击日期格子循环切换状态，无需刷新

## 运行

```bash
go build -o gscowork .
./gscowork
```

访问 `http://localhost:8080`

默认管理员账号：`admin` / `admin123`

### 参数

```
-port 8080    监听端口
-db data.db   数据库文件路径
```

## 部署到 Debian

```bash
GOOS=linux GOARCH=amd64 go build -o gscowork .
scp gscowork your-server:/opt/gscowork/
```

## 服务运行

systemd 服务示例 `/etc/systemd/system/gscowork.service`：

```ini
[Unit]
Description=GSCoWork
After=network.target

[Service]
ExecStart=/opt/gscowork/gscowork -port 8080 -db /opt/gscowork/data.db
WorkingDirectory=/opt/gscowork
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now gscowork
```

## 直接运行

```bash
./gscowork -port 8080 -db /data.db
```







## 技术栈

- Go + 标准库 net/http + html/template
- SQLite（modernc.org/sqlite，纯 Go，无 CGO）
- 原生 HTML/CSS/JS
