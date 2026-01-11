package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"hawker-backend/logic"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 建议的消息结构
type PlayEventData struct {
	ProductID string               `json:"product_id"`
	Intro     *models.HawkingIntro `json:"intro,omitempty"` // 独立开场白对象
	Product   *models.HawkingTask  `json:"product"`         // 商品叫卖任务
	VoiceType string               `json:"voice_type"`      // 全局同步音色
}

type HawkingScheduler struct {
	productRepo  repositories.ProductRepository
	introRepo    repositories.IntroRepository // 👈 新增：开场白仓库
	audioService AudioService
	Hub          *Hub
	IsRunning    int32 // 使用原子操作标记

	ActiveTasks  map[string]*models.HawkingTask
	taskMutex    sync.RWMutex // 使用读写锁提高并发性能
	currentIntro *models.HawkingIntro

	taskNotify chan struct{} //用于通知新任务到达
}

func NewHawkingScheduler(repo repositories.ProductRepository, introRepo repositories.IntroRepository, audio AudioService, hub *Hub) *HawkingScheduler {
	return &HawkingScheduler{
		productRepo:  repo,
		introRepo:    introRepo,
		audioService: audio,
		Hub:          hub,
		ActiveTasks:  make(map[string]*models.HawkingTask),
		taskNotify:   make(chan struct{}, 1), // 缓冲大小设置为1即可
	}
}

func (s *HawkingScheduler) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&s.IsRunning, 0, 1) {
		return
	}

	go func() {
		// 1. 增加异常恢复，防止协程挂掉
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ 叫卖引擎崩溃重燃: %v", r)
				atomic.StoreInt32(&s.IsRunning, 0)
				s.Start(ctx) // 尝试重启
			}
		}()

		log.Printf("🚀 叫卖引擎启动 [地址:%p]", s)

		for {
			// --- A. 获取任务列表 ---
			s.taskMutex.RLock()
			// 找出所有活跃，且【尚未合成】的任务
			var pendingIDs []string
			for id, task := range s.ActiveTasks {
				if !task.IsSynthesized {
					pendingIDs = append(pendingIDs, id)
				}
			}
			s.taskMutex.RUnlock()

			// --- B. 没活干就死等信号 ---
			if len(pendingIDs) == 0 {
				select {
				case <-ctx.Done():
					log.Println("🔔 收到ctx.Done 信号")
					return
				case <-s.taskNotify:
					log.Println("🔔 收到唤醒信号")
					continue // 重新回到顶部拿任务
				}
			}

			// --- C. 有活干，逐个处理 ---
			for _, id := range pendingIDs {
				s.taskMutex.RLock()
				task, ok := s.ActiveTasks[id]
				s.taskMutex.RUnlock()
				if !ok {
					continue
				}

				product, err := s.productRepo.FindByID(id)
				if err != nil {
					s.RemoveTask(id) // 找不到商品才真正移除
					continue
				}

				// 执行合成
				log.Printf("🎙️ 合成新任务: %s", product.Name)

				// 如果这个函数不返回，下面的信号监听永远不生效
				audioURL, script, err := s.executeHawking(ctx, product, task)
				if err != nil {
					s.RemoveTask(id)
					continue
				}

				// 合成完后，不 Remove，只标记为已合成
				s.taskMutex.Lock()
				if t, ok := s.ActiveTasks[id]; ok {
					t.IsSynthesized = true
					t.AudioURL = audioURL
					t.Text = script
					s.ActiveTasks[id] = t
				}
				currentTask := s.ActiveTasks[id] // 获取最新指针
				intro := s.getOrRefreshIntro(currentTask)

				s.taskMutex.Unlock()

				s.Hub.BroadcastTaskBundle(s.GetActiveTasksSnapshot())

				log.Printf("📡 正在通过 WebSocket 广播指令...")
				s.broadcastPlayEvent(product, currentTask, intro) // 仅发送当前正在处理的这一个
				log.Printf("🎉 广播已发出，等待 App 播放")

				// 休息10秒，随时响应退出
				sleepTime := 10
				if product.IntervalSec > 0 {
					sleepTime = product.IntervalSec
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(sleepTime) * time.Second):
				}
			}

			// 处理完一波，清空多余信号
			select {
			case <-s.taskNotify:
			default:
			}
		}
	}()
}

