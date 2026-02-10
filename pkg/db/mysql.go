package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitMySQL() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		"root",
		"root",
		os.Getenv("DB_HOST"),
		"3306",
		os.Getenv("MYSQL_DATABASE"),
	)

	// 本地开发兜底
	if os.Getenv("DB_HOST") == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/cloud_compute?charset=utf8mb4&parseTime=True&loc=Local"
	}

	// Key Point 2: 必须先定义 err
	var err error

	for i := 0; i < 10; i++ {
		// Key Point 3: 这里必须用 "=" 而不是 ":=" !!!
		// "=" 表示：给外面那个全局变量 DB 赋值
		// ":=" 表示：创建一个新的局部变量 DB (这会导致全局 DB 永远是 nil，程序会崩)
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})

		if err == nil {
			break
		}
		log.Printf("⚠️ 连接数据库失败，等待 2 秒重试... (%d/10)", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("❌ 无法连接数据库: %v", err)
	}

	log.Println("✅ 数据库连接成功!")
}
