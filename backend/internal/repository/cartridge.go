package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Cartridge struct {
	ID                 string    `json:"id"`
	SKUCode            string    `json:"sku_code"`
	Name               string    `json:"name"`
	CompatibleModels   string    `json:"compatible_models"`
	StockNew           int       `json:"stock_new"`
	StockRefilled      int       `json:"stock_refilled"`
	StockEmpty         int       `json:"stock_empty"`
	StockRefillBottles int       `json:"stock_refill_bottles"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CartridgeLog struct {
	ID                 string    `json:"id"`
	CartridgeID        string    `json:"cartridge_id"`
	DeviceID           string    `json:"device_id,omitempty"`
	ActionType         string    `json:"action_type"`
	SourceType         string    `json:"source_type,omitempty"`
	Quantity           int       `json:"quantity"`
	PageCount          int       `json:"page_count,omitempty"`
	PrintedPages       int       `json:"printed_pages,omitempty"`
	CounterQuality     string    `json:"counter_quality,omitempty"`
	CounterCollectedAt time.Time `json:"counter_collected_at,omitempty"`
	Notes              string    `json:"notes,omitempty"`
	PerformedAt        time.Time `json:"performed_at"`
}

type ReplaceCartridgeParams struct {
	LogID              string
	CartridgeID        string
	DeviceID           string
	SourceType         string // "new" or "refilled"
	PageCount          int
	CounterQuality     string
	CounterCollectedAt time.Time
	Notes              string
}

type RefillPrinterCartridgeParams struct {
	LogID              string
	CartridgeID        string
	DeviceID           string
	Quantity           int
	PageCount          int
	CounterQuality     string
	CounterCollectedAt time.Time
	Notes              string
}

const cartridgeCounterFallbackWindow = 24 * time.Hour

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
	_, err := r.db.ExecContext(ctx, `INSERT INTO cartridges(id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, stock_refill_bottles, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)`,
		c.ID, c.SKUCode, c.Name, c.CompatibleModels, c.StockNew, c.StockRefilled, c.StockEmpty, c.StockRefillBottles,
		c.CreatedAt.Format(time.RFC3339Nano), c.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create cartridge: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListCartridges(ctx context.Context) ([]Cartridge, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, stock_refill_bottles, created_at, updated_at FROM cartridges ORDER BY name, sku_code`)
	if err != nil {
		return nil, fmt.Errorf("list cartridges: %w", err)
	}
	defer rows.Close()
	var result []Cartridge
	for rows.Next() {
		var c Cartridge
		var comp, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&c.ID, &c.SKUCode, &c.Name, &comp, &c.StockNew, &c.StockRefilled, &c.StockEmpty, &c.StockRefillBottles, &createdAt, &updatedAt); err != nil {
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
	err := r.db.QueryRowContext(ctx, `SELECT id, sku_code, name, compatible_models, stock_new, stock_refilled, stock_empty, stock_refill_bottles, created_at, updated_at FROM cartridges WHERE id = ?`, id).
		Scan(&c.ID, &c.SKUCode, &c.Name, &comp, &c.StockNew, &c.StockRefilled, &c.StockEmpty, &c.StockRefillBottles, &createdAt, &updatedAt)
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

func (r *SQLiteRepository) AddRefillBottles(ctx context.Context, id string, quantity int, logID, notes string) error {
	if strings.TrimSpace(id) == "" || quantity < 1 || strings.TrimSpace(logID) == "" {
		return fmt.Errorf("cartridge id, quantity (>=1) and log id are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refill bottle stock tx: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE cartridges SET stock_refill_bottles = stock_refill_bottles + ?, updated_at = ? WHERE id = ?`, quantity, now, id)
	if err != nil {
		return fmt.Errorf("add refill bottle stock: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, notes, performed_at) VALUES (?, ?, NULL, 'import', NULL, ?, NULLIF(?, ''), ?)`, logID, id, quantity, notes, now); err != nil {
		return fmt.Errorf("log refill bottle import: %w", err)
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

	if params.CounterQuality == "" {
		params.CounterQuality = "unavailable"
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, page_count, counter_quality, counter_collected_at, notes, performed_at)
		VALUES (?, ?, ?, 'replace', ?, 1, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`,
		params.LogID, params.CartridgeID, params.DeviceID, params.SourceType, params.PageCount, params.CounterQuality, formatTime(params.CounterCollectedAt), params.Notes, now)
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

func (r *SQLiteRepository) RefillPrinterCartridge(ctx context.Context, params RefillPrinterCartridgeParams) error {
	if params.LogID == "" || params.CartridgeID == "" || params.DeviceID == "" || params.Quantity < 1 {
		return fmt.Errorf("log_id, cartridge_id, device_id and quantity (>=1) are required")
	}
	if params.CounterQuality == "" {
		params.CounterQuality = "unavailable"
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin printer refill tx: %w", err)
	}
	defer tx.Rollback()
	var stock int
	if err := tx.QueryRowContext(ctx, `SELECT stock_refill_bottles FROM cartridges WHERE id = ?`, params.CartridgeID).Scan(&stock); err != nil {
		return fmt.Errorf("check refill bottle stock: %w", err)
	}
	if stock < params.Quantity {
		return fmt.Errorf("out of stock: need %d refill bottles, only %d available", params.Quantity, stock)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE cartridges SET stock_refill_bottles = stock_refill_bottles - ?, updated_at = ? WHERE id = ?`, params.Quantity, now, params.CartridgeID); err != nil {
		return fmt.Errorf("decrement refill bottle stock: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cartridge_logs(id, cartridge_id, device_id, action_type, source_type, quantity, page_count, counter_quality, counter_collected_at, notes, performed_at) VALUES (?, ?, ?, 'refill', 'refilled', ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`, params.LogID, params.CartridgeID, params.DeviceID, params.Quantity, params.PageCount, params.CounterQuality, formatTime(params.CounterCollectedAt), params.Notes, now); err != nil {
		return fmt.Errorf("log printer refill: %w", err)
	}
	return tx.Commit()
}

func (r *SQLiteRepository) ListCartridgeLogs(ctx context.Context, cartridgeID, deviceID string, limit, offset int) ([]CartridgeLog, error) {
	if limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid log pagination")
	}
	query := `SELECT id, cartridge_id, device_id, action_type, source_type, quantity, page_count, counter_quality, counter_collected_at, notes, performed_at FROM cartridge_logs WHERE 1=1`
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
		var counterQuality, counterCollectedAt sql.NullString
		if err := rows.Scan(&l.ID, &l.CartridgeID, &deviceIDVal, &l.ActionType, &sourceType, &l.Quantity, &pageCount, &counterQuality, &counterCollectedAt, &notes, &performedAt); err != nil {
			return nil, fmt.Errorf("scan cartridge log: %w", err)
		}
		l.DeviceID = nullString(deviceIDVal)
		l.SourceType = nullString(sourceType)
		l.PageCount = int(pageCount.Int64)
		l.CounterQuality = nullString(counterQuality)
		l.CounterCollectedAt = parseTime(counterCollectedAt)
		l.Notes = nullString(notes)
		l.PerformedAt, _ = time.Parse(time.RFC3339Nano, performedAt.String)
		result = append(result, l)
	}
	_ = rows.Close()

	// Calculate PrintedPages for replacement logs. Counter data is best effort:
	// it estimates yield and must never block or change inventory transactions.
	for i := range result {
		if result[i].ActionType != "replace" || result[i].DeviceID == "" {
			continue
		}
		baseline := result[i].PageCount
		if baseline <= 0 {
			var found bool
			baseline, found, err = r.nearestCartridgeCounter(ctx, result[i].DeviceID, result[i].PerformedAt, true)
			if err != nil {
				return nil, fmt.Errorf("find cartridge counter baseline: %w", err)
			}
			if !found {
				continue
			}
		}

		// Prefer the next replacement snapshot. If it was not captured, use the
		// closest counter before that event, then a nearby reading after it.
		var nextLogAtText string
		var nextLogPage int
		var nextLogFound bool
		err := r.db.QueryRowContext(ctx, `SELECT performed_at, page_count FROM cartridge_logs WHERE device_id = ? AND action_type = 'replace' AND performed_at > ? ORDER BY performed_at ASC, id ASC LIMIT 1`, result[i].DeviceID, result[i].PerformedAt.Format(time.RFC3339Nano)).Scan(&nextLogAtText, &nextLogPage)
		if err == nil {
			nextLogAt, parseErr := time.Parse(time.RFC3339Nano, nextLogAtText)
			if parseErr != nil {
				return nil, fmt.Errorf("parse next cartridge replacement time: %w", parseErr)
			}
			if nextLogPage > 0 {
				nextLogFound = true
			} else {
				nextLogPage, nextLogFound, err = r.nearestCartridgeCounter(ctx, result[i].DeviceID, nextLogAt, false)
				if err != nil {
					return nil, fmt.Errorf("find cartridge counter endpoint: %w", err)
				}
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find next cartridge replacement: %w", err)
		} else {
			// The current cartridge has no next replacement yet. Use the latest
			// nearby reading as an interim estimate.
			nextLogPage, nextLogFound, err = r.nearestCartridgeCounter(ctx, result[i].DeviceID, time.Now().UTC(), false)
			if err != nil {
				return nil, fmt.Errorf("find current cartridge counter: %w", err)
			}
		}
		if nextLogFound && nextLogPage > baseline {
			result[i].PrintedPages = nextLogPage - baseline
		}
	}

	return result, nil
}

// nearestCartridgeCounter returns one stable, page-like counter for a device.
// It prefers the profile's marker_life/engine counter and only considers valid
// or unverified readings inside a small window. This intentionally produces an
// estimate instead of pretending to provide an exact cartridge boundary.
func (r *SQLiteRepository) nearestCartridgeCounter(ctx context.Context, deviceID string, at time.Time, preferAfter bool) (int, bool, error) {
	var definitionID string
	if err := r.db.QueryRowContext(ctx, `SELECT id FROM counter_definitions WHERE device_id = ? ORDER BY CASE WHEN semantic_type = 'marker_life' THEN 0 WHEN scope = 'engine' AND unit IN ('impressions', 'sheets') THEN 1 ELSE 2 END, id LIMIT 1`, deviceID).Scan(&definitionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("select cartridge counter definition: %w", err)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	from := at.Add(-cartridgeCounterFallbackWindow).Format(time.RFC3339Nano)
	to := at.Add(cartridgeCounterFallbackWindow).Format(time.RFC3339Nano)
	atText := at.Format(time.RFC3339Nano)
	query := `SELECT raw_value FROM counter_readings WHERE counter_definition_id = ? AND quality IN ('valid', 'unverified') AND collected_at >= ? AND collected_at <= ? AND collected_at <= ? ORDER BY collected_at DESC, id DESC LIMIT 1`
	args := []any{definitionID, from, to, atText}
	if preferAfter {
		query = `SELECT raw_value FROM counter_readings WHERE counter_definition_id = ? AND quality IN ('valid', 'unverified') AND collected_at >= ? AND collected_at <= ? AND collected_at >= ? ORDER BY collected_at ASC, id ASC LIMIT 1`
		args = []any{definitionID, from, to, atText}
	}
	var value int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&value); err == nil {
		if value < 0 {
			return 0, false, nil
		}
		return int(value), true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, fmt.Errorf("read nearby cartridge counter: %w", err)
	}

	// If the preferred side is missing, accept the closest reading on the
	// other side of the event within the same bounded window.
	if preferAfter {
		query = `SELECT raw_value FROM counter_readings WHERE counter_definition_id = ? AND quality IN ('valid', 'unverified') AND collected_at >= ? AND collected_at <= ? AND collected_at <= ? ORDER BY collected_at DESC, id DESC LIMIT 1`
	} else {
		query = `SELECT raw_value FROM counter_readings WHERE counter_definition_id = ? AND quality IN ('valid', 'unverified') AND collected_at >= ? AND collected_at <= ? AND collected_at >= ? ORDER BY collected_at ASC, id ASC LIMIT 1`
	}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read fallback cartridge counter: %w", err)
	}
	if value < 0 {
		return 0, false, nil
	}
	return int(value), true, nil
}

func (r *SQLiteRepository) CountCartridgeLogs(ctx context.Context, cartridgeID, deviceID string) (int, error) {
	query := `SELECT COUNT(*) FROM cartridge_logs WHERE 1=1`
	args := []any{}
	if cartridgeID != "" {
		query += ` AND cartridge_id = ?`
		args = append(args, cartridgeID)
	}
	if deviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, deviceID)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count cartridge logs: %w", err)
	}
	return count, nil
}
