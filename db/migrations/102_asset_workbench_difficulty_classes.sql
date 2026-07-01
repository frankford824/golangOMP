CREATE TABLE IF NOT EXISTS asset_workbench_difficulty_classes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    sort_order INT NOT NULL DEFAULT 0,
    created_by BIGINT NULL,
    updated_by BIGINT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_aw_difficulty_code (code),
    KEY idx_aw_difficulty_enabled_sort (enabled, sort_order, id),
    CONSTRAINT fk_aw_difficulty_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT fk_aw_difficulty_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO asset_workbench_difficulty_classes
    (code, name, description, enabled, sort_order)
VALUES
    ('A', 'A类', '默认 A 类计价难度', 1, 10),
    ('B', 'B类', '默认 B 类计价难度', 1, 20),
    ('C', 'C类', '默认 C 类计价难度', 1, 30),
    ('A+小夜灯', 'A+小夜灯', '小夜灯专项计价难度', 1, 40)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    sort_order = VALUES(sort_order),
    updated_at = CURRENT_TIMESTAMP;
