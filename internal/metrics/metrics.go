package metrics

import (
	"context"
	"encoding/json"
	"net/netip"
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
		InboundUser string `json:"inboundUser"`
		SourceIP    string `json:"sourceIP"`
		Network     string `json:"network"`
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
	State         func() model.State
	HistoryPolicy func() model.TrafficHistoryPolicy
	mu            sync.Mutex
	last          map[string]Counts
	totals        Counts
	initialized   bool
	lastAt        time.Time
	current       []Point
	connections   map[string]int
	Online        bool
	writeFailed   bool
	maintenance   historyMaintenance
}

func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	c.Sample(ctx)
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

func NormalizeIP(value string) string {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return ""
	}
	return ip.WithZone("").Unmap().String()
}

type dimensions struct {
	protocol, ip, inbound, user, network string
}

func (c *Collector) Sample(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.State()
	policy := state.HistoryPolicy()
	if c.HistoryPolicy != nil {
		policy = c.HistoryPolicy()
	}
	c.maintainHistory(ctx, time.Now(), policy)

	var snapshot Snapshot
	running := c.Core.Running()
	if !running || c.Core.Request(ctx, "/connections", &snapshot) != nil {
		c.Online = false
		if !running {
			c.initialized = false
			c.last = nil
		}
		c.current = nil
		c.connections = nil
		return
	}
	c.Online = true
	now := time.Now()
	timestamp := now.Unix()
	byName := make(map[string]model.Endpoint, len(state.Endpoints))
	for _, e := range state.Endpoints {
		byName["pf-"+e.ID] = e
	}
	users := make(map[string]bool, len(state.Users))
	for _, user := range state.Users {
		users[user.ID] = true
	}
	bucket := map[string]Counts{}
	details := map[dimensions]Counts{}
	next := map[string]Counts{}
	connections := map[string]int{"total": len(snapshot.Connections)}
	for _, conn := range snapshot.Connections {
		current := Counts{conn.Upload, conn.Download}
		next[conn.ID] = current
		previous := c.last[conn.ID]
		entry := byName[conn.Metadata.InboundName]
		p := entry.Protocol
		if p == "" {
			p = "unknown"
		}
		connections[p]++
		up, down := delta(current.Upload, previous.Upload), delta(current.Download, previous.Download)
		b := bucket[p]
		b.Upload += up
		b.Download += down
		bucket[p] = b
		if up == 0 && down == 0 {
			continue
		}
		key := dimensions{protocol: p, ip: NormalizeIP(conn.Metadata.SourceIP), inbound: entry.ID}
		if !entry.Native && users[conn.Metadata.InboundUser] {
			key.user = conn.Metadata.InboundUser
		}
		if network := conn.Metadata.Network; network == "tcp" || network == "udp" {
			key.network = network
		}
		detail := details[key]
		detail.Upload += up
		detail.Download += down
		details[key] = detail
	}
	c.connections = connections
	current := Counts{snapshot.Upload, snapshot.Download}
	if !c.initialized {
		c.initialized = true
		c.totals = current
		c.last = next
		c.lastAt = now
		return
	}
	bucket["total"] = Counts{delta(current.Upload, c.totals.Upload), delta(current.Download, c.totals.Download)}
	elapsed := now.Sub(c.lastAt).Seconds()
	c.current = make([]Point, 0, len(bucket))
	for protocol, v := range bucket {
		c.current = append(c.current, Point{timestamp, protocol, int64(float64(v.Upload) / elapsed), int64(float64(v.Download) / elapsed)})
	}
	// 持久化成功后才推进计数基线，避免临时写入失败使长连接的增量丢失。
	if err := c.saveMinute(ctx, now.Truncate(time.Minute).Unix(), bucket, details); err != nil {
		c.writeFailed = true
		return
	}
	c.writeFailed = false
	c.maintenance.writes += len(bucket) + len(details)
	c.totals = current
	c.last = next
	c.lastAt = now
}
func (c *Collector) Summary() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	points := c.current
	if points == nil || !c.Online {
		points = []Point{}
	}
	return map[string]any{
		"online": c.Online, "rates": points, "activeConnections": c.connections, "sampledAt": c.lastAt,
		"intervalSeconds": 5, "protocolAccounting": "sampled-active-connections", "historyWriteFailed": c.writeFailed,
		"historyStorage": c.maintenance.storage, "historyMaintenanceFailed": c.maintenance.failed,
	}
}
func (c *Collector) History(since int64) ([]Point, error) {
	return c.BucketedHistory(since, 300)
}

func (c *Collector) BucketedHistory(since, bucketSeconds int64) ([]Point, error) {
	rows, err := c.Store.DB.Query("SELECT (time/?2)*?2,protocol,SUM(upload),SUM(download) FROM samples WHERE time>=?1 GROUP BY 1,2 ORDER BY 1,2", since, bucketSeconds)
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
