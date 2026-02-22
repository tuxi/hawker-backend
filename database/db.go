package database

import (
	"fmt"
	"hawker-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDB(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", host, user, password, dbname, port, "disable")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// 可以在这里关闭外键约束检查（如果迁移遇到循环依赖报错的话）
		// DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: " + err.Error())
	}

	// 先确保扩展开启
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")

	// 自动迁移表结构
	err = db.AutoMigrate(
		&models.Owner{},
		&models.Store{},
		&models.Category{},
		&models.Product{},
		&models.SalesRecord{},
		&models.ProductDependency{},
	)
	if err != nil {
		return nil, fmt.Errorf("数据库迁移失败: " + err.Error())
	}
	fmt.Println("✅ 数据库初始化完成，表结构已就绪")
	return db, nil
}
