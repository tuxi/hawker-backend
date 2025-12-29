package repositories

import (
	"hawker-backend/models"

	"gorm.io/gorm"
)

type ProductRepository interface {
	Create(p *models.Product) error
	FindByID(id string) (*models.Product, error)
	FindAll() ([]models.Product, error)
	Update(p *models.Product) error
	Delete(id string) error
	// 专门处理叫卖状态的原子更新
	UpdateHawkingFields(id string, fields map[string]interface{}) error

	GetNextHawkingProduct() (*models.Product, error)
	UpdateHawkingStatus(id string, updates map[string]interface{}) error
}

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

// Create 实现接口：创建商品
func (r *productRepository) Create(p *models.Product) error {
	return r.db.Create(p).Error
}

// FindByID 实现接口：根据 ID 查询
func (r *productRepository) FindByID(id string) (*models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Category").First(&product, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

// FindAll 实现接口：查询所有
func (r *productRepository) FindAll() ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Category").Find(&products).Error
	return products, err
}

// Update 实现接口：全字段更新
func (r *productRepository) Update(p *models.Product) error {
	return r.db.Save(p).Error
}

// UpdateHawkingFields 实现接口：局部字段原子更新（最常用）
func (r *productRepository) UpdateHawkingFields(id string, fields map[string]interface{}) error {
	return r.db.Model(&models.Product{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 实现接口：删除
func (r *productRepository) Delete(id string) error {
	return r.db.Delete(&models.Product{}, "id = ?", id).Error
}

func (r *productRepository) GetNextHawkingProduct() (*models.Product, error) {
	var product models.Product
	// 封装复杂的调度查询 SQL
	err := r.db.Where("is_hawking = ?", true).
		Order("priority DESC, last_hawked_at ASC NULLS FIRST").
		First(&product).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) UpdateHawkingStatus(id string, updates map[string]interface{}) error {
	return r.db.Model(&models.Product{}).Where("id = ?", id).Updates(updates).Error
}
