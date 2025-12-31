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
	"sync/atomic"
	"time"
)

type HawkingScheduler struct {
	productRepo  repositories.ProductRepository
	audioService AudioService
	hub          *Hub
	isRunning    int32 // 使用原子操作标记
}

func NewHawkingScheduler(repo repositories.ProductRepository, audio AudioService, hub *Hub) *HawkingScheduler {
	return &HawkingScheduler{
		productRepo:  repo,
		audioService: audio,
		hub:          hub,
	}
}

func (s *HawkingScheduler) Start(ctx context.Context) {
	// 确保智能启动一个实例
	if !atomic.CompareAndSwapInt32(&s.isRunning, 0, 1) {
		log.Println("⚠️ 调度引擎已经在运行中，请勿重复启动")
		return
	}
	// 使用显式的协程管理
	go func() {
		defer atomic.StoreInt32(&s.isRunning, 0)
		log.Println("🚀 叫卖调度引擎已启动...")
		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 叫卖调度引擎已停止")
				return
			default:
				// 1. 获取下一个需要叫卖的商品
				product, err := s.productRepo.GetNextHawkingProduct()
				if err != nil {
					// 没找到商品时，休眠一段时间再试
					time.Sleep(5 * time.Second)
					continue
				}

				// 2. 执行叫卖业务逻辑
				s.executeHawking(ctx, product)

				// 3. 动态休眠：优先使用商品自定义间隔，默认 10 秒
				sleepTime := 10
				if product.IntervalSec > 0 {
					sleepTime = product.IntervalSec
				}
				time.Sleep(time.Duration(sleepTime) * time.Second)
			}
		}
	}()
}

// executeHawking 封装具体的执行步骤，保持 Start 方法简洁
func (s *HawkingScheduler) executeHawking(ctx context.Context, p *models.Product) {
	// 1. 生成文案
	script := logic.GenerateSmartScript(*p)

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
		log.Printf("🎙️ 文案已更新，开始实时合成: %s", p.Name)
		audioURL, err = s.audioService.GenerateAudio(ctx, script, p.ID.String())
		if err != nil {
			log.Printf("❌ 语音合成失败: %v", err)
			s.productRepo.UpdateHawkingStatus(p.ID.String(), map[string]interface{}{"hawking_status": "idle"})
			return
		}
		// 更新哈希值准备存入数据库
		p.LastScriptHash = currentHash
	}

	// 5. 推送并更新状态
	s.hub.Broadcast(audioURL, script)

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
