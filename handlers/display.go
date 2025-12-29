package handlers

import (
	"hawker-backend/database"
	"hawker-backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddToDisplay 将商品上架到柜台
func AddToDisplay(c *gin.Context) {
	var item models.DisplayItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 检查商品是否存在于档案库
	var product models.Product
	if err := database.DB.First(&product, "id = ?", item.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "档案库无此商品"})
		return
	}

	// 2. 如果没传价格，默认使用商品档案的价格
	if item.CurrentPrice == 0 {
		item.CurrentPrice = product.Price
	}

	// 3. 上架 (使用 Upsert 逻辑：如果已在柜台，则更新库存)
	err := database.DB.Where(models.DisplayItem{ProductID: item.ProductID}).
		Assign(models.DisplayItem{DisplayStock: item.DisplayStock, CurrentPrice: item.CurrentPrice}).
		FirstOrCreate(&item).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "上架失败"})
		return
	}

	c.JSON(http.StatusOK, item)
}

// UpdateDisplayStock 更新柜台实时库存
func UpdateDisplayStock(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Quantity float64 `json:"quantity"` // 变动数量，负数代表卖出
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var item models.DisplayItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "柜台无此商品"})
		return
	}

	// 更新库存
	item.DisplayStock += input.Quantity
	if item.DisplayStock < 0 {
		item.DisplayStock = 0
	}

	database.DB.Save(&item)
	c.JSON(http.StatusOK, item)
}

// GetDisplayItems 获取当前柜台上所有的商品
func GetDisplayItems(c *gin.Context) {
	var items []models.DisplayItem
	database.DB.Preload("Product").Find(&items)
	c.JSON(http.StatusOK, items)
}
