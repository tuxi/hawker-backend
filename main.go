package main

import (
	"context"
	"fmt"
	"hawker-backend/config"
	"hawker-backend/database"
	"hawker-backend/handlers"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"hawker-backend/services"
	"log"
	"os"
	"path/filepath"

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
	introRepository := repositories.NewMemIntroRepository()

	// 初始化语音服务
	audioService := services.NewDoubaoAudioService(
		cfg.Volcengine.AppID,
		cfg.Volcengine.AccessToken,
		cfg.Volcengine.ClusterID,
		//cfg.Volcengine.VoiceType, // 建议用 "zh_male_shuangkuai_ads" 或 "zh_female_shuangkuai_ads"
		cfg.Server.StaticDir,
	)

	hub := services.NewHub()
	go hub.Run()

	// 注入调度器
	scheduler := services.NewHawkingScheduler(productRepo, introRepository, audioService, hub)

	// 初始化 Handlers (注入 Repo)
	productHandler := handlers.NewProductHandler(productRepo, scheduler)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	scheduler.Start(context.Background())

	SetupAndPrewarmIntros(introRepository, audioService)

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
		v1.GET("/hawking/tasks", productHandler.GetHawkingTasksHandler)

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

// 初始化预设模版
func SetupAndPrewarmIntros(repo *repositories.MemIntroRepository, audio services.AudioService) {
	// 定义我们支持的音色
	// sunny_boy: 阳光男声, soft_girl: 亲切女声
	voices := []string{models.VoiceSunnyBoy, models.VoiceSoftGirl, models.VoicePromoBoss, models.VoiceSweetGirl}

	// 定义叫卖时段和文案
	scenes := []struct {
		id     string
		tag    string
		text   string
		trange [2]int
	}{
		{"morning_01", "morning", "大家早上好！新鲜肉菜刚刚到货，快来选购吧！", [2]int{6, 11}},
		{"noon_01", "noon", "中午好，辛苦忙碌半天，买点好菜犒劳一下家人吧！", [2]int{11, 14}},
		{"evening_01", "evening", "晚市大促销开始啦，新鲜不隔夜，卖完就收摊！", [2]int{17, 21}},
		{"default_01", "default", "走过路过不要错过，咱家生鲜，品质看得见！", [2]int{0, 24}},
	}

	log.Println("🛠️ 正在检查并预热开场白音频资源...")

	for _, voice := range voices {
		for _, scene := range scenes {
			// 构造存储标识符，注意带上 intros/ 前缀
			identifier := fmt.Sprintf("intros/%s_%s", scene.tag, voice)

			// 尝试预合成（GenerateAudio 内部会处理目录创建和重复跳过）
			// 我们在外部先检查一下，如果文件已存在，就不调 API 浪费钱
			fullPath := filepath.Join("./static/audio", identifier+".mp3")
			audioURL := "/static/audio/" + identifier + ".mp3"

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				log.Printf("🎙️ 合成新模版: [%s] 音色: %s", scene.text, voice)
				_, err := audio.GenerateAudio(context.Background(), scene.text, identifier, voice)
				if err != nil {
					log.Printf("❌ 预热合成失败: %v", err)
					continue
				}
			}

			// 注入内存仓库
			repo.AddTemplate(models.IntroTemplate{
				ID:        scene.id,
				Text:      scene.text,
				VoiceType: voice,
				SceneTag:  scene.tag,
				TimeRange: scene.trange,
				AudioURL:  audioURL,
			})
		}
	}
	log.Println("✅ 开场白资源预热完成")
}
