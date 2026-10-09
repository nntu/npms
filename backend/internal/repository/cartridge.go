package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Cartridge struct {
	ID               string    `json:"id"`
	SKUCode          string    `json:"sku_code"`
	Name             string    `json:"name"`
	CompatibleModels string    `json:"compatible_models"`
	StockNew         int       `json:"stock_new"`
	StockRefilled    int       `json:"stock_refilled"`
	StockEmpty       int       `json:"stock_empty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CartridgeLog struct {
	ID           string    `json:"id"`
	CartridgeID  string    `json:"cartridge_id"`
	DeviceID     string    `json:"device_id,omitempty"`
	ActionType   string    `json:"action_type"`
	SourceType   string    `json:"source_type,omitempty"`
	Quantity     int       `json:"quantity"`
	PageCount    int       `json:"page_count,omitempty"`
	PrintedPages int       `json:"printed_pages,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	PerformedAt  time.Time `json:"performed_at"`
}

type ReplaceCartridgeParams struct {
	LogID       string
	CartridgeID string
	DeviceID    string
	SourceType  string // "new" or "refilled"
	PageCount   int
	Notes       string
}

func (c Cartridge) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("cartridge id is required")
	}
	if strings.TrimSpace(c.SKUCode) == "" {
		return fmt.Errorf("cartridge sku code is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("cartridge name is required")
	}
	return nil
}

func (r *SQLiteRepository) CreateCartridge(ctx context.Context, c Cartridge) error {
	if err := c.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO cartridges(id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		c.ID, c.SKUCode, c.Name, c.CompatibleModels, c.StockNew, c.StockRefilled, c.StockEmpty,
		c.CreatedAt.Format(time.RFC3339Nano), c.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create cartridge: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListCartridges(ctx context.Context) ([]Cartridge, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, created_at, updated_at FROM cartridges ORDER BY name, sku_code`)
	if err != nil {
		return nil, fmt.Errorf("list cartridges: %w", err)
	}
	defer rows.Close()
	var result []Cartridge
	for rows.Next() {
		var c Cartridge
		var comp, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.SKUCode, &c.Name, &comp, &c.StockNew, &c.StockRefilled, &c.StockEmpty, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan cartridge: %w", err)
		}
		c.CompatibleModels = nullString(comp)
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt.String)
		c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt.String)
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) GetCartridge(ctx context.Context, id string) (Cartridge, error) {
	var c Cartridge
	var comp, createdAt, updatedAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, created_at, updated_at FROM cartridges WHERE id = ?`, id).
		Scan(&c.ID, &c.SKUCode, &c.Name, &comp, &c.StockNew, &c.StockRefilled, &c.StockEmpty, &createdAt, &updatedAt)
	if err != nil {
		return Cartridge{}, fmt.Errorf("get cartridge: %w", err)
	}
	c.CompatibleModels = nullString(comp)
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt.String)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt.String)
	return c, nil
}

func (r *SQLiteRepository) UpdateCartridgeStock(ctx context.Context, id string, addStockNew, addStockRefilled, addStockEmpty int, logID, notes string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stock update tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `UPDATE cartridges SET stock_new = MAX(0, stock_new + ?), stock_refilled = MAX(0, stock_refilled + ?), stock_empty = MAX(0, stock_empty + ?), updated_at = ? WHERE id = ?`,
		addStockNew, addStockRefilled, addStockEmpty, now, id)
	if err != nil {
		return fmt.Errorf("update stock: %w", err)
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}

	if logID != "" {
		qty := addStockNew
		if qty == 0 {
			qty = addStockRefilled
		}
		if qty == 0 {
			qty = addStockEmpty
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, notes, performed_at)
			VALUES (?, ?, NULL, 'import', NULL, ?, NULLIF(?, ''), ?)`,
			logID, id, qty, notes, now)
		if err != nil {
			return fmt.Errorf("insert stock log: %w", err)
		}
	}

	return tx.Commit()
}

func (r *SQLiteRepository) ReplacePrinterCartridge(ctx context.Context, params ReplaceCartridgeParams) error {
	if params.CartridgeID == "" || params.DeviceID == "" {
		return fmt.Errorf("cartridge_id and device_id are required")
	}
	if params.SourceType != "new" && params.SourceType != "refilled" {
		return fmt.Errorf("source_type must be 'new' or 'refilled'")
	}
	if params.LogID == "" {
		return fmt.Errorf("log_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace cartridge tx: %w", err)
	}
	defer tx.Rollback()

	var stockNew, stockRefilled int
	err = tx.QueryRowContext(ctx, `SELECT stock_new, stock_refilled FROM cartridges WHERE id = ?`, params.CartridgeID).Scan(&stockNew, &stockRefilled)
	if err != nil {
		return fmt.Errorf("check cartridge stock: %w", err)
	}

	if params.SourceType == "new" {
		if stockNew < 1 {
			return fmt.Errorf("out of stock: no new cartridges available")
		}
		_, err = tx.ExecContext(ctx, `UPDATE cartridges SET stock_new = stock_new - 1, stock_empty = stock_empty + 1, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), params.CartridgeID)
	} else {
		if stockRefilled < 1 {
			return fmt.Errorf("out of stock: no refilled cartridges available")
		}
		_, err = tx.ExecContext(ctx, `UPDATE cartridges SET stock_refilled = stock_refilled - 1, stock_empty = stock_empty + 1, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), params.CartridgeID)
	}
	if err != nil {
		return fmt.Errorf("decrement cartridge stock: %w", err)
	}

	if params.PageCount == 0 {
		var maxPage sql.NullInt64
		_ = tx.QueryRowContext(ctx, `SELECT MAX(cr.raw_value) FROM counter_readings cr JOIN counter_definitions cd ON cd.id = cr.counter_definition_id WHERE cd.device_id = ? AND cr.quality = 'valid'`, params.DeviceID).Scan(&maxPage)
		if maxPage.Valid {
			params.PageCount = int(maxPage.Int64)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, page_count, notes, performed_at)
		VALUES (?, ?, ?, 'replace', ?, 1, ?, NULLIF(?, ''), ?)`,
		params.LogID, params.CartridgeID, params.DeviceID, params.SourceType, params.PageCount, params.Notes, now)
	if err != nil {
		return fmt.Errorf("log cartridge replacement: %w", err)
	}

	return tx.Commit()
}

