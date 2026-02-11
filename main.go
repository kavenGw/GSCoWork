package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8080, "监听端口")
	dbPath := flag.String("db", "data.db", "数据库文件路径")
	flag.Parse()

	initDB(*dbPath)
	initTemplates()

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

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("GSCoWork 启动在 http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
