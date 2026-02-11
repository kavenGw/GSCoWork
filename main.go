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