func (s *HawkingScheduler) getOrRefreshIntro(task *models.HawkingTask) *models.HawkingIntro {
	now := time.Now().Hour()

	// 逻辑：如果 intro 为空，或者当前小时超出了当前 intro 的适用范围，则刷新
	// 假设 HawkingIntro 结构体里带了 StartHour 和 EndHour
	if s.currentIntro == nil ||
		now < s.currentIntro.StartHour ||
		now >= s.currentIntro.EndHour ||
		s.currentIntro.VoiceType != task.VoiceType { // 💡 别忘了音色也要匹配

		log.Printf("🔄 正在刷新开场白 (当前小时: %d)", now)
		s.currentIntro = s.getIntroTask(task)
	}

	return s.currentIntro
}

// executeHawking 封装具体的执行步骤，保持 Start 方法简洁
func (s *HawkingScheduler) executeHawking(ctx context.Context, p *models.Product, task *models.HawkingTask) (audioURL string, script string, err error) {
	if task == nil {
		return
	}

	// 1. 生成文案
	script = task.Text
	if len(task.Text) == 0 {
		script = logic.GenerateSmartScript(*p, task)
		log.Printf("📝 为 [%s] 生成文案: %s", p.Name, script)
	}

	// 2. 计算当前文案的哈希值
	currentHash := fmt.Sprintf("%x", md5.Sum([]byte(script)))
	// 取 Hash 的前 8 位作为后缀即可，既保证唯一性又让文件名不太长
	shortHash := currentHash[:8]
	// 新的文件名格式：ProductID_ShortHash.mp3
	// 🌟 文件名哈希中也建议加入音色 ID，防止同文案不同音色覆盖
	newFileName := fmt.Sprintf("%s_%s_%s", p.ID.String(), task.VoiceType, shortHash)

	// 3. 缓存校验
	// 如果文案没变，且对应的音频文件确实存在于磁盘上
	if s.checkAudioExists(newFileName) {
		audioURL = fmt.Sprintf("/static/audio/%s.mp3", newFileName)
		log.Printf("♻️ 文案未变，复用缓存音频: %s", p.Name)
	} else {
		// 4. 文案变了或文件丢失，调用火山引擎合成
		log.Printf("🎙️ 文案已更新，正在调用火山引擎合成音频: %s", p.Name)
		audioURL, err = s.audioService.GenerateAudio(ctx, script, newFileName, task.VoiceType)
		if err != nil {
			log.Printf("❌ 语音合成失败 [%s]: %v", p.Name, err)
			s.productRepo.UpdateHawkingStatus(p.ID.String(), map[string]interface{}{"hawking_status": "idle"})
			return
		}

		log.Printf("✅ 音频合成成功! 文件路径: %s", audioURL) // 👈 新增：确认合成完成

		// 5. 【可选】清理旧版本的音频文件
		// 为了防止磁盘被同一个商品的各种历史版本占满，可以异步删掉该商品旧 Hash 的文件
		go s.cleanupOldVersions(p.ID.String(), newFileName)
	}

	// 更新哈希值准备存入数据库
	p.LastScriptHash = currentHash

	updates := map[string]interface{}{
		"last_script_hash": p.LastScriptHash,
		"last_hawked_at":   time.Now(),
		"priority":         0,
		"hawking_status":   "idle",
	}
	s.productRepo.UpdateHawkingStatus(p.ID.String(), updates)
	return
}

// 辅助方法：检查本地文件是否还在（防止被手动删了）
func (s *HawkingScheduler) checkAudioExists(identifier string) bool {
	filePath := filepath.Join("./static/audio", identifier+".mp3")
	_, err := os.Stat(filePath)
	return err == nil
}

