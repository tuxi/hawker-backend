package handlers

import (
	"hawker-backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// GetRevenues 获取门店营业额记录
func (h *StoreHandler) GetRevenues(c *gin.Context) {
	storeID := c.Param("id")
	var revenues []models.SalesRecord

	if err := h.DB.Where("store_id = ?", storeID).Order("date DESC").Find(&revenues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取营业额失败"})
		return
	}

	c.JSON(http.StatusOK, revenues)
}

// GetCategories 获取门店分类
func (h *StoreHandler) GetCategories(c *gin.Context) {
	storeID := c.Param("id")
	var categories []models.Category

	// 假设 ProductCategory 表中有 store_id 字段
	if err := h.DB.Where("store_id = ?", storeID).Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分类失败"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// 同步营业额接口
func (h *StoreHandler) SyncRevenuesHandler(c *gin.Context) {
	var items []models.RevenueDTO
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(400, gin.H{"error": "无效的数据格式"})
		return
	}

	if err := h.syncRevenues(items); err != nil {
		c.JSON(500, gin.H{"error": "营业额同步失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (r *StoreHandler) syncRevenues(items []models.RevenueDTO) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			record := models.SalesRecord{
				Base:    models.Base{ID: item.ID},
				Date:    item.Date,
				Revenue: item.Revenue,
				Notes:   item.Notes,
				StoreID: item.StoreID,
			}

			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"revenue", "notes", "date", "store_id", "updated_at",
				}),
			}).Create(&record).Error

			if err != nil {
				return err
			}
		}
		return nil
	})
}
