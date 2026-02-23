package repositories

import (
	"hawker-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CategoryRepository interface {
	Create(c *models.Category) error
	FindCategoriesByStoreID(storeID string) ([]models.Category, error)
	FindByID(id string) (*models.Category, error)
	SyncCategories(items []models.CategoryDTO) error
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

func (r *categoryRepository) FindCategoriesByStoreID(storeID string) ([]models.Category, error) {
	var categories []models.Category
	err := r.db.Find(&categories).Where("store_id = ?", storeID).Error
	return categories, err
}

func (r *categoryRepository) FindByID(id string) (*models.Category, error) {
	var category models.Category
	err := r.db.First(&category, "id = ?", id).Error
	return &category, err
}

func (r *categoryRepository) SyncCategories(items []models.CategoryDTO) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			cat := models.Category{
				Base:    models.Base{ID: item.ID},
				Name:    item.Name,
				StoreID: item.StoreID,
			}

			// 🌟 终极逻辑：只认 ID。
			// ID 一样就更新名称；ID 不一样就插入，不管名字重不重复。
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "store_id", "updated_at"}),
			}).Create(&cat).Error

			if err != nil {
				return err
			}
		}
		return nil
	})
}
