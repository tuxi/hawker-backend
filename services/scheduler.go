package services

import (
	"context"
	"hawker-backend/logic"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"log"
	"time"
)

type HawkingScheduler struct {
	productRepo  repositories.ProductRepository
	audioService AudioService
	hub          *Hub
}

func NewHawkingScheduler(repo repositories.ProductRepository, audio AudioService, hub *Hub) *HawkingScheduler {
	return &HawkingScheduler{
		productRepo:  repo,
		audioService: audio,
		hub:          hub,
	}
}

func (s *HawkingScheduler) Start(ctx context.Context) {
	// 使用显式的协程管理
	go func() {
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
	script := logic.GenerateHawkingScript(*p)

	// 2. 合成语音
	audioURL, err := s.audioService.GenerateAudio(ctx, script, p.ID.String())
	if err != nil {
		log.Printf("❌ 语音合成失败 [%s]: %v", p.Name, err)
		return
	}

	// 3. WebSocket 广播推送
	s.hub.Broadcast(audioURL, script)
	log.Printf("📢 正在叫卖: %s | 文案: %s", p.Name, script)

	// 4. 更新数据库状态 (重置优先级并记录时间)
	updates := map[string]interface{}{
		"last_hawked_at": time.Now(),
		"priority":       0, // 执行完后重置插播优先级
	}
	if err := s.productRepo.UpdateHawkingStatus(p.ID.String(), updates); err != nil {
		log.Printf("❌ 更新叫卖状态失败: %v", err)
	}
}
