package hosttraffic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"sync"
	"time"
)

type savedUsage struct {
	Counters         Counters
	Start            int64
	Upload, Download int64
	SampledAt        time.Time
}
type Summary struct {
	Upload    int64     `json:"upload"`
	Download  int64     `json:"download"`
	Used      int64     `json:"used"`
	Total     int64     `json:"total"`
	ResetAt   int64     `json:"resetAt"`
	Interface string    `json:"interface"`
	Available bool      `json:"available"`
	Message   string    `json:"message"`
	SampledAt time.Time `json:"sampledAt"`
}
type Monitor struct {
	Store    *store.Store
	Settings func() model.TrafficSettings
	mu       sync.Mutex
	read     func(string) (Counters, error)
	now      func() time.Time
}

func cycle(now time.Time, t model.TrafficSettings) (time.Time, time.Time) {
	t = t.Normalized()
	zone, _ := time.LoadLocation(t.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	now = now.In(zone)
	date := func(y int, m time.Month) time.Time {
		last := time.Date(y, m+1, 0, 0, 0, 0, 0, zone).Day()
		day := t.ResetDay
		if day > last {
			day = last
		}
		return time.Date(y, m, day, 0, 0, 0, 0, zone)
	}
	start := date(now.Year(), now.Month())
	if now.Before(start) {
		previous := time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, zone)
		start = date(previous.Year(), previous.Month())
	}
	nextMonth := time.Date(start.Year(), start.Month()+1, 1, 0, 0, 0, 0, zone)
	return start, date(nextMonth.Year(), nextMonth.Month())
}
func increment(current, previous uint64) int64 {
	if current < previous {
		return int64(current)
	}
	return int64(current - previous)
}

// 只在成功持久化后返回新用量；重启沿用基线，首次启用不会把整机历史当成本周期流量。
func (m *Monitor) Sample() Summary {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	settings := m.Settings().Normalized()
	start, next := cycle(now, settings)
	usage := savedUsage{}
	err := m.Store.Get("host-traffic", &usage)
	result := Summary{Total: settings.MaxBytes, ResetAt: next.Unix()}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		result.Message = "流量记录读取失败"
		return result
	}
	if usage.Start != start.Unix() {
		usage.Upload = 0
		usage.Download = 0
	}
	result.Upload, result.Download, result.SampledAt, result.Interface = usage.Upload, usage.Download, usage.SampledAt, usage.Counters.Interface
	read := m.read
	if read == nil {
		read = readCounters
	}
	current, readErr := read(settings.Interface)
	if readErr != nil {
		result.Message = readErr.Error()
		result.Used = result.Upload + result.Download
		if settings.OutboundOnly {
			result.Download = 0
			result.Used = result.Upload
		}
		return result
	}
	if !usage.SampledAt.IsZero() && !usage.SampledAt.Before(start) && usage.Counters.Interface == current.Interface {
		if usage.Counters.BootID == current.BootID {
			usage.Upload += increment(current.Upload, usage.Counters.Upload)
			usage.Download += increment(current.Download, usage.Counters.Download)
		} else {
			usage.Upload += int64(current.Upload)
			usage.Download += int64(current.Download)
		}
	}
	usage.Counters = current
	usage.Start = start.Unix()
	usage.SampledAt = now
	if err = m.Store.Set("host-traffic", usage); err != nil {
		result.Message = "流量记录保存失败"
		return result
	}
	result.Available = true
	result.Upload = usage.Upload
	result.Download = usage.Download
	result.Interface = current.Interface
	result.SampledAt = now
	if settings.OutboundOnly {
		result.Download = 0
	}
	result.Used = result.Upload + result.Download
	result.Message = "服务器网卡累计 · 全用户共享"
	return result
}
func (s Summary) Header() string {
	return fmt.Sprintf("upload=%d; download=%d; total=%d", s.Upload, s.Download, s.Total)
}
func (m *Monitor) Run(ctx context.Context) {
	m.Sample()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Sample()
		}
	}
}
