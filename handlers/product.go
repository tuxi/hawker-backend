package handlers

import (
	"fmt"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"hawker-backend/services"
	"net/http"

	"github.com/gin-gonic/gin"
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
	storeID := c.Param("id")

	if storeID == "" {
		c.JSON(400, gin.H{"error": "缺少store_id字段"})
		return
	}
	products, err := h.Repo.FindProductsByStoreID(storeID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, products)
}

// GetDependencies 获取该门店下所有商品的依赖关系
func (h *ProductHandler) GetDependencies(c *gin.Context) {
	storeID := c.Param("id")

	dependencies, err := h.Repo.FindDependencies(storeID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取依赖关系失败"})
		return
	}

	c.JSON(http.StatusOK, dependencies)
}

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

// 获取所有叫卖任务
func (h *ProductHandler) GetHawkingTasksHandler(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(400, gin.H{"error": "必须提供 session_id 以定位叫卖任务"})
		return
	}
	currentTasks := h.Scheduler.GetActiveTasksSnapshot(sessionID)

	c.JSON(200, gin.H{
		"status":  "resumed",
		"message": "已恢复叫卖会话",
		"tasks":   currentTasks,
	})
}

// AddHawkingTaskHandler 添加叫卖任务
func (h *ProductHandler) AddHawkingTaskHandler(c *gin.Context) {
	var req models.AddTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 安全校验：确保商品属于该门店
	product, err := h.Repo.FindByID(req.ProductID)
	if err != nil || product.StoreID.String() != req.StoreID {
		c.JSON(403, gin.H{"error": "非法操作：商品与门店不匹配"})
		return
	}

	// 策略：将 StoreID 作为 SessionID
	// 这样能保证每个门店只有一个独立的 runSessionLoop 在运行
	sessionID := req.StoreID
	// 1. 调用 Scheduler 的 AddTask (内部会自动处理 Session 的懒加载启动)
	h.Scheduler.AddTask(product, req, sessionID)

	// 2. 获取该 Session 的专属快照 (包含该音色的 IntroPool)
	currentTasks := h.Scheduler.GetActiveTasksSnapshot(sessionID)

	// 3. 返回结果给 Swift 端
	c.JSON(200, gin.H{
		"message":    "任务已同步",
		"session_id": sessionID,
		"tasks":      currentTasks,
	})
}

// RemoveHawkingTaskHandler 移除叫卖任务
func (h *ProductHandler) RemoveHawkingTaskHandler(c *gin.Context) {
	productID := c.Param("id")
	storeID := c.Query("store_id") // 对应 Swift: .../tasks/123?store_id=ABC

	if storeID == "" {
		c.JSON(400, gin.H{"error": "必须提供 session_id 以定位叫卖任务"})
		return
	}

	sessionID := storeID
	// 1. 从 Session 中移除任务 (如果任务清空，Scheduler 会自动 StopSession)
	h.Scheduler.RemoveTask(sessionID, productID)

	// 2. 获取快照
	// 注意：如果 Session 刚被销毁，这个方法会返回一个空的 Task 列表
	currentTasks := h.Scheduler.GetActiveTasksSnapshot(sessionID)

	c.JSON(200, gin.H{
		"message":    "移除成功",
		"session_id": sessionID,
		"tasks":      currentTasks,
	})
}

// 同步开场白
func (h *ProductHandler) SyncIntroHandler(c *gin.Context) {
	var req models.SyncIntroReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}

}

// 切换音色
func (h *ProductHandler) SwitchVoiceHandler(c *gin.Context) {
	var req struct {
		SessionID  string   `json:"session_id"`
		VoiceID    string   `json:"voice_id"`
		ProductIDs []string `json:"product_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"status": "参数错误", "session_id": req.SessionID})
		return
	}

	// 触发后端重置与重新合成任务
	h.Scheduler.ChangeSessionVoice(req.SessionID, req.VoiceID, req.ProductIDs)

	currentTasks := h.Scheduler.GetActiveTasksSnapshot(req.SessionID)

	// 3. 在接口响应中立即下发，让客户端知道“文案已经变了”
	c.JSON(200, gin.H{
		"status":     "processing",
		"session_id": req.SessionID,
		"tasks":      currentTasks,
	})
}

// 同步依赖接口
func (h *ProductHandler) SyncDependenciesHandler(c *gin.Context) {
	var items []models.DependencyDTO
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(400, gin.H{"error": "无效的数据格式"})
		return
	}

	if err := h.Repo.SyncDependencies(items); err != nil {
		c.JSON(500, gin.H{"error": "依赖同步失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "count": len(items)})
}
