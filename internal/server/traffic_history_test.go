package server

import (
	"encoding/json"
	"net/url"
	"prismflow/internal/metrics"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTrafficCalendarBoundsUseLocalDays(t *testing.T) {
	for _, test := range []struct {
		date  string
		hours int
	}{
		{"2026-03-08", 23},
		{"2026-11-01", 25},
	} {
		filter, err := trafficHistoryFilter(url.Values{"timezone": {"America/New_York"}, "from": {test.date}, "to": {test.date}})
		if err != nil {
			t.Fatal(err)
		}
		for precision, count := range map[string]int{"minute": test.hours * 60, "hour": test.hours, "day": 1} {
			bounds, err := historyBounds(filter, precision)
			if err != nil || len(bounds) != count+1 || bounds[len(bounds)-1]-bounds[0] != int64(test.hours*3600) {
				t.Fatalf("%s %s 边界错误：%d %v", test.date, precision, len(bounds), err)
			}
		}
	}
}

func TestTrafficHistoryAPIAuthenticatesAndFiltersSelectedSlot(t *testing.T) {
	s := testServer(t)
	handler := s.AdminHandler()
	query := "?from=2026-11-22&to=2026-11-22&timezone=Asia%2FShanghai&granularity=minute"
	if w := call(t, handler, "GET", "/api/metrics/history"+query, nil, nil, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	cookie, csrf := setupLogin(t, s)
	settings := call(t, handler, "PUT", "/api/settings", map[string]int{"retentionDays": 0, "historyLimitMiB": 64}, cookie, csrf)
	if settings.Code != 200 || s.State().RetentionDays != 0 || s.State().HistoryPolicy().LimitMiB != 64 {
		t.Fatal(settings.Code, settings.Body.String())
	}
	settings = call(t, handler, "PUT", "/api/settings", map[string]int{"historyLimitMiB": 31}, cookie, csrf)
	if settings.Code != 400 || s.State().HistoryPolicy().LimitMiB != 64 {
		t.Fatal(settings.Code, settings.Body.String())
	}
	minute := time.Date(2026, 11, 21, 16, 0, 0, 0, time.UTC).Unix()
	_, err := s.Store.DB.Exec(`INSERT INTO traffic_details(time,protocol,source_ip,inbound,user_id,network,upload,download) VALUES
		(?,'vless','2001:db8::1','entry','user','tcp',12,24),
		(?,'vless','2001:db8::1','entry','user','tcp',6,8),
		(?,'vless','192.0.2.2','entry','','udp',20,50)`, minute, minute+60, minute+60)
	if err != nil {
		t.Fatal(err)
	}
	w := call(t, handler, "GET", "/api/metrics/history"+query+"&ip=2001:0DB8::1", nil, cookie, "")
	var history struct {
		Starts  []int64
		Buckets []metrics.HistoryBucket
	}
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(w.Body.Bytes(), &history) != nil ||
		len(history.Starts) != 1440 || history.Starts[0] != minute || history.Starts[2] != minute+120 ||
		len(history.Buckets) != 2 || history.Buckets[0].Time != minute || history.Buckets[1].Upload != 6 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(t, handler, "GET", "/api/metrics/breakdown"+query+"&group=ip&page=1&slot="+strconv.FormatInt(minute, 10), nil, cookie, "")
	var ranking metrics.Breakdown
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ranking) != nil || len(ranking.Rows) != 1 || ranking.Rows[0].Download != 24 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(t, handler, "GET", "/api/metrics/history"+query+"&ip=invalid-address", nil, cookie, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "IPv4") {
		t.Fatal(w.Code, w.Body.String())
	}
}
