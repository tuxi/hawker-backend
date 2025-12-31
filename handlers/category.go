package handlers

import (
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
	categories, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, categories)
}
