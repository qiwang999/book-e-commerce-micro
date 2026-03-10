CREATE DATABASE IF NOT EXISTS bookhive CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE bookhive;

-- 用户表
CREATE TABLE users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(500) DEFAULT '',
    role ENUM('customer', 'admin', 'store_manager') DEFAULT 'customer',
    status TINYINT DEFAULT 1 COMMENT '1=active, 0=disabled',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_email (email)
) ENGINE=InnoDB;

-- 用户画像/偏好表
CREATE TABLE user_profiles (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL UNIQUE,
    phone VARCHAR(20) DEFAULT '',
    gender TINYINT DEFAULT 0 COMMENT '0=unknown, 1=male, 2=female',
    birthday DATE DEFAULT NULL,
    favorite_categories JSON COMMENT '喜好分类',
    favorite_authors JSON COMMENT '喜好作者',
    reading_preferences JSON COMMENT '阅读偏好标签',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 用户地址表
CREATE TABLE user_addresses (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    province VARCHAR(50) NOT NULL,
    city VARCHAR(50) NOT NULL,
    district VARCHAR(50) NOT NULL,
    detail VARCHAR(500) NOT NULL,
    is_default TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 门店表
CREATE TABLE stores (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    address VARCHAR(500) NOT NULL,
    city VARCHAR(50) NOT NULL,
    district VARCHAR(50) DEFAULT '',
    phone VARCHAR(20) DEFAULT '',
    location POINT NOT NULL SRID 4326,
    business_hours VARCHAR(100) DEFAULT '09:00-21:00',
    status TINYINT DEFAULT 1 COMMENT '1=open, 0=closed',
    image_url VARCHAR(500) DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    SPATIAL INDEX idx_location (location),
    INDEX idx_city (city)
) ENGINE=InnoDB;

-- 门店库存表
CREATE TABLE store_inventory (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    store_id BIGINT UNSIGNED NOT NULL,
    book_id VARCHAR(24) NOT NULL COMMENT 'MongoDB ObjectId',
    quantity INT UNSIGNED DEFAULT 0,
    locked_quantity INT UNSIGNED DEFAULT 0,
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_store_book (store_id, book_id),
    INDEX idx_book_id (book_id),
    FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 库存锁定记录表
CREATE TABLE inventory_locks (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    lock_id VARCHAR(36) NOT NULL UNIQUE COMMENT 'UUID',
    store_id BIGINT UNSIGNED NOT NULL,
    book_id VARCHAR(24) NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    status ENUM('locked', 'released', 'deducted') DEFAULT 'locked',
    expire_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_lock_id (lock_id),
    INDEX idx_expire (status, expire_at)
) ENGINE=InnoDB;

-- 订单表
CREATE TABLE orders (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    order_no VARCHAR(32) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL,
    store_id BIGINT UNSIGNED NOT NULL,
    total_amount DECIMAL(10,2) NOT NULL,
    status ENUM('pending_payment', 'paid', 'preparing', 'ready', 'shipping', 'completed', 'cancelled') DEFAULT 'pending_payment',
    pickup_method ENUM('in_store', 'delivery') DEFAULT 'in_store',
    address_id BIGINT UNSIGNED DEFAULT NULL,
    remark TEXT,
    paid_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    cancelled_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_store_id (store_id),
    INDEX idx_status (status),
    INDEX idx_order_no (order_no)
) ENGINE=InnoDB;

-- 订单项表
CREATE TABLE order_items (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    order_id BIGINT UNSIGNED NOT NULL,
    book_id VARCHAR(24) NOT NULL,
    book_title VARCHAR(500) NOT NULL,
    book_author VARCHAR(200) DEFAULT '',
    book_cover VARCHAR(500) DEFAULT '',
    price DECIMAL(10,2) NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    lock_id VARCHAR(36) DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_order_id (order_id),
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 支付记录表
CREATE TABLE payments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    payment_no VARCHAR(32) NOT NULL UNIQUE,
    order_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    method ENUM('wechat', 'alipay', 'credit_card', 'simulated') DEFAULT 'simulated',
    status ENUM('pending', 'processing', 'success', 'failed', 'refunded') DEFAULT 'pending',
    paid_at TIMESTAMP NULL,
    refunded_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_order_id (order_id),
    INDEX idx_payment_no (payment_no)
) ENGINE=InnoDB;

-- 插入测试门店数据
INSERT INTO stores (name, description, address, city, district, phone, location, business_hours) VALUES
('BookHive 东京秋叶原店', '位于秋叶原电器街的旗舰书店，专营动漫、轻小说和技术书籍', '东京都千代田区外神田1-15-1', '东京', '千代田区', '03-1234-5678', ST_GeomFromText('POINT(139.7711 35.6985)', 4326), '10:00-21:00'),
('BookHive 东京池袋店', '池袋最大的综合书店，涵盖各类书籍', '东京都丰岛区东池袋1-1-1', '东京', '丰岛区', '03-2345-6789', ST_GeomFromText('POINT(139.7109 35.7295)', 4326), '10:00-22:00'),
('BookHive 大阪难波店', '难波商圈核心书店', '大阪府大阪市中央区难波5-1-1', '大阪', '中央区', '06-1234-5678', ST_GeomFromText('POINT(135.5014 34.6659)', 4326), '10:00-21:00'),
('BookHive 名古屋店', '名古屋站前综合书店', '爱知县名古屋市中村区名�的1-1-1', '名古屋', '中村区', '052-123-4567', ST_GeomFromText('POINT(136.8826 35.1709)', 4326), '10:00-20:00'),
('BookHive 北京三里屯店', '三里屯文化创意书店', '北京市朝阳区三里屯路19号', '北京', '朝阳区', '010-12345678', ST_GeomFromText('POINT(116.4551 39.9339)', 4326), '09:00-22:00'),
('BookHive 上海南京路店', '南京路步行街旗舰店', '上海市黄浦区南京东路100号', '上海', '黄浦区', '021-12345678', ST_GeomFromText('POINT(121.4868 31.2435)', 4326), '09:00-22:00');
