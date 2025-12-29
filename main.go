package main

import (
	"hawker-backend/database"
	"hawker-backend/handlers"
	"hawker-backend/repositories"
	"hawker-backend/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化数据库
	db, err := database.InitDB("127.0.0.1", 5432, "postgres", "postgres", "hawker_db")
	if err != nil {
		panic(err)
	}

	// 2. 按照依赖链注入
	productRepo := repositories.NewProductRepository(db)
	productHandler := handlers.NewProductHandler(productRepo)

	// 1. 初始化语音服务
	audioService := services.NewDoubaoAudioService(appID, accessToken, clusterID)

	// 2. 注入调度器
	scheduler := services.NewHawkingScheduler(productRepo, audioService, globalHub)

	// 3. 注册路由
	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		v1.POST("/products", productHandler.CreateProduct)
		v1.GET("/products", productHandler.GetProducts)
		v1.PATCH("/products/:id/hawking", productHandler.UpdateHawkingConfig)
	}
	_ = r.Run(":8080")
}
