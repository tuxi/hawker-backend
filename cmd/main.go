package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hawker-backend/conf"
	"hawker-backend/database"
	"hawker-backend/handlers"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"hawker-backend/services"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {

	// 加载配置
	cfg, err := conf.LoadConfig("conf/config.yaml")
	if err != nil {
		panic(err)
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbPass == "" || dbHost == "" {
		dbUser = cfg.Database.User
		dbPass = cfg.Database.Password
		dbHost = cfg.Database.Host
		dbPort = cfg.Database.Port
		dbName = cfg.Database.Dbname
	}

	// 初始化数据库
	db, err := database.InitDB(dbHost, dbPort, dbUser, dbPass, dbName)
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

	setupAndPrewarmIntros(introRepository, audioService)

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
		// 叫卖任务管理
		v1.POST("/hawking/tasks", productHandler.AddHawkingTaskHandler)          // 添加任务
		v1.DELETE("/hawking/tasks/:id", productHandler.RemoveHawkingTaskHandler) // 移除任务
		v1.GET("/hawking/tasks", productHandler.GetHawkingTasksHandler)
		v1.POST("/hawking/intro", productHandler.SyncIntroHandler)
		//v1.GET("/hawking/intros", productHandler.SyncIntroHandler) // 根据音色和时间点获取到开场白池

		// Category 路由
		v1.POST("/categories", categoryHandler.CreateCategory)
		v1.GET("/categories", categoryHandler.GetAll)

		// 3. 注册 WebSocket 路由
		v1.GET("/ws", func(c *gin.Context) {
			handlers.ServeWs(hub, c)
		})
	}
	_ = r.Run(fmt.Sprintf(":%d", cfg.Server.Port))
}

// 初始化预设模版
func setupAndPrewarmIntros(repo *repositories.MemIntroRepository, audio services.AudioService) {
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
		// 建议在这里通过 audio service 先获取真实的火山 VoiceID
		// 这样如果 mapping 里的 ID 变了，hash 也会变
		realVoiceID := audio.GetRealVoiceID(voice)
		for _, scene := range scenes {
			// 生成指纹：基于文案和真实音色 ID
			fingerprint := generateContentHash(scene.text, realVoiceID)
			// 构造新的存储标识：intros/morning_sunny_boy_a1b2c3d4
			identifier := fmt.Sprintf("intros/%s_%s_%s", scene.tag, voice, fingerprint)

			fullPath := filepath.Join("./static/audio", identifier+".mp3")
			audioURL := "/static/audio/" + identifier + ".mp3"

			// 只有当这个特定“内容+音色”的文件不存在时，才去合成
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				log.Printf("🎙️ 合成新模版: [%s] 音色: %s", scene.text, voice)
				_, err := audio.GenerateAudio(context.Background(), scene.text, identifier, voice)
				if err != nil {
					log.Printf("❌ 预热合成失败: %v", err)
					continue
				}

				// 【可选】这里可以清理旧版本的同场景同音色文件（如果有的话）
				cleanupOldIntros(scene.tag, voice, identifier)
			}

			// 注入内存仓库
			repo.AddTemplate(models.IntroTemplate{
				ID:        scene.id, // 这样 ID 就是 "morning_01", "default_01"
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

// 辅助函数：生成内容哈希
func generateContentHash(text string, voiceID string) string {
	// 建议：如果你能拿到真实的火山 VoiceID（如 bv001），用它更准确
	data := fmt.Sprintf("%s|%s", text, voiceID)
	hash := sha1.Sum([]byte(data))
	return hex.EncodeToString(hash[:8]) // 取前8位即可
}

func cleanupOldIntros(tag, voice, currentIdentifier string) {
	pattern := filepath.Join("static/audio/intros", fmt.Sprintf("%s_%s_*.mp3", tag, voice))
	files, _ := filepath.Glob(pattern)
	for _, f := range files {
		if !strings.Contains(f, currentIdentifier) {
			os.Remove(f)
			log.Printf("🧹 清理旧缓存文件: %s", f)
		}
	}
}
