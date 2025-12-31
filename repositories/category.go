package repositories

import (
	"hawker-backend/models"

	"gorm.io/gorm"
)

type CategoryRepository interface {
	Create(c *models.Category) error
	FindAll() ([]models.Category, error)
	FindByID(id string) (*models.Category, error)
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(c *models.Category) error {
	return r.db.Create(c).Error
}

func (r *categoryRepository) FindAll() ([]models.Category, error) {
	var categories []models.Category
	err := r.db.Find(&categories).Error
	return categories, err
}

func (r *categoryRepository) FindByID(id string) (*models.Category, error) {
	var category models.Category
	err := r.db.First(&category, "id = ?", id).Error
	return &category, err
}
