-- Inventory shard instance 2: databases bookhive_inventory_2 and bookhive_inventory_3

CREATE DATABASE IF NOT EXISTS bookhive_inventory_2 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS bookhive_inventory_3 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Shard 2
USE bookhive_inventory_2;

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
    INDEX idx_book_id (book_id)
) ENGINE=InnoDB;

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

-- Shard 3
USE bookhive_inventory_3;

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
    INDEX idx_book_id (book_id)
) ENGINE=InnoDB;

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
