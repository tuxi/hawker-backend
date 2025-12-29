package handlers

import (
	"hawker-backend/database"
	"hawker-backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateCategory 创建分类
func CreateCategory(c *gin.Context) {
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Create(&category)
	c.JSON(http.StatusOK, category)
}

// GetCategories 获取所有分类（包含商品）
func GetCategories(c *gin.Context) {
	var categories []models.Category
	database.DB.Preload("Products").Find(&categories)
	c.JSON(http.StatusOK, categories)
}

// DeleteCategory 删除分类
func DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.Category{}, "id = ?", id)
	c.JSON(http.StatusOK, gin.H{"message": "分类已删除"})
}
