package repositories

import (
	"hawker-backend/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductRepository interface {
	Create(p *models.Product) error
	FindByID(id string) (*models.Product, error)
	FindAll() ([]models.Product, error)
	Update(p *models.Product) error
	Delete(id string) error
	SyncProducts(items []models.ProductDTO) error
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
	now := time.Now()
	// 定义锁超时时间：如果一个商品锁定超过 2 分钟未释放，视为挂掉，允许重新调度
	lockDeadline := now.Add(-2 * time.Minute)

	// 开启事务，确保查询和更新的原子性
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 构建子查询：找到当前最该叫卖的 1 个商品 ID
		// 过滤条件：
		// a. is_hawking = true (开启了叫卖)
		// b. hawking_status = 'idle' (空闲) 或者 (正在处理中但已超时)
		subQuery := tx.Model(&models.Product{}).
			Select("id").
			Where("is_hawking = ?", true).
			Where("(hawking_status = ?) OR (hawking_status = ? AND locked_at < ?)", "idle", "processing", lockDeadline).
			Order("priority DESC, last_hawked_at ASC"). // 优先级最高且最久没喊过的优先
			Limit(1)

		// 2. 使用原生 SQL 的 FOR UPDATE SKIP LOCKED 实现行级抢占
		// 这在 PostgreSQL 和 MySQL 8.0+ 中支持极好，能完美解决并发竞争
		var targetID uuid.UUID
		rawSQL := "SELECT id FROM (?) AS t FOR UPDATE SKIP LOCKED"
		if err := tx.Raw(rawSQL, subQuery).Scan(&targetID).Error; err != nil {
			return err
		}

		if targetID == uuid.Nil {
			return gorm.ErrRecordNotFound
		}

		// 3. 执行锁定更新：将状态改为 processing
		if err := tx.Model(&product).
			Where("id = ?", targetID).
			Updates(map[string]interface{}{
				"hawking_status": "processing",
				"locked_at":      now,
			}).First(&product).Error; err != nil {
			return err
		}

		return nil
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
				Base:       models.Base{ID: item.ID},
				Name:       item.Name,
				Unit:       item.Unit,
				CategoryID: category.ID,
			}

			// 3. 执行 Upsert (存在则更新，不存在则插入)
			// 注意：AssignmentColumns 仅列出需要从 App 同步过来的字段
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "unit", "category_id"}),
			}).Create(&p).Error

			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
