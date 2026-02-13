package handlers

import (
	"hawker-backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StoreHandler struct {
	DB *gorm.DB
}

func NewStoreHandler(db *gorm.DB) *StoreHandler {
	return &StoreHandler{DB: db}
}

func (h *StoreHandler) CreateStore(c *gin.Context) {
	// 从中间件中获取已验证的老板 ID
	ownerID := c.MustGet("current_owner_id").(uuid.UUID)

	var store models.Store
	if err := c.ShouldBindJSON(&store); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	store.OwnerID = ownerID // 强制绑定归属关系

	if err := h.DB.Create(&store).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, store)
}

// GetMyStores 获取当前老板名下的所有门店
func (h *StoreHandler) GetMyStores(c *gin.Context) {
	ownerID := c.MustGet("current_owner_id").(uuid.UUID)

	var stores []models.Store
	if err := h.DB.Where("owner_id = ?", ownerID).Find(&stores).Error; err != nil {
		c.JSON(500, gin.H{"error": "获取门店失败"})
		return
	}

	c.JSON(200, stores)
}
