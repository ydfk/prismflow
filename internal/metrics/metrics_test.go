package metrics

import (
	"context"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"testing"
)

type source struct {
	snapshot Snapshot
	online   bool
}

func (s *source) Running() bool { return s.online }
func (s *source) Request(_ context.Context, _ string, v any) error {
	*v.(*Snapshot) = s.snapshot
	return nil
}

func TestSamplingBaselineDeltasAndRestart(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Close()
	state := model.Default()
	state.Endpoints = []model.Endpoint{{ID: "entry", Protocol: "vless"}}
	conn := Connection{ID: "connection", Upload: 100, Download: 200}
	conn.Metadata.InboundName = "pf-entry"
	input := &source{online: true, snapshot: Snapshot{Upload: 1000, Download: 2000, Connections: []Connection{conn}}}
	c := &Collector{Store: db, Core: input, State: func() model.State { return state }}
	c.Sample(context.Background())
	input.snapshot.Upload += 50
	input.snapshot.Download += 70
	input.snapshot.Connections[0].Upload += 20
	input.snapshot.Connections[0].Download += 30
	c.Sample(context.Background())
	points, err := c.History(0)
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]Counts{}
	for _, p := range points {
		totals[p.Protocol] = Counts{p.Upload, p.Download}
	}
	if totals["total"] != (Counts{50, 70}) || totals["vless"] != (Counts{20, 30}) {
		t.Fatalf("incorrect deltas: %+v", totals)
	}
	input.online = false
	c.Sample(context.Background())
	input.online = true
	input.snapshot = Snapshot{Upload: 5, Download: 8}
	c.Sample(context.Background())
	if len(c.Summary()["rates"].([]Point)) != 0 {
		t.Fatal("stale rates after restart")
	}
	input.snapshot.Upload = 15
	input.snapshot.Download = 18
	c.Sample(context.Background())
	points, err = c.History(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range points {
		if p.Protocol == "total" && (p.Upload != 60 || p.Download != 80) {
			t.Fatalf("restart double counted: %+v", p)
		}
	}
}
