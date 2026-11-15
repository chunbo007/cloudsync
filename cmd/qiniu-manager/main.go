package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"qiniu-site-manager/internal/database"
)

func main() {
	dbPath := flag.String("db", "qiniu-manager.db", "SQLite 数据库文件路径")
	flag.Parse()

	absoluteDBPath, err := filepath.Abs(*dbPath)
	if err != nil {
		log.Fatalf("解析数据库路径失败：%v", err)
	}

	if err := os.MkdirAll(filepath.Dir(absoluteDBPath), 0o755); err != nil {
		log.Fatalf("创建数据库目录失败：%v", err)
	}

	db, err := database.Open(absoluteDBPath)
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		log.Fatalf("初始化数据库失败：%v", err)
	}

	fmt.Println("七牛静态网站管理工具核心模块已启动")
	fmt.Printf("数据库：%s\n", absoluteDBPath)
}
