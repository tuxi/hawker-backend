package main

import (
	"context"
	"hawker-backend/config"
	"hawker-backend/database"
	"hawker-backend/handlers"
	"hawker-backend/repositories"
	"hawker-backend/services"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {

	// 加载配置
	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		panic(err)
	}

	// 初始化数据库
	db, err := database.InitDB(cfg.Database)
	if err != nil {
		panic(err)
	}

	// 1. 初始化 Repositories
	productRepo := repositories.NewProductRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)

	// 2. 初始化 Handlers (注入 Repo)
	productHandler := handlers.NewProductHandler(productRepo)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)

	// 1. 初始化语音服务
	audioService := services.NewDoubaoAudioService(
		cfg.Volcengine.AppID,
		cfg.Volcengine.AccessToken,
		cfg.Volcengine.ClusterID,
		cfg.Volcengine.VoiceType, // 建议用 "zh_male_shuangkuai_ads" 或 "zh_female_shuangkuai_ads"
		cfg.Server.StaticDir,
	)

	hub := services.GlobalHub

	// 注入调度器
	scheduler := services.NewHawkingScheduler(productRepo, audioService, hub)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	scheduler.Start(ctx)

	// 3. 注册路由
	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		// Product 路由
		v1.POST("/products", productHandler.CreateProduct)
		v1.GET("/products", productHandler.GetProducts)
		v1.PATCH("/products/:id/hawking", productHandler.UpdateHawkingConfig)
		v1.POST("/products/sync", productHandler.SyncProductsHandler)

		// Category 路由
		v1.POST("/categories", categoryHandler.CreateCategory)
		v1.GET("/categories", categoryHandler.GetAll)
	}
	_ = r.Run(":8080")
}
