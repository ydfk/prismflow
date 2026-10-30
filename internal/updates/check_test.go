package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionOrdering(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{{"1.2.3", "v1.2.4", true}, {"v1.2.3", "v1.2.3", false}, {"1.9.0", "v1.10.0", true}, {"2.0.0", "v1.9.9", false}, {"1.2.0-rc.1", "v1.2.0", true}, {"1.3.0-rc.1", "v1.2.0", false}, {"dev", "v1.0.0", true}, {"1.0.0", "v2.0.0-rc.1", false}, {"1.0.0", "invalid", false}} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("%s -> %s = %v", tc.current, tc.latest, got)
		}
	}
}
func TestReleaseValidation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		code  int
		valid bool
	}{{`{"tag_name":"v1.2.3","draft":false,"prerelease":false}`, 200, true}, {`{"tag_name":"v1.2.4","draft":false,"prerelease":true}`, 200, false}, {`{"tag_name":"../../tmp","draft":false,"prerelease":false}`, 200, false}, {`{"tag_name":"v1.2.3","draft":true}`, 200, false}, {`{`, 200, false}, {`{}`, 404, false}, {`{}`, 403, false}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); w.Write([]byte(tc.body)) }))
		result, err := Check(context.Background(), server.Client(), "1.0.0", server.URL)
		server.Close()
		if (err == nil) != tc.valid {
			t.Fatalf("unexpected result for %s: %v", tc.body, err)
		}
		if tc.valid && (!result.Available || result.URL != "https://github.com/ydfk/prismflow/releases/tag/v1.2.3") {
			t.Fatal(result)
		}
	}
}
