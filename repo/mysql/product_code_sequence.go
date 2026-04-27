package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/repo"
)

type productCodeSequenceRepo struct {
	db *DB
}

func NewProductCodeSequenceRepo(db *DB) repo.ProductCodeSequenceRepo {
	return &productCodeSequenceRepo{db: db}
}

func (r *productCodeSequenceRepo) AllocateRange(ctx context.Context, tx repo.Tx, prefix, categoryShortCode string, count int) (int64, error) {
	if count <= 0 {
		return 0, fmt.Errorf("count must be > 0")
	}
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	categoryShortCode = strings.ToUpper(strings.TrimSpace(categoryShortCode))
	if prefix == "" {
		return 0, fmt.Errorf("prefix is required")
	}
	if categoryShortCode == "" {
		return 0, fmt.Errorf("category_short_code is required")
	}

	sqlTx := Unwrap(tx)
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO product_code_sequences (prefix, category_code, next_value)
		VALUES (?, ?, 0)
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`,
		prefix,
		categoryShortCode,
	); err != nil {
		return 0, fmt.Errorf("upsert product_code_sequences: %w", err)
	}

	var rowID int64
	if err := sqlTx.QueryRowContext(ctx, `SELECT LAST_INSERT_ID()`).Scan(&rowID); err != nil {
		return 0, fmt.Errorf("read product_code_sequences row id: %w", err)
	}

	var current int64
	err := sqlTx.QueryRowContext(ctx, `
		SELECT next_value
		FROM product_code_sequences
		WHERE id = ?
		FOR UPDATE`, rowID).Scan(&current)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("product_code_sequences row disappeared for id=%d", rowID)
	}
	if err != nil {
		return 0, fmt.Errorf("select product_code_sequences next_value: %w", err)
	}
	if current == 0 {
		bootstrap, err := maxExistingSequenceByShortCode(ctx, sqlTx, prefix, categoryShortCode)
		if err != nil {
			return 0, fmt.Errorf("bootstrap max existing sequence: %w", err)
		}
		if bootstrap >= 0 {
			current = bootstrap + 1
		}
	}

	next := current + int64(count)
	if _, err := sqlTx.ExecContext(ctx, `
		UPDATE product_code_sequences
		SET next_value = ?
		WHERE id = ?`, next, rowID); err != nil {
		return 0, fmt.Errorf("update product_code_sequences next_value: %w", err)
	}
	return current, nil
}

func maxExistingSequenceByShortCode(ctx context.Context, tx *sql.Tx, prefix, categoryShortCode string) (int64, error) {
	codePrefix := prefix + categoryShortCode
	skuLen := len(codePrefix) + 6
	digitStart := len(codePrefix) + 1 // SUBSTRING is 1-based.
	likePattern := codePrefix + "%"

	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT MAX(CAST(SUBSTRING(sku_code, ?, 6) AS UNSIGNED))
		FROM task_sku_items
		WHERE sku_code LIKE ?
		  AND CHAR_LENGTH(sku_code) = ?
		  AND SUBSTRING(sku_code, ?, 6) REGEXP '^[0-9]{6}$'`,
		digitStart,
		likePattern,
		skuLen,
		digitStart,
	).Scan(&maxSeq); err != nil {
		return 0, err
	}
	if !maxSeq.Valid {
		return -1, nil
	}
	return maxSeq.Int64, nil
}
