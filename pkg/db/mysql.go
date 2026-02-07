package db

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	// 这里直接用你报错信息里显示的正确路径
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
)

// DB 全局数据库对象
var DB *gorm.DB

func InitMySQL() {
	// 1. 配置 DSN (Data Source Name)
	// 账号:root, 密码:root, 端口:3306, 库名:gsp_db
	dsn := "root:root@tcp(127.0.0.1:3306)/gsp_db?charset=utf8mb4&parseTime=True&loc=Local"

	var err error
	// 2. 连接数据库
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败: ", err)
	}

	// 3. 自动迁移 (Auto Migrate)
	// 这一步会自动在数据库创建 tasks 表
	err = DB.AutoMigrate(&model.Task{})
	if err != nil {
		log.Fatal("建表失败: ", err)
	}

	fmt.Println("✅ MySQL 连接成功，表结构已同步！")
}
