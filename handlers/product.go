package handlers

import (
	"fmt"
	"hawker-backend/logic"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	Repo repositories.ProductRepository
}

// NewProductHandler 构造函数，强制注入 Repository
func NewProductHandler(repo repositories.ProductRepository) *ProductHandler {
	return &ProductHandler{Repo: repo}
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
func (h *ProductHandler) UpdateHawkingConfig(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Mode        models.HawkingMode `json:"mode"`
		CustomPrice float64            `json:"custom_price"`
		Weight      int                `json:"weight"`
		IntervalSec int                `json:"interval_sec"`
		IsUrgent    bool               `json:"is_urgent"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 1. 获取现有商品
	product, err := h.Repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}

	// 2. 组装更新字段，避免 Save 全字段覆盖
	updates := make(map[string]interface{})
	updates["hawking_mode"] = input.Mode
	updates["is_hawking"] = (input.Mode != models.ModeStop)

	if input.CustomPrice > 0 {
		updates["custom_price"] = input.CustomPrice
		product.CustomPrice = input.CustomPrice // 用于后面生成脚本预览
	}
	if input.Weight > 0 {
		updates["weight"] = input.Weight
	}
	if input.IntervalSec > 0 {
		updates["interval_sec"] = input.IntervalSec
	}

	if input.IsUrgent {
		updates["priority"] = 99
		past := time.Now().Add(-24 * time.Hour)
		updates["last_hawked_at"] = &past
	} else {
		updates["priority"] = 0
	}

	// 3. 执行更新
	if err := h.Repo.UpdateHawkingFields(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 4. 生成预览（这里依然引用 logic 包，符合 Domain 逻辑）
	product.HawkingMode = input.Mode // 同步内存对象用于预览
	c.JSON(http.StatusOK, gin.H{
		"message":        "叫卖配置已更新",
		"script_preview": logic.GenerateSmartScript(*product),
	})
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
