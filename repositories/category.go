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
	// 开启事务
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			// 构造分类模型
			cat := models.Category{
				Base:    models.Base{ID: item.ID},
				Name:    item.Name,
				StoreID: item.StoreID,
			}

			// 执行 Upsert 操作
			// 如果 ID 冲突（已存在），则更新名称和所属门店
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
