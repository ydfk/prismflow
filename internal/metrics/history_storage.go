package metrics

import (
	"context"
	"database/sql"
	"math"
	"prismflow/internal/model"
	"time"
)

const (
	historyTrimBatch   = 20000
	historyCheckWrites = 5000
)

type HistoryStorage struct {
	SummaryBytes    int64 `json:"summaryBytes"`
	DetailBytes     int64 `json:"detailBytes"`
	DatabaseBytes   int64 `json:"databaseBytes"`
	ReusableBytes   int64 `json:"reusableBytes"`
	OldestSummaryAt int64 `json:"oldestSummaryAt"`
	OldestDetailAt  int64 `json:"oldestDetailAt"`
	CheckedAt       int64 `json:"checkedAt"`
}

type historyMaintenance struct {
	storage   HistoryStorage
	policy    model.TrafficHistoryPolicy
	lastCheck time.Time
	writes    int
	pending   bool
	failed    bool
}

type historyPages struct {
	bytes, records int64
}

func (c *Collector) maintainHistory(ctx context.Context, now time.Time, policy model.TrafficHistoryPolicy) {
	m := &c.maintenance
	if m.policy == policy && now.Sub(m.lastCheck) < time.Minute && m.writes < historyCheckWrites && !m.pending {
		return
	}
	storage, pending, err := c.cleanHistory(ctx, now, policy)
	m.lastCheck, m.policy, m.writes, m.pending = now, policy, 0, pending
	if err != nil {
		if !m.failed && ctx.Err() == nil {
			c.Store.Event("流量历史容量检查或清理失败")
		}
		m.failed = true
		return
	}
	m.storage, m.failed = storage, false
}

func (c *Collector) cleanHistory(ctx context.Context, now time.Time, policy model.TrafficHistoryPolicy) (HistoryStorage, bool, error) {
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return HistoryStorage{}, false, err
	}
	defer tx.Rollback()

	cutoff := int64(math.MinInt64)
	if policy.Days > 0 {
		cutoff = now.Add(-time.Duration(policy.Days) * 24 * time.Hour).Truncate(time.Minute).Unix()
	}
	limit := int64(policy.LimitMiB) << 20
	storage, changed, pending, err := trimHistory(ctx, tx, cutoff, limit)
	if err != nil {
		return HistoryStorage{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return HistoryStorage{}, false, err
	}
	storage.ReusableBytes, err = c.reclaimHistoryPages(ctx, changed)
	if err != nil {
		return HistoryStorage{}, false, err
	}
	storage.DatabaseBytes, err = c.Store.FileBytes()
	if err == nil {
		storage.CheckedAt = now.Unix()
	}
	return storage, pending, err
}

func trimHistory(ctx context.Context, tx *sql.Tx, cutoff, limit int64) (HistoryStorage, bool, bool, error) {
	oldSummary, err := trimOldHistory(ctx, tx, "samples", cutoff, historyTrimBatch)
	if err != nil {
		return HistoryStorage{}, false, false, err
	}
	oldDetails, err := trimOldHistory(ctx, tx, "traffic_details", cutoff, historyTrimBatch)
	if err != nil {
		return HistoryStorage{}, false, false, err
	}
	summary, details, err := measureHistory(ctx, tx)
	if err != nil {
		return HistoryStorage{}, false, false, err
	}
	// 给汇总保留四分之一容量，避免来源 IP 的数量挤占协议和总量的历史。
	summaryBudget, detailBudget := limit/4, limit-limit/4
	trimmedSummary, err := trimHistoryBudget(ctx, tx, "samples", summary, summaryBudget)
	if err != nil {
		return HistoryStorage{}, false, false, err
	}
	trimmedDetails, err := trimHistoryBudget(ctx, tx, "traffic_details", details, detailBudget)
	if err != nil {
		return HistoryStorage{}, false, false, err
	}
	if trimmedSummary > 0 || trimmedDetails > 0 {
		summary, details, err = measureHistory(ctx, tx)
		if err != nil {
			return HistoryStorage{}, false, false, err
		}
	}
	storage := HistoryStorage{SummaryBytes: summary.bytes, DetailBytes: details.bytes}
	err = tx.QueryRowContext(ctx, `SELECT
		COALESCE((SELECT MIN(time) FROM samples), 0),
		COALESCE((SELECT MIN(time) FROM traffic_details), 0)`).Scan(&storage.OldestSummaryAt, &storage.OldestDetailAt)
	changed := oldSummary+oldDetails+trimmedSummary+trimmedDetails > 0
	pending := oldSummary >= historyTrimBatch || oldDetails >= historyTrimBatch || summary.bytes > summaryBudget || details.bytes > detailBudget
	return storage, changed, pending, err
}

func measureHistory(ctx context.Context, tx *sql.Tx) (historyPages, historyPages, error) {
	var summary, details historyPages
	err := tx.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN name IN ('samples','sqlite_autoindex_samples_1') THEN pgsize END),0),
		COALESCE(MAX(CASE WHEN name='samples' THEN ncell END),0),
		COALESCE(SUM(CASE WHEN name IN ('traffic_details','traffic_details_ip_time') THEN pgsize END),0),
		COALESCE(MAX(CASE WHEN name='traffic_details' THEN ncell END),0)
		FROM dbstat WHERE aggregate=1
		AND name IN ('samples','sqlite_autoindex_samples_1','traffic_details','traffic_details_ip_time')`).
		Scan(&summary.bytes, &summary.records, &details.bytes, &details.records)
	return summary, details, err
}

func trimHistoryBudget(ctx context.Context, tx *sql.Tx, table string, pages historyPages, budget int64) (int64, error) {
	if pages.bytes <= budget || pages.records == 0 {
		return 0, nil
	}
	// 估算删除量并留出余量，清理后以实际页数决定是否继续下一批。
	records := int64(float64(pages.records) * (1 - 0.9*float64(budget)/float64(pages.bytes)))
	return trimOldHistory(ctx, tx, table, math.MaxInt64, min(max(records, 1), historyTrimBatch))
}

func trimOldHistory(ctx context.Context, tx *sql.Tx, table string, before, count int64) (int64, error) {
	if before == math.MinInt64 {
		return 0, nil
	}
	// 整个时间桶一起清理，避免保留同一分钟中不完整的排名或协议用量。
	result, err := tx.ExecContext(ctx, "DELETE FROM "+table+` WHERE time < ? AND time <= COALESCE(
		(SELECT time FROM `+table+` WHERE time < ? ORDER BY time LIMIT 1 OFFSET ?), ?)`,
		before, before, count-1, before-1)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *Collector) reclaimHistoryPages(ctx context.Context, changed bool) (int64, error) {
	db := c.Store.DB
	const freeSpace = `SELECT (SELECT freelist_count FROM pragma_freelist_count) * (SELECT page_size FROM pragma_page_size)`
	var reusable int64
	var mode int
	if err := db.QueryRowContext(ctx, freeSpace+", (SELECT auto_vacuum FROM pragma_auto_vacuum)").Scan(&reusable, &mode); err != nil {
		return 0, err
	}
	if mode == 2 && reusable >= 1<<20 {
		rows, err := db.QueryContext(ctx, "PRAGMA incremental_vacuum(1024)")
		if err != nil {
			return 0, err
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
		if err := db.QueryRowContext(ctx, freeSpace).Scan(&reusable); err != nil {
			return 0, err
		}
		changed = true
	}
	if changed {
		if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			return 0, err
		}
	}
	return reusable, nil
}
