package main

import (
	"context"
	"hawker-backend/config"
	"hawker-backend/database"
	"hawker-backend/handlers"
	"hawker-backend/repositories"
	"hawker-backend/services"

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

	//  初始化 Repositories
	productRepo := repositories.NewProductRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)

	// 初始化语音服务
	audioService := services.NewDoubaoAudioService(
		cfg.Volcengine.AppID,
		cfg.Volcengine.AccessToken,
		cfg.Volcengine.ClusterID,
		cfg.Volcengine.VoiceType, // 建议用 "zh_male_shuangkuai_ads" 或 "zh_female_shuangkuai_ads"
		cfg.Server.StaticDir,
	)

	hub := services.NewHub()
	go hub.Run()

	// 注入调度器
	scheduler := services.NewHawkingScheduler(productRepo, audioService, hub)

	// 初始化 Handlers (注入 Repo)
	productHandler := handlers.NewProductHandler(productRepo, scheduler)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	
	scheduler.Start(context.Background())

	// 3. 注册路由
	r := gin.Default()
	r.Static("/static", "./static")
	v1 := r.Group("/api/v1")
	{
		// Product 路由
		v1.POST("/products", productHandler.CreateProduct)
		v1.GET("/products", productHandler.GetProducts)
		//v1.PATCH("/products/:id/hawking", productHandler.UpdateHawkingConfig)
		v1.POST("/products/sync", productHandler.SyncProductsHandler)
		v1.POST("/hawking/start", productHandler.StartHawkingHandler)
		// 叫卖任务管理
		v1.POST("/hawking/tasks", productHandler.AddHawkingTaskHandler)          // 添加任务
		v1.DELETE("/hawking/tasks/:id", productHandler.RemoveHawkingTaskHandler) // 移除任务

		// Category 路由
		v1.POST("/categories", categoryHandler.CreateCategory)
		v1.GET("/categories", categoryHandler.GetAll)

		// 3. 注册 WebSocket 路由
		v1.GET("/ws", func(c *gin.Context) {
			handlers.ServeWs(hub, c)
		})
	}
	_ = r.Run(":8080")
}
