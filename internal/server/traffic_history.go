package server

import (
	"errors"
	"net/http"
	"net/url"
	"prismflow/internal/metrics"
	"strconv"
	"strings"
	"time"
)

func trafficHistoryFilter(query url.Values) (metrics.HistoryFilter, error) {
	zone := time.UTC
	if name := query.Get("timezone"); name != "" {
		var err error
		zone, err = time.LoadLocation(name)
		if err != nil || name == "Local" {
			return metrics.HistoryFilter{}, errors.New("请选择有效的统计时区")
		}
	}
	start, startErr := time.Parse(time.DateOnly, query.Get("from"))
	end, endErr := time.Parse(time.DateOnly, query.Get("to"))
	if startErr != nil || endErr != nil || start.Year() < 1970 || end.Before(start) || end.Sub(start) >= 366*24*time.Hour {
		return metrics.HistoryFilter{}, errors.New("请选择有效的起止日期，每次最多查询 366 天")
	}
	filter := metrics.HistoryFilter{
		From: localDayStart(start, zone), Until: localDayStart(end.AddDate(0, 0, 1), zone),
		Protocol: query.Get("protocol"), Inbound: query.Get("inbound"), User: query.Get("user"), Network: query.Get("network"),
	}
	if ip := strings.TrimSpace(query.Get("ip")); ip != "" {
		filter.IP = metrics.NormalizeIP(ip)
		if filter.IP == "" {
			return metrics.HistoryFilter{}, errors.New("请输入有效的 IPv4 或 IPv6 地址")
		}
	}
	if filter.Protocol == "total" || len(filter.Protocol) > 64 || len(filter.Inbound) > 128 || len(filter.User) > 128 ||
		(filter.Network != "" && filter.Network != "tcp" && filter.Network != "udp") {
		return metrics.HistoryFilter{}, errors.New("流量筛选条件无效")
	}
	return filter, nil
}

func localDayStart(date time.Time, zone *time.Location) time.Time {
	midnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, zone)
	start, end := midnight.ZoneBounds()
	if midnight.Format(time.DateOnly) < date.Format(time.DateOnly) && !end.IsZero() {
		// 时区切换跳过当地午夜时，以切换后的第一刻作为当天起点。
		return end
	}
	if !start.IsZero() && start.Add(-time.Nanosecond).Format(time.DateOnly) == date.Format(time.DateOnly) {
		_, before := start.Add(-time.Nanosecond).Zone()
		_, after := start.Zone()
		return midnight.Add(time.Duration(after-before) * time.Second)
	}
	return midnight
}

func historyBounds(filter metrics.HistoryFilter, granularity string) ([]int64, error) {
	var advance func(time.Time) time.Time
	switch granularity {
	case "minute":
		advance = func(t time.Time) time.Time { return t.Add(time.Minute) }
	case "hour":
		advance = func(t time.Time) time.Time {
			next := t.Add(time.Duration(60-t.Minute()) * time.Minute)
			if _, end := t.ZoneBounds(); !end.IsZero() && end.Before(next) {
				return end
			}
			return next
		}
	case "day":
		advance = func(t time.Time) time.Time {
			date := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
			return localDayStart(date, t.Location())
		}
	default:
		return nil, errors.New("流量统计粒度必须为分钟、小时或天")
	}
	var bounds []int64
	for t := filter.From; t.Before(filter.Until); t = advance(t) {
		if len(bounds) == metrics.MaxHistoryBuckets {
			return nil, errors.New("所选时段过长，请缩短日期范围或切换到更粗的统计粒度")
		}
		bounds = append(bounds, t.Unix())
	}
	return append(bounds, filter.Until.Unix()), nil
}

func (s *Server) calendarHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	filter, err := trafficHistoryFilter(query)
	if err != nil {
		fail(w, 400, err)
		return
	}
	bounds, err := historyBounds(filter, query.Get("granularity"))
	if err != nil {
		fail(w, 400, err)
		return
	}
	buckets, err := s.Metrics.CalendarHistory(r.Context(), filter, bounds)
	if err != nil {
		fail(w, 500, errors.New("读取流量历史失败"))
		return
	}
	respond(w, 200, map[string]any{"from": bounds[0], "until": bounds[len(bounds)-1], "buckets": buckets})
}

func (s *Server) trafficBreakdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	filter, err := trafficHistoryFilter(query)
	if err != nil {
		fail(w, 400, err)
		return
	}
	group := query.Get("group")
	page, err := strconv.Atoi(query.Get("page"))
	if !metrics.HistoryGroupValid(group) || err != nil || page < 1 || page > 100000 {
		fail(w, 400, errors.New("流量分组或页码无效"))
		return
	}
	if slot := query.Get("slot"); slot != "" {
		bounds, boundsErr := historyBounds(filter, query.Get("granularity"))
		start, parseErr := strconv.ParseInt(slot, 10, 64)
		if boundsErr == nil && parseErr == nil {
			for i := 0; i+1 < len(bounds); i++ {
				if bounds[i] == start {
					filter.From = time.Unix(bounds[i], 0)
					filter.Until = time.Unix(bounds[i+1], 0)
					slot = ""
					break
				}
			}
		}
		if slot != "" {
			fail(w, 400, errors.New("所选流量时段无效"))
			return
		}
	}
	result, err := s.Metrics.UsageBreakdown(r.Context(), filter, group, (page-1)*metrics.BreakdownPageSize)
	if err != nil {
		fail(w, 500, errors.New("读取流量分组失败"))
		return
	}
	respond(w, 200, result)
}