func (r *SQLiteRepository) RefillCartridges(ctx context.Context, logID, cartridgeID string, quantity int, notes string) error {
	if cartridgeID == "" || quantity < 1 || logID == "" {
		return fmt.Errorf("log_id, cartridge_id and quantity (>=1) are required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refill tx: %w", err)
	}
	defer tx.Rollback()

	var stockEmpty int
	err = tx.QueryRowContext(ctx, `SELECT stock_empty FROM cartridges WHERE id = ?`, cartridgeID).Scan(&stockEmpty)
	if err != nil {
		return fmt.Errorf("check empty stock: %w", err)
	}

	if stockEmpty < quantity {
		return fmt.Errorf("cannot refill %d cartridges: only %d empty cartridges in stock", quantity, stockEmpty)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `UPDATE cartridges SET stock_empty = stock_empty - ?, stock_refilled = stock_refilled + ?, updated_at = ? WHERE id = ?`,
		quantity, quantity, now, cartridgeID)
	if err != nil {
		return fmt.Errorf("update stock for refill: %w", err)
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, notes, performed_at)
		VALUES (?, ?, NULL, 'refill', 'refilled', ?, NULLIF(?, ''), ?)`,
		logID, cartridgeID, quantity, notes, now)
	if err != nil {
		return fmt.Errorf("log cartridge refill: %w", err)
	}

	return tx.Commit()
}

func (r *SQLiteRepository) ListCartridgeLogs(ctx context.Context, cartridgeID, deviceID string, limit, offset int) ([]CartridgeLog, error) {
	if limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid log pagination")
	}
	query := `SELECT id, cartridge_id, device_id, action_type, source_type, quantity, page_count, notes, performed_at FROM cartridge_logs WHERE 1=1`
	args := []any{}
	if cartridgeID != "" {
		query += ` AND cartridge_id = ?`
		args = append(args, cartridgeID)
	}
	if deviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, deviceID)
	}
	query += ` ORDER BY performed_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cartridge logs: %w", err)
	}
	defer rows.Close()

	var result []CartridgeLog
	for rows.Next() {
		var l CartridgeLog
		var deviceIDVal, sourceType, notes, performedAt sql.NullString
		var pageCount sql.NullInt64
		if err := rows.Scan(&l.ID, &l.CartridgeID, &deviceIDVal, &l.ActionType, &sourceType, &l.Quantity, &pageCount, &notes, &performedAt); err != nil {
			return nil, fmt.Errorf("scan cartridge log: %w", err)
		}
		l.DeviceID = nullString(deviceIDVal)
		l.SourceType = nullString(sourceType)
		l.PageCount = int(pageCount.Int64)
		l.Notes = nullString(notes)
		l.PerformedAt, _ = time.Parse(time.RFC3339Nano, performedAt.String)
		result = append(result, l)
	}
	_ = rows.Close()

	// Calculate PrintedPages for replacement logs
	for i := range result {
		if result[i].ActionType != "replace" || result[i].DeviceID == "" {
			continue
		}
		// Find next replace log for the same device (which occurred later than this log)
		var nextLogPage sql.NullInt64
		err := r.db.QueryRowContext(ctx, `SELECT page_count FROM cartridge_logs WHERE device_id = ? AND action_type = 'replace' AND performed_at > ? AND page_count > 0 ORDER BY performed_at ASC LIMIT 1`, result[i].DeviceID, result[i].PerformedAt.Format(time.RFC3339Nano)).Scan(&nextLogPage)
		if err == nil && nextLogPage.Valid && result[i].PageCount > 0 {
			if diff := int(nextLogPage.Int64) - result[i].PageCount; diff > 0 {
				result[i].PrintedPages = diff
			}
		} else if result[i].PageCount > 0 {
			// If it's the currently active cartridge, query current max counter reading
			var currentMax sql.NullInt64
			_ = r.db.QueryRowContext(ctx, `SELECT MAX(cr.raw_value) FROM counter_readings cr JOIN counter_definitions cd ON cd.id = cr.counter_definition_id WHERE cd.device_id = ? AND cr.quality = 'valid'`, result[i].DeviceID).Scan(&currentMax)
			if currentMax.Valid && int(currentMax.Int64) > result[i].PageCount {
				result[i].PrintedPages = int(currentMax.Int64) - result[i].PageCount
			}
		}
	}

	return result, nil
}
