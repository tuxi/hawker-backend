-- 1. 开启 UUID 扩展（PostgreSQL 必须步骤）
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 2. 分类表
CREATE TABLE categories (
                            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                            name VARCHAR(100) NOT NULL UNIQUE,
                            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                            updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                            deleted_at TIMESTAMP WITH TIME ZONE -- 支撑 GORM 的软删除
);

-- 3. 商品档案表
CREATE TABLE products (
                          id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                          category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
                          name VARCHAR(255) NOT NULL,
                          unit VARCHAR(50),
                          price DECIMAL(10, 2) DEFAULT 0.0,
                          is_active BOOLEAN DEFAULT TRUE,
                          created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                          deleted_at TIMESTAMP WITH TIME ZONE
);

-- 4. 柜台展示/叫卖项表
CREATE TABLE display_items (
                               id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                               product_id UUID NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
                               display_stock DECIMAL(10, 2) DEFAULT 0.0,
                               threshold DECIMAL(10, 2) DEFAULT 1.0,
                               current_price DECIMAL(10, 2) DEFAULT 0.0,
                               is_promoted BOOLEAN DEFAULT FALSE,
                               is_hawking_enabled BOOLEAN DEFAULT FALSE,
                               last_hawked_at TIMESTAMP WITH TIME ZONE,
                               created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                               updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                               deleted_at TIMESTAMP WITH TIME ZONE
);

-- 创建索引优化查询
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_display_items_product ON display_items(product_id);