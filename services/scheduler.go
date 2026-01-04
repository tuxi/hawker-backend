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

type HawkingScheduler struct {
	productRepo  repositories.ProductRepository
	audioService AudioService
	Hub          *Hub
	IsRunning    int32 // 使用原子操作标记

	ActiveTasks map[string]*models.HawkingTask
	taskMutex   sync.RWMutex // 使用读写锁提高并发性能

	taskNotify chan struct{} //用于通知新任务到达
}

func NewHawkingScheduler(repo repositories.ProductRepository, audio AudioService, hub *Hub) *HawkingScheduler {
	return &HawkingScheduler{
		productRepo:  repo,
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
			var pIDs []string
			for id := range s.ActiveTasks {
				pIDs = append(pIDs, id)
			}
			s.taskMutex.RUnlock()

			// --- B. 没活干就死等信号 ---
			if len(pIDs) == 0 {
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
			for _, id := range pIDs {
				s.taskMutex.RLock()
				task, ok := s.ActiveTasks[id]
				s.taskMutex.RUnlock()
				if !ok {
					continue
				}

				// 🌟 重点排查：FindByID 是否有数据库连接泄露导致阻塞？
				product, err := s.productRepo.FindByID(id)
				if err != nil {
					s.RemoveTask(id)
					continue
				}

				log.Printf("🎙️ 开始叫卖: %s", product.Name)

				// 🌟 重点排查：executeHawking 内部是否有 30 秒的超时？
				// 如果这个函数不返回，下面的信号监听永远不生效
				s.executeHawking(ctx, product, task)

				s.RemoveTask(id)

				// 休息，且能随时响应退出
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

// executeHawking 封装具体的执行步骤，保持 Start 方法简洁
func (s *HawkingScheduler) executeHawking(ctx context.Context, p *models.Product, task *models.HawkingTask) {
	if task == nil {
		return
	}

	// 1. 生成文案
	script := task.Text
	if len(task.Text) == 0 {
		script = logic.GenerateSmartScript(*p, task.Price, task.OriginalPrice)
		log.Printf("📝 为 [%s] 生成文案: %s", p.Name, script)
	}

	// 2. 计算当前文案的哈希值
	currentHash := fmt.Sprintf("%x", md5.Sum([]byte(script)))

	var audioURL string
	var err error

	// 3. 缓存校验
	// 如果文案没变，且对应的音频文件确实存在于磁盘上
	if p.LastScriptHash == currentHash && s.checkAudioExists(p.ID.String()) {
		audioURL = fmt.Sprintf("/static/audio/%s.mp3", p.ID.String())
		log.Printf("♻️ 文案未变，复用缓存音频: %s", p.Name)
	} else {
		// 4. 文案变了或文件丢失，调用火山引擎合成
		log.Printf("🎙️ 文案已更新，正在调用火山引擎合成音频: %s", p.Name)
		audioURL, err = s.audioService.GenerateAudio(ctx, script, p.ID.String())
		if err != nil {
			log.Printf("❌ 语音合成失败 [%s]: %v", p.Name, err)
			s.productRepo.UpdateHawkingStatus(p.ID.String(), map[string]interface{}{"hawking_status": "idle"})
			return
		}
		log.Printf("✅ 音频合成成功! 文件路径: %s", audioURL) // 👈 新增：确认合成完成
		// 更新哈希值准备存入数据库
		p.LastScriptHash = currentHash
	}

	// 5. 推送并更新状态
	log.Printf("📡 正在通过 WebSocket 广播指令...")
	s.Hub.BroadcastHawking(audioURL, script, p.ID.String())
	log.Printf("🎉 广播已发出，等待 App 播放") // 👈 新增：确认发送成功

	updates := map[string]interface{}{
		"last_script_hash": p.LastScriptHash,
		"last_hawked_at":   time.Now(),
		"priority":         0,
		"hawking_status":   "idle",
	}
	s.productRepo.UpdateHawkingStatus(p.ID.String(), updates)
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
		Scene:         scene,
	}
	s.taskMutex.Unlock()

	// 发送通知
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
func (s *HawkingScheduler) GetActiveTasksSnapshot() []*models.HawkingTask {
	s.taskMutex.RLock()
	defer s.taskMutex.RUnlock()

	var list = make([]*models.HawkingTask, 0) // 即使为空也返回 [] 而不是 nil
	for _, task := range s.ActiveTasks {
		list = append(list, task)
	}
	return list
}
