-- Order shard instance 1: databases bookhive_order_0 and bookhive_order_1

CREATE DATABASE IF NOT EXISTS bookhive_order_0 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS bookhive_order_1 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Shard 0
USE bookhive_order_0;

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

-- Shard 1
USE bookhive_order_1;

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
