package metrics

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const (
	BreakdownPageSize = 25
	MaxHistoryBuckets = 1500
)

func (c *Collector) saveMinute(ctx context.Context, minute int64, totals map[string]Counts, details map[dimensions]Counts) error {
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for protocol, counts := range totals {
		_, err = tx.ExecContext(ctx, `INSERT INTO samples(time,protocol,upload,download) VALUES(?,?,?,?)
			ON CONFLICT(time,protocol) DO UPDATE SET upload=upload+excluded.upload,download=download+excluded.download`,
			minute, protocol, counts.Upload, counts.Download)
		if err != nil {
			return err
		}
	}
	if len(details) > 0 {
		insert, err := tx.PrepareContext(ctx, `INSERT INTO traffic_details(time,protocol,source_ip,inbound,user_id,network,upload,download)
			VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(time,protocol,source_ip,inbound,user_id,network)
			DO UPDATE SET upload=upload+excluded.upload,download=download+excluded.download`)
		if err != nil {
			return err
		}
		defer insert.Close()
		for key, counts := range details {
			if _, err := insert.ExecContext(ctx, minute, key.protocol, key.ip, key.inbound, key.user, key.network, counts.Upload, counts.Download); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

type HistoryFilter struct {
	From, Until                          time.Time
	Protocol, IP, Inbound, User, Network string
}

type HistoryBucket struct {
	Time int64 `json:"time"`
	End  int64 `json:"end"`
	Counts
}

type UsageRow struct {
	Value string `json:"value"`
	Counts
}

type Breakdown struct {
	Rows    []UsageRow `json:"rows"`
	HasMore bool       `json:"hasMore"`
}

var dimensionColumns = map[string]string{
	"ip": "source_ip", "protocol": "protocol", "inbound": "inbound", "user": "user_id", "network": "network",
}

func HistoryGroupValid(group string) bool {
	return dimensionColumns[group] != ""
}

func (f HistoryFilter) hasDetails() bool {
	return f.IP != "" || f.Inbound != "" || f.User != "" || f.Network != ""
}

func (f HistoryFilter) predicates() (string, []any) {
	var clauses []string
	var values []any
	for _, filter := range []struct{ column, value string }{
		{"protocol", f.Protocol}, {"source_ip", f.IP}, {"inbound", f.Inbound}, {"user_id", f.User}, {"network", f.Network},
	} {
		if filter.value != "" {
			clauses = append(clauses, "s."+filter.column+" = ?")
			values = append(values, filter.value)
		}
	}
	return strings.Join(clauses, " AND "), values
}

func (c *Collector) CalendarHistory(ctx context.Context, filter HistoryFilter, bounds []int64) ([]HistoryBucket, error) {
	table := "traffic_details"
	if !filter.hasDetails() {
		table = "samples"
		if filter.Protocol == "" {
			filter.Protocol = "total"
		}
	}
	where, values := filter.predicates()
	encoded, err := json.Marshal(bounds)
	if err != nil {
		return nil, err
	}
	// 由调用方按日历生成边界，SQLite 只按时间索引查找对应区间，避免固定按 24 小时切日。
	rows, err := c.Store.DB.QueryContext(ctx, `
		WITH buckets AS (SELECT value AS start, LEAD(value) OVER (ORDER BY key) AS end FROM json_each(?))
		SELECT b.start,b.end,SUM(s.upload),SUM(s.download)
		FROM buckets b CROSS JOIN `+table+` s ON s.time >= b.start AND s.time < b.end
		WHERE `+where+` GROUP BY b.start,b.end ORDER BY b.start`, append([]any{string(encoded)}, values...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []HistoryBucket{}
	for rows.Next() {
		var bucket HistoryBucket
		if err := rows.Scan(&bucket.Time, &bucket.End, &bucket.Upload, &bucket.Download); err != nil {
			return nil, err
		}
		result = append(result, bucket)
	}
	return result, rows.Err()
}

func (c *Collector) UsageBreakdown(ctx context.Context, filter HistoryFilter, group string, offset int) (Breakdown, error) {
	column := dimensionColumns[group]
	table := "traffic_details"
	if group == "protocol" && !filter.hasDetails() {
		table = "samples"
	}
	where, values := filter.predicates()
	if where != "" {
		where = " AND " + where
	}
	args := append([]any{filter.From.Unix(), filter.Until.Unix()}, values...)
	args = append(args, BreakdownPageSize+1, offset)
	rows, err := c.Store.DB.QueryContext(ctx, `
		SELECT s.`+column+`,SUM(s.upload),SUM(s.download) FROM `+table+` s
		WHERE s.time >= ? AND s.time < ? AND s.`+column+` <> '' AND s.protocol <> 'total'`+where+`
		GROUP BY s.`+column+` HAVING SUM(s.upload)+SUM(s.download) > 0
		ORDER BY SUM(s.upload)+SUM(s.download) DESC, s.`+column+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return Breakdown{}, err
	}
	defer rows.Close()
	result := Breakdown{Rows: []UsageRow{}}
	for rows.Next() {
		var row UsageRow
		if err := rows.Scan(&row.Value, &row.Upload, &row.Download); err != nil {
			return Breakdown{}, err
		}
		result.Rows = append(result.Rows, row)
	}
	if len(result.Rows) > BreakdownPageSize {
		result.Rows = result.Rows[:BreakdownPageSize]
		result.HasMore = true
	}
	return result, rows.Err()
}
