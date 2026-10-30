package metrics

import (
	"context"
	"encoding/json"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"sync"
	"time"
)

type Point struct {
	Time     int64  `json:"time"`
	Protocol string `json:"protocol"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}
type Counts struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}
type Connection struct {
	ID       string `json:"id"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Metadata struct {
		InboundName string `json:"inboundName"`
	} `json:"metadata"`
}
type Snapshot struct {
	Upload      int64        `json:"uploadTotal"`
	Download    int64        `json:"downloadTotal"`
	Connections []Connection `json:"connections"`
}
type Collector struct {
	Store *store.Store
	Core  interface {
		Running() bool
		Request(context.Context, string, any) error
	}
	State       func() model.State
	mu          sync.Mutex
	last        map[string]Counts
	totals      Counts
	initialized bool
	lastAt      time.Time
	current     []Point
	Online      bool
}

func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Sample(ctx)
		}
	}
}
func delta(now, old int64) int64 {
	if now < old {
		return now
	}
	return now - old
}
func (c *Collector) Sample(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var snapshot Snapshot
	if !c.Core.Running() || c.Core.Request(ctx, "/connections", &snapshot) != nil {
		c.Online = false
		c.initialized = false
		c.last = nil
		c.current = nil
		return
	}
	c.Online = true
	now := time.Now()
	timestamp := now.Unix()
	state := c.State()
	byName := map[string]string{}
	for _, e := range state.Endpoints {
		byName["pf-"+e.ID] = e.Protocol
	}
	bucket := map[string]Counts{}
	next := map[string]Counts{}
	for _, conn := range snapshot.Connections {
		current := Counts{conn.Upload, conn.Download}
		next[conn.ID] = current
		previous := c.last[conn.ID]
		p := byName[conn.Metadata.InboundName]
		if p == "" {
			p = "unknown"
		}
		b := bucket[p]
		b.Upload += delta(current.Upload, previous.Upload)
		b.Download += delta(current.Download, previous.Download)
		bucket[p] = b
	}
	current := Counts{snapshot.Upload, snapshot.Download}
	if !c.initialized {
		c.initialized = true
		c.totals = current
		c.last = next
		c.lastAt = now
		return
	}
	bucket["total"] = Counts{delta(current.Upload, c.totals.Upload), delta(current.Download, c.totals.Download)}
	c.totals = current
	c.last = next
	elapsed := now.Sub(c.lastAt).Seconds()
	c.lastAt = now
	c.current = []Point{}
	tx, err := c.Store.DB.Begin()
	if err != nil {
		return
	}
	for protocol, v := range bucket {
		c.current = append(c.current, Point{timestamp, protocol, int64(float64(v.Upload) / elapsed), int64(float64(v.Download) / elapsed)})
		if _, err = tx.Exec("INSERT INTO samples(time,protocol,upload,download) VALUES(?,?,?,?) ON CONFLICT(time,protocol) DO UPDATE SET upload=upload+excluded.upload,download=download+excluded.download", timestamp, protocol, v.Upload, v.Download); err != nil {
			_ = tx.Rollback()
			return
		}
	}
	_, err = tx.Exec("DELETE FROM samples WHERE time < ?", now.AddDate(0, 0, -state.RetentionDays).Unix())
	if err != nil {
		_ = tx.Rollback()
		return
	}
	_ = tx.Commit()
}
func (c *Collector) Summary() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	points := c.current
	if points == nil || !c.Online {
		points = []Point{}
	}
	return map[string]any{"online": c.Online, "rates": points, "sampledAt": c.lastAt, "intervalSeconds": 5, "protocolAccounting": "sampled-active-connections"}
}
func (c *Collector) History(since int64) ([]Point, error) {
	rows, err := c.Store.DB.Query("SELECT (time/300)*300,protocol,SUM(upload),SUM(download) FROM samples WHERE time>=? GROUP BY time/300,protocol ORDER BY time/300", since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []Point{}
	for rows.Next() {
		var p Point
		if err = rows.Scan(&p.Time, &p.Protocol, &p.Upload, &p.Download); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
func DecodeSnapshot(b []byte) (Snapshot, error) {
	var s Snapshot
	err := json.Unmarshal(b, &s)
	return s, err
}
