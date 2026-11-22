package metrics

import (
	"context"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"slices"
	"testing"
	"time"
)

func TestTrafficDetailsSurviveReopenAndPreserveLegacyTotals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DB.Close() })
	state := model.Default()
	state.Endpoints = []model.Endpoint{{ID: "entry", Protocol: "vless"}}
	state.Users = []model.User{{ID: "user"}}
	snapshot, err := DecodeSnapshot([]byte(`{"uploadTotal":20,"downloadTotal":40,"connections":[
		{"id":"first","upload":20,"download":40,"metadata":{"inboundName":"pf-entry","inboundUser":"user","sourceIP":"::ffff:192.0.2.1","network":"tcp"}}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	input := &source{online: true, snapshot: snapshot}
	c := &Collector{Store: db, Core: input, State: func() model.State { return state }}
	c.Sample(ctx)
	for range 2 {
		input.snapshot.Upload += 100
		input.snapshot.Download += 200
		input.snapshot.Connections[0].Upload += 30
		input.snapshot.Connections[0].Download += 60
		c.Sample(ctx)
	}
	now := time.Now().Truncate(time.Minute)
	legacy := now.Add(-5 * time.Minute)
	if _, err := db.DB.Exec("INSERT INTO samples(time,protocol,upload,download) VALUES(?,?,?,?)", legacy.Unix()+5, "total", 10, 20); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.DB.Close()
	collector := &Collector{Store: reopened}
	filter := HistoryFilter{From: legacy, Until: now.Add(time.Minute)}
	bounds := []int64{filter.From.Unix(), now.Add(-time.Minute).Unix(), filter.Until.Unix()}
	points, err := collector.CalendarHistory(ctx, filter, bounds)
	want := []HistoryBucket{
		{Time: bounds[0], End: bounds[1], Counts: Counts{10, 20}},
		{Time: bounds[1], End: bounds[2], Counts: Counts{200, 400}},
	}
	if err != nil || !slices.Equal(points, want) {
		t.Fatalf("历史聚合错误：%+v %v", points, err)
	}
	for group, value := range map[string]string{"ip": "192.0.2.1", "inbound": "entry", "user": "user", "network": "tcp", "protocol": "vless"} {
		result, err := collector.UsageBreakdown(ctx, filter, group, 0)
		if err != nil || !slices.Equal(result.Rows, []UsageRow{{Value: value, Counts: Counts{60, 120}}}) {
			t.Fatalf("%s 分组错误：%+v %v", group, result, err)
		}
	}
	filter.IP, filter.Network = "192.0.2.1", "tcp"
	points, err = collector.CalendarHistory(ctx, filter, bounds)
	want = []HistoryBucket{{Time: bounds[1], End: bounds[2], Counts: Counts{60, 120}}}
	if err != nil || !slices.Equal(points, want) {
		t.Fatalf("多条件筛选错误：%+v %v", points, err)
	}
	filter.Network = "udp"
	if points, err = collector.CalendarHistory(ctx, filter, bounds); err != nil || len(points) != 0 {
		t.Fatalf("筛选应无匹配结果：%+v %v", points, err)
	}
}

func TestTrafficHistoryRetentionWhileOffline(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	days := 0
	collector := &Collector{
		Store: db, Core: &source{}, State: model.Default,
		HistoryPolicy: func() model.TrafficHistoryPolicy {
			return model.TrafficHistoryPolicy{Days: days, LimitMiB: model.DefaultTrafficHistoryMiB}
		},
	}
	ctx := context.Background()
	now := time.Now().Truncate(time.Minute)
	for _, minute := range []time.Time{now.Add(-72 * time.Hour), now} {
		if err := collector.saveMinute(ctx, minute.Unix(), map[string]Counts{"total": {Upload: 1}},
			map[dimensions]Counts{{protocol: "vless", ip: "2001:db8::1"}: {Download: 2}}); err != nil {
			t.Fatal(err)
		}
	}
	assertRows := func(want int) {
		t.Helper()
		for _, table := range []string{"samples", "traffic_details"} {
			var count int
			if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != want {
				t.Fatalf("%s 应有 %d 条记录，实际 %d：%v", table, want, count, err)
			}
		}
	}
	collector.Sample(ctx)
	assertRows(2)
	days = 1
	collector.Sample(ctx)
	assertRows(1)
}