func (s *HawkingScheduler) AddTask(product *models.Product, req models.AddTaskReq) {
	// 2. 确定最终文案：如果前端传了就用前端的，否则用数据库里的描述
	finalText := req.Text
	scene := "default"
	if finalText != "" {
		scene = "custom"
	} else if req.Price > 0 {
		scene = "price_promo"
	}
	s.taskMutex.Lock()
	key := strings.ToLower(product.ID.String())
	s.ActiveTasks[key] = &models.HawkingTask{
		ProductID:     req.ProductID,
		Text:          req.Text,
		Price:         req.Price,
		OriginalPrice: req.OriginalPrice,
		Unit:          req.Unit, // 👈 保存单位
		MinQty:        req.MinQty,
		ConditionUnit: req.ConditionUnit,
		VoiceType:     req.VoiceType,
		Scene:         scene,
		IsSynthesized: false, // 每次添加或更新，都重置为 false 以触发重新合成
	}
	s.taskMutex.Unlock()

	// 触发信号唤醒 Start 中的 for 循环
	select {
	case s.taskNotify <- struct{}{}:
		log.Println("✅ 信号发送成功")
	default:
		log.Println("⚠️ 信号队列已满，说明已有任务在排队")
	}
}
func (s *HawkingScheduler) RemoveTask(productID string) {
	key := strings.ToLower(productID)
	s.taskMutex.Lock()
	delete(s.ActiveTasks, key)
	s.taskMutex.Unlock()
}

// 获取当前所有任务的快照（用于推送给 Swift）
func (s *HawkingScheduler) GetActiveTasksSnapshot() *models.TasksSnapshotData {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	var list = make([]*models.HawkingTask, 0) // 即使为空也返回 [] 而不是 nil
	for _, task := range s.ActiveTasks {
		list = append(list, task)
	}
	return &models.TasksSnapshotData{
		Intro:    s.currentIntro,
		Products: list,
	}
}

// 场景 A：全量同步 (配置更新)
func (s *HawkingScheduler) broadcastConfig() {
	payload := models.WSMessage{
		Type: "TASK_CONF_UPDATE",
		Data: s.GetActiveTasksSnapshot(), // 返回 []HawkingTask
	}
	s.Hub.Broadcast(payload)
}

// 场景 B：单次播放指令
func (s *HawkingScheduler) broadcastPlayEvent(p *models.Product, task *models.HawkingTask, intro *models.HawkingIntro) {

	data := PlayEventData{
		ProductID: p.ID.String(),
		Intro:     intro,
		Product:   task,
		VoiceType: task.VoiceType,
	}
	payload := models.WSMessage{
		Type: "HAWKING_PLAY_EVENT",
		Data: data,
	}
	s.Hub.Broadcast(payload)
}
func (s *HawkingScheduler) cleanupOldVersions(productID string, currentFullFileName string) {
	// 查找 static/audio/ 目录下所有以 productID 开头但不是 currentFullFileName 的文件并删除
	files, _ := filepath.Glob(filepath.Join("static/audio", productID+"_*.mp3"))
	for _, f := range files {
		if !strings.Contains(f, currentFullFileName) {
			os.Remove(f)
		}
	}
}

// 辅助方法：匹配逻辑
func (s *HawkingScheduler) getIntroTask(task *models.HawkingTask) *models.HawkingIntro {
	// 逻辑核心：必须传入 task.VoiceType
	// 确保开场白与商品语音的人声完全统一

	// 1. 获取当前小时
	hour := time.Now().Hour()

	// 2. 从仓库查找：匹配 (时间段 + 任务指定的音色)
	template := s.introRepo.FindByTime(hour, task.VoiceType)
	if template == nil {
		// 3. 如果该音色没有对应时段的开场白，回退到默认
		template = s.introRepo.FindByID("default_01", task.VoiceType)
		fmt.Printf("模版中没有对应时段的开场白，使用默认开场白音频：%s", template.AudioURL)
	} else {
		fmt.Printf("从模版中匹配到了开场白音频：%s", template.AudioURL)
	}

	return &models.HawkingIntro{
		AudioURL:  template.AudioURL,
		Text:      template.Text,
		Scene:     template.SceneTag,
		IntroID:   template.ID,
		StartHour: template.TimeRange[0],
		EndHour:   template.TimeRange[1],
		VoiceType: template.VoiceType,
	}

}
