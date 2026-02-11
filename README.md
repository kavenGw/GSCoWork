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

### 1. 构建

```bash
GOOS=linux GOARCH=amd64 go build -o gscowork .
```

### 2. 上传文件

```bash
scp gscowork deploy/gscowork.service deploy/gscowork.sh your-server:/opt/gscowork/
```

### 3. 安装服务

```bash
ssh your-server
cd /opt/gscowork
chmod +x gscowork.sh
sudo ./gscowork.sh install
```

## 服务管理命令

使用 `deploy/gscowork.sh` 脚本管理服务：

```bash
# 安装服务（首次部署）
sudo ./gscowork.sh install

# 启动服务
sudo ./gscowork.sh start

# 停止服务
sudo ./gscowork.sh stop

# 重启服务
sudo ./gscowork.sh restart

# 查看状态
./gscowork.sh status

# 查看实时日志
./gscowork.sh logs

# 查看最近50条日志
./gscowork.sh logs-recent

# 更新程序（重新编译后）
sudo ./gscowork.sh update

# 卸载服务
sudo ./gscowork.sh uninstall
```

### 使用 systemctl 直接管理

```bash
# 启动
sudo systemctl start gscowork

# 停止
sudo systemctl stop gscowork

# 重启
sudo systemctl restart gscowork

# 查看状态
sudo systemctl status gscowork

# 开机自启
sudo systemctl enable gscowork

# 禁用开机自启
sudo systemctl disable gscowork

# 查看日志
sudo journalctl -u gscowork -f
```

## 直接运行（开发测试）

```bash
./gscowork -port 8080 -db data.db
```







## 技术栈

- Go + 标准库 net/http + html/template
- SQLite（modernc.org/sqlite，纯 Go，无 CGO）
- 原生 HTML/CSS/JS
