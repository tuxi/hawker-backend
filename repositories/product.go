package repositories

import (
	"hawker-backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductRepository interface {
	Create(p *models.Product) error
	FindByID(id string) (*models.Product, error)
	FindProductsByStoreID(storeID string) ([]models.Product, error)
	Update(p *models.Product) error
	Delete(id string) error
	SyncProducts(items []models.ProductDTO) error
	// 专门处理叫卖状态的原子更新
	UpdateHawkingFields(id string, fields map[string]interface{}) error
	FindDependencies(storeID string) ([]models.ProductDependency, error)
	SyncDependencies(items []models.DependencyDTO) error

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

// FindCategoriesByStoreID 实现接口：查询某个门店的所有商品
func (r *productRepository) FindProductsByStoreID(storeID string) ([]models.Product, error) {
	var products []models.Product
	err := r.db.Preload("Category").Find(&products).Where("store_id = ?", storeID).Error
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
	now := time.Now()
	lockDeadline := now.Add(-2 * time.Minute)

	// 开启事务
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 直接使用 Clauses 配合锁机制
		// GORM 会自动将这些条件编译为：
		// SELECT * FROM products WHERE ... ORDER BY ... LIMIT 1 FOR UPDATE SKIP LOCKED
		err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "SKIP LOCKED",
		}).Where("is_hawking = ?", true).
			Where("(hawking_status = ?) OR (hawking_status = ? AND locked_at < ?)",
				"idle", "processing", lockDeadline).
			Order("priority DESC, last_hawked_at ASC").
			First(&product).Error

		if err != nil {
			return err // 如果没找到会返回 gorm.ErrRecordNotFound，自动回滚
		}

		// 2. 锁定成功后，立即更新状态
		return tx.Model(&product).Updates(map[string]interface{}{
			"hawking_status": "processing",
			"locked_at":      now,
		}).Error
	})

	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) UpdateHawkingStatus(id string, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	// 每次更新都强制归还状态为 idle
	updates["hawking_status"] = "idle"

	return r.db.Model(&models.Product{}).Where("id = ?", id).Updates(updates).Error
}

func (r *productRepository) SyncProducts(items []models.ProductDTO) error {

	// 在事务中处理同步
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			// 1. 确保分类存在 (根据名称查找或创建)
			var category models.Category
			if err := tx.FirstOrCreate(&category, models.Category{Name: item.CategoryName}).Error; err != nil {
				return err
			}

			// 2. 构造模型
			p := models.Product{
				Base:           models.Base{ID: item.ID},
				Name:           item.Name,
				Unit:           item.Unit,
				CategoryID:     category.ID,
				MarketingLabel: item.MarketingLabel,
			}

			// 3. 执行 Upsert (存在则更新，不存在则插入)
			// 注意：AssignmentColumns 仅列出需要从 App 同步过来的字段
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "unit", "category_id", "marketing_label"}),
			}).Create(&p).Error

			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (r *productRepository) FindDependencies(storeID string) ([]models.ProductDependency, error) {
	var dependencies []models.ProductDependency
	// 核心逻辑：通过 Join Product 表来过滤属于该门店的依赖
	err := r.db.Table("product_dependencies").
		Select("product_dependencies.*").
		Joins("JOIN products ON products.id = product_dependencies.parent_id").
		Where("products.store_id = ?", storeID).
		Find(&dependencies).Error
	return dependencies, err
}

func (r *productRepository) SyncDependencies(items []models.DependencyDTO) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			dep := models.ProductDependency{
				Base:           models.Base{ID: item.ID},
				ParentID:       item.ParentID,
				ChildID:        item.ChildID,
				Ratio:          item.Ratio,
				Priority:       item.Priority,
				AllowsSeparate: item.AllowsSeparate,
			}

			// 执行 Upsert
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"parent_id", "child_id", "ratio", "priority", "allows_separate", "updated_at",
				}),
			}).Create(&dep).Error

			if err != nil {
				return err // 如果 parent_id 或 child_id 不存在，由于外键约束会报错
			}
		}
		return nil
	})
}

func (r *productRepository) SyncRevenues(items []models.RevenueDTO) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
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
