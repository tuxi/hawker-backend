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

// GetPromotions 获取门店促销活动（包含明细）
func (h *StoreHandler) GetPromotions(c *gin.Context) {
	storeID := c.Param("id")
	var sessions []models.PromotionSession

	// 使用 Preload 预加载关联的 Items，并按日期倒序
	if err := h.DB.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("marketing_promotions.sort_order ASC")
	}).Where("store_id = ?", storeID).Order("date DESC").Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取促销活动失败"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// SyncPromotionsHandler 同步促销活动接口
func (h *StoreHandler) SyncPromotionsHandler(c *gin.Context) {
	var items []models.PromotionSessionDTO
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(400, gin.H{"error": "无效的数据格式"})
		return
	}

	if err := h.syncPromotions(items); err != nil {
		c.JSON(500, gin.H{"error": "促销活动同步失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (r *StoreHandler) syncPromotions(items []models.PromotionSessionDTO) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for _, sessionDTO := range items {
			// 1. 同步 Session 外壳
			session := models.PromotionSession{
				Base:      models.Base{ID: sessionDTO.ID},
				Title:     sessionDTO.Title,
				Date:      sessionDTO.Date,
				StartDate: sessionDTO.StartDate,
				StoreID:   sessionDTO.StoreID,
			}

			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"title", "date", "start_date", "store_id", "updated_at",
				}),
			}).Create(&session).Error
			if err != nil {
				return err
			}

			// 2. 同步 Session 下的所有 Promotion Items
			for _, itemDTO := range sessionDTO.Items {
				promoItem := models.MarketingPromotion{
					Base:          models.Base{ID: itemDTO.ID},
					SessionID:     sessionDTO.ID, // 关联父级 ID
					ProductID:     itemDTO.ProductID,
					ProductName:   itemDTO.ProductName,
					OriginalPrice: itemDTO.OriginalPrice,
					PromoPrice:    itemDTO.PromoPrice,
					PromoUnit:     itemDTO.PromoUnit,
					PromoTag:      itemDTO.PromoTag,
					Remark:        itemDTO.Remark,
					SortOrder:     itemDTO.SortOrder,
				}

				err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "id"}},
					DoUpdates: clause.AssignmentColumns([]string{
						"product_id", "product_name", "original_price", "promo_price",
						"promo_unit", "promo_tag", "remark", "sort_order", "updated_at",
					}),
				}).Create(&promoItem).Error
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}
