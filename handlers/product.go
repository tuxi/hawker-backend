package handlers

import (
	"context"
	"fmt"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"hawker-backend/services"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProductHandler struct {
	Repo      repositories.ProductRepository
	Scheduler *services.HawkingScheduler
}

// NewProductHandler 构造函数，强制注入 Repository
func NewProductHandler(repo repositories.ProductRepository, Scheduler *services.HawkingScheduler) *ProductHandler {
	return &ProductHandler{Repo: repo, Scheduler: Scheduler}
}

// CreateProduct 创建商品
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 通过 Repo 操作，不再直接调用 database.DB
	if err := h.Repo.Create(&product); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// GetProducts 获取所有
func (h *ProductHandler) GetProducts(c *gin.Context) {
	products, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, products)
}

// UpdateHawkingConfig 更新叫卖配置 (重构后的核心逻辑)
//func (h *ProductHandler) UpdateHawkingConfig(c *gin.Context) {
//	id := c.Param("id")
//	var input struct {
//		Mode        models.HawkingMode `json:"mode"`
//		CustomPrice float64            `json:"custom_price"`
//		Weight      int                `json:"weight"`
//		IntervalSec int                `json:"interval_sec"`
//		IsUrgent    bool               `json:"is_urgent"`
//	}
//
//	if err := c.ShouldBindJSON(&input); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
//		return
//	}
//
//	// 1. 获取现有商品
//	product, err := h.Repo.FindByID(id)
//	if err != nil {
//		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
//		return
//	}
//
//	// 2. 组装更新字段，避免 Save 全字段覆盖
//	updates := make(map[string]interface{})
//	updates["hawking_mode"] = input.Mode
//	updates["is_hawking"] = (input.Mode != models.ModeStop)
//
//	if input.CustomPrice > 0 {
//		updates["custom_price"] = input.CustomPrice
//		product.CustomPrice = input.CustomPrice // 用于后面生成脚本预览
//	}
//	if input.Weight > 0 {
//		updates["weight"] = input.Weight
//	}
//	if input.IntervalSec > 0 {
//		updates["interval_sec"] = input.IntervalSec
//	}
//
//	if input.IsUrgent {
//		updates["priority"] = 99
//		past := time.Now().Add(-24 * time.Hour)
//		updates["last_hawked_at"] = &past
//	} else {
//		updates["priority"] = 0
//	}
//
//	// 3. 执行更新
//	if err := h.Repo.UpdateHawkingFields(id, updates); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
//		return
//	}
//
//	// 4. 生成预览（这里依然引用 logic 包，符合 Domain 逻辑）
//	product.HawkingMode = input.Mode // 同步内存对象用于预览
//	c.JSON(http.StatusOK, gin.H{
//		"message":        "叫卖配置已更新",
//		"script_preview": logic.GenerateSmartScript(*product),
//	})
//}

func (h *ProductHandler) SyncProductsHandler(c *gin.Context) {

	var items []models.ProductDTO
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(400, gin.H{"error": "无效的数据格式"})
		return
	}

	err := h.Repo.SyncProducts(items)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步失败: " + err.Error()})
	} else {
		c.JSON(200, gin.H{"status": "ok", "message": fmt.Sprintf("同步成功，共处理 %d 条数据", len(items))})
	}
}

// StartHawkingHandler 触发商品进入叫卖队列
func (h *ProductHandler) StartHawkingHandler(c *gin.Context) {
	// 1. 定义请求结构
	var req struct {
		ProductID uuid.UUID `json:"product_id"`
		Mode      int       `json:"mode"`         // 0-停止, 1-常规, 2-充足, 3-低库存, 4-促销
		Price     float64   `json:"custom_price"` // 叫卖时的临时价格
		Priority  int       `json:"priority"`     // 优先级：建议传 100 以上实现插播
	}

	// 2. 解析参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}

	// 3. 构造更新数据
	updates := map[string]interface{}{
		"is_hawking":     true,
		"hawking_mode":   models.HawkingMode(req.Mode),
		"custom_price":   req.Price,
		"priority":       req.Priority,
		"hawking_status": "idle", // 强制重置状态，允许调度器立即抓取
	}

	// 4. 调用 Repo 执行更新
	err := h.Repo.UpdateHawkingStatus(req.ProductID.String(), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新叫卖状态失败"})
		return
	}

	// 检查调度器是否在运行 (需要你把 scheduler 实例传给 handler)
	if atomic.LoadInt32(&h.Scheduler.IsRunning) == 0 {
		log.Println("⚡ 监测到新任务且引擎未运行，正在自动唤醒...")
		// 注意：这里需要一个新的 context，不能用已经失效的旧 context
		go h.Scheduler.Start(context.Background())
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"message":    "叫卖指令已下达",
		"product_id": req.ProductID,
		"status":     "queued",
	})
}

// AddHawkingTaskHandler 添加叫卖任务
func (h *ProductHandler) AddHawkingTaskHandler(c *gin.Context) {
	var req models.AddTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}

	product, err := h.Repo.FindByID(req.ProductID)
	if err != nil {
		c.JSON(404, gin.H{"error": "商品不存在"})
		return
	}

	// 更新内存
	h.Scheduler.AddTask(product, req)

	// --- 关键点：直接返回全量列表 ---
	currentTasks := h.Scheduler.GetActiveTasksSnapshot()

	// (可选) 仍然保留广播，为了通知其他可能在线的设备
	h.Scheduler.Hub.BroadcastTaskBundle(currentTasks)

	c.JSON(200, gin.H{
		"message": "添加成功",
		"tasks":   currentTasks, // Swift 端拿到这个直接更新数组
	})
}

// RemoveHawkingTaskHandler 移除叫卖任务
func (h *ProductHandler) RemoveHawkingTaskHandler(c *gin.Context) {
	productID := c.Param("id")

	h.Scheduler.RemoveTask(productID)

	currentTasks := h.Scheduler.GetActiveTasksSnapshot()

	// (可选) 仍然广播给其他设备
	h.Scheduler.Hub.BroadcastTaskBundle(currentTasks)

	c.JSON(200, gin.H{
		"message": "移除成功",
		"tasks":   currentTasks,
	})
}
