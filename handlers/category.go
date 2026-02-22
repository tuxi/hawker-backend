package handlers

import (
	"fmt"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	Repo repositories.CategoryRepository
}

func NewCategoryHandler(repo repositories.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{Repo: repo}
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if err := h.Repo.Create(&category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建分类失败"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

func (h *CategoryHandler) GetAll(c *gin.Context) {
	var req struct {
		StoreID string `form:"store_id" json:"store_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	categories, err := h.Repo.FindCategoriesByStoreID(req.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *CategoryHandler) SyncCategoriesHandler(c *gin.Context) {
	var items []models.CategoryDTO
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的数据格式: " + err.Error()})
		return
	}

	// 调用 Repository 执行同步
	err := h.Repo.SyncCategories(items)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步分类失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("分类同步成功，共处理 %d 条数据", len(items)),
	})
}
