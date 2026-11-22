package metrics

import (
	"context"
	"math"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"testing"
	"time"
)

func TestHistoryBudgetEvictsOldestWholeMinutesAndKeepsSummary(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	_, err = db.DB.Exec(`
		INSERT INTO samples(time,protocol,upload,download) VALUES (60,'total',1,2),(120,'total',3,4);
		WITH RECURSIVE ids(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM ids WHERE i<2999)
		INSERT INTO traffic_details(time,protocol,source_ip,inbound,user_id,network,upload,download)
		SELECT 60*(1+i/20),'vless',printf('2001:db8::%x',i),'entry','user','tcp',1,2 FROM ids;`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	summary, details, err := measureHistory(ctx, tx)
	if err != nil || details.bytes < 100<<10 {
		t.Fatalf("未能测得明细占用：%+v %v", details, err)
	}

	const limit = 128 << 10
	var storage HistoryStorage
	for range 8 {
		var pending bool
		storage, _, pending, err = trimHistory(ctx, tx, math.MinInt64, limit)
		if err != nil {
			t.Fatal(err)
		}
		if !pending {
			break
		}
	}
	if storage.SummaryBytes != summary.bytes || storage.OldestSummaryAt != 60 ||
		storage.DetailBytes > limit*3/4 || storage.OldestDetailAt <= 60 || storage.OldestDetailAt >= 9000 {
		t.Fatalf("容量回收后记录不符合预期：%+v", storage)
	}
	var entries int
	if err := tx.QueryRow("SELECT COUNT(*) FROM traffic_details WHERE time=?", storage.OldestDetailAt).Scan(&entries); err != nil || entries != 20 {
		t.Fatalf("最早保留的时间桶应完整，获得 %d 条：%v", entries, err)
	}
}

func TestHistoryMaintenanceReclaimsPhysicalPagesOnNewDatabase(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	now := time.Now()
	_, err = db.DB.Exec(`
		WITH RECURSIVE ids(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM ids WHERE i<1999)
		INSERT INTO traffic_details(time,protocol,source_ip,inbound,user_id,network,upload,download)
		SELECT 1,'vless',printf('2001:db8::%x',i),hex(zeroblob(400)),'user','tcp',1,2 FROM ids;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	var pagesBefore int64
	if err := db.DB.QueryRow("PRAGMA page_count").Scan(&pagesBefore); err != nil {
		t.Fatal(err)
	}
	bytesBefore, err := db.FileBytes()
	if err != nil {
		t.Fatal(err)
	}
	collector := &Collector{Store: db}
	storage, _, err := collector.cleanHistory(context.Background(), now, model.TrafficHistoryPolicy{Days: 1, LimitMiB: 32})
	var pagesAfter int64
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("PRAGMA page_count").Scan(&pagesAfter); err != nil || pagesAfter >= pagesBefore ||
		storage.OldestDetailAt != 0 || storage.DatabaseBytes >= bytesBefore {
		t.Fatalf("回收结果错误：页数 %d -> %d，状态 %+v，错误 %v", pagesBefore, pagesAfter, storage, err)
	}
}
