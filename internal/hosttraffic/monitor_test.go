package hosttraffic

import (
	"errors"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"testing"
	"time"
)

func TestPersistResetAndUnavailable(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	settings := model.TrafficSettings{ResetDay: 31, MaxBytes: 10000}
	counters := Counters{"eth0", "boot", 1000, 2000}
	monitor := &Monitor{Store: db, Settings: func() model.TrafficSettings { return settings }, now: func() time.Time { return now }, read: func(string) (Counters, error) { return counters, nil }}
	if summary := monitor.Sample(); summary.Used != 0 || !summary.Available {
		t.Fatal(summary)
	}
	counters.Upload += 100
	counters.Download += 200
	if summary := monitor.Sample(); summary.Used != 300 {
		t.Fatal(summary)
	}
	// 新进程继续使用数据库中的基线。
	second := &Monitor{Store: db, Settings: monitor.Settings, now: monitor.now, read: monitor.read}
	counters.Upload += 50
	if summary := second.Sample(); summary.Upload != 150 || summary.Download != 200 {
		t.Fatal(summary)
	}
	settings.OutboundOnly = true
	if summary := second.Sample(); summary.Used != 150 || summary.Download != 0 || summary.Header() != "upload=150; download=0; total=10000" {
		t.Fatal(summary)
	}
	// 跨周期未观测的间隙不能把旧周期的流量全部计入新周期。
	now = time.Date(2026, 1, 31, 0, 0, 1, 0, time.UTC)
	counters.Upload += 1000
	if summary := second.Sample(); summary.Used != 0 {
		t.Fatal(summary)
	}
	counters.Upload += 25
	if summary := second.Sample(); summary.Used != 25 {
		t.Fatal(summary)
	}
	second.read = func(string) (Counters, error) { return Counters{}, errors.New("unavailable") }
	if summary := second.Sample(); summary.Available || summary.Used != 25 {
		t.Fatal(summary)
	}
}
func TestMonthEndAndTimezone(t *testing.T) {
	settings := model.TrafficSettings{ResetDay: 31, Timezone: "Asia/Shanghai"}
	now := time.Date(2028, 2, 28, 16, 0, 0, 0, time.UTC)
	start, next := cycle(now, settings)
	if start.Day() != 29 || start.Month() != time.February || next.Day() != 31 || next.Month() != time.March {
		t.Fatal(start, next)
	}
	now = now.Add(-time.Second)
	start, next = cycle(now, settings)
	if start.Day() != 31 || start.Month() != time.January || next.Day() != 29 {
		t.Fatal(start, next)
	}
}
func TestCounterRestartAndInterfaceChange(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	current := Counters{"eth0", "boot1", 1000, 2000}
	monitor := &Monitor{Store: db, Settings: func() model.TrafficSettings { return model.TrafficSettings{} }, read: func(string) (Counters, error) { return current, nil }}
	monitor.Sample()
	current = Counters{"eth0", "boot2", 30, 40}
	if summary := monitor.Sample(); summary.Used != 70 {
		t.Fatal(summary)
	}
	current = Counters{"ens3", "boot2", 5000, 6000}
	if summary := monitor.Sample(); summary.Used != 70 {
		t.Fatal(summary)
	}
	current.Upload += 5
	if summary := monitor.Sample(); summary.Used != 75 {
		t.Fatal(summary)
	}
}
