package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

var (
	port    *int
	dbPath  *string
	pidFile *string
)

func main() {
	port = flag.Int("port", 8080, "监听端口")
	dbPath = flag.String("db", "data.db", "数据库文件路径")
	pidFile = flag.String("pid", "/var/run/gscowork.pid", "PID 文件路径")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "start":
			startDaemon()
			return
		case "stop":
			stopDaemon()
			return
		case "restart":
			stopDaemon()
			startDaemon()
			return
		case "status":
			showStatus()
			return
		case "run":
			// 内部命令，实际运行服务
			runServer()
			return
		default:
			fmt.Printf("未知命令: %s\n", args[0])
			fmt.Println("可用命令: start, stop, restart, status")
			os.Exit(1)
		}
	}

	// 无子命令时直接前台运行（兼容原有用法）
	runServer()
}

func runServer() {
	// 写入 PID 文件
	if err := writePIDFile(); err != nil {
		log.Printf("警告: 无法写入 PID 文件: %v", err)
	}

	initDB(*dbPath)
	initTemplates()

	// 启动 session 清理任务
	startSessionCleanup()

	// 静态资源
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 路由
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleLogin(w, r)
		} else {
			handleLoginPage(w, r)
		}
	})
	http.HandleFunc("/logout", requireLogin(handleLogout))
	http.HandleFunc("/", requireLogin(handleHome))
	http.HandleFunc("/schedule", requireLogin(handleScheduleUpdate))
	http.HandleFunc("/admin", requireAdmin(handleAdminPage))
	http.HandleFunc("/admin/user", requireAdmin(handleCreateUser))
	http.HandleFunc("/admin/user/edit", requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleUpdateUser(w, r)
		} else {
			handleEditUserPage(w, r)
		}
	}))
	http.HandleFunc("/admin/user/delete", requireAdmin(handleDeleteUser))

	// 费用管理路由
	http.HandleFunc("/expense", requireLogin(handleExpensePage))
	http.HandleFunc("/expense/calculate", requireLogin(handleExpenseCalculate))
	http.HandleFunc("/expense/save", requireLogin(handleExpenseSave))
	http.HandleFunc("/expense/history", requireLogin(handleExpenseHistory))
	http.HandleFunc("/expense/detail", requireLogin(handleExpenseDetail))
	http.HandleFunc("/expense/delete", requireAdmin(handleExpenseDelete))
	http.HandleFunc("/expense/user/add", requireAdmin(handleExpenseUserAdd))
	http.HandleFunc("/expense/user/delete", requireAdmin(handleExpenseUserDelete))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("GSCoWork 启动在 http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// writePIDFile 写入当前进程 PID 到文件
func writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(*pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// readPIDFile 读取 PID 文件中的进程号
func readPIDFile() (int, error) {
	data, err := os.ReadFile(*pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// isProcessRunning 检查指定 PID 的进程是否存在
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// 发送信号 0 来检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// startDaemon 在后台启动服务
func startDaemon() {
	// 检查是否已在运行
	if pid, err := readPIDFile(); err == nil {
		if isProcessRunning(pid) {
			fmt.Printf("GSCoWork 已在运行 (PID: %d)\n", pid)
			return
		}
	}

	// 获取当前可执行文件路径
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("无法获取可执行文件路径: %v\n", err)
		os.Exit(1)
	}
	executable, _ = filepath.Abs(executable)

	// 构建启动参数
	args := []string{
		fmt.Sprintf("-port=%d", *port),
		fmt.Sprintf("-db=%s", *dbPath),
		fmt.Sprintf("-pid=%s", *pidFile),
		"run",
	}

	// 创建后台进程
	cmd := exec.Command(executable, args...)
	cmd.Dir = filepath.Dir(executable)

	// 将输出重定向到日志文件
	logFile := filepath.Join(filepath.Dir(*pidFile), "gscowork.log")
	if f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}

	// 设置进程组，使其独立于当前终端
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("GSCoWork 已在后台启动 (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("日志文件: %s\n", logFile)
}

// stopDaemon 停止后台运行的服务
func stopDaemon() {
	pid, err := readPIDFile()
	if err != nil {
		fmt.Println("GSCoWork 未在运行（无法读取 PID 文件）")
		return
	}

	if !isProcessRunning(pid) {
		fmt.Println("GSCoWork 未在运行")
		os.Remove(*pidFile)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("无法找到进程: %v\n", err)
		return
	}

	// 发送 SIGTERM 信号优雅停止
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("停止失败: %v\n", err)
		return
	}

	os.Remove(*pidFile)
	fmt.Printf("GSCoWork 已停止 (PID: %d)\n", pid)
}

// showStatus 显示服务运行状态
func showStatus() {
	pid, err := readPIDFile()
	if err != nil {
		fmt.Println("GSCoWork 状态: 未运行")
		return
	}

	if isProcessRunning(pid) {
		fmt.Printf("GSCoWork 状态: 运行中 (PID: %d)\n", pid)
	} else {
		fmt.Println("GSCoWork 状态: 未运行（进程已退出）")
		os.Remove(*pidFile)
	}
}
