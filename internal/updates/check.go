package updates

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ReleasesURL = "https://api.github.com/repos/ydfk/prismflow/releases/latest"

var stable = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Release struct {
	Version   string    `json:"version"`
	URL       string    `json:"url"`
	Available bool      `json:"available"`
	CheckedAt time.Time `json:"checkedAt"`
}

func Newer(current, latest string) bool {
	b := stable.FindStringSubmatch(latest)
	if b == nil {
		return false
	}
	// 开发版与预发布版可切换至相同或更新的稳定版，但不自动降级。
	base := strings.SplitN(strings.TrimPrefix(current, "v"), "-", 2)[0]
	a := stable.FindStringSubmatch(base)
	if a == nil {
		return true
	}
	for i := 1; i <= 3; i++ {
		av, e1 := strconv.ParseUint(a[i], 10, 64)
		bv, e2 := strconv.ParseUint(b[i], 10, 64)
		if e1 != nil || e2 != nil {
			return false
		}
		if av != bv {
			return bv > av
		}
	}
	return strings.Contains(current, "-")
}
func Check(ctx context.Context, client *http.Client, current, endpoint string) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "PrismFlow-Version-Check")
	response, err := client.Do(request)
	if err != nil {
		return Release{}, errors.New("无法连接 GitHub，请稍后重试")
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return Release{}, errors.New("尚未发布稳定版 Release")
	}
	if response.StatusCode != 200 {
		return Release{}, errors.New("GitHub 版本接口暂不可用或达到访问限额")
	}
	var payload struct {
		Tag        string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil || payload.Prerelease || payload.Draft || !strings.HasPrefix(payload.Tag, "v") || !stable.MatchString(payload.Tag) {
		return Release{}, errors.New("稳定版元数据无效")
	}
	return Release{payload.Tag, "https://github.com/ydfk/prismflow/releases/tag/" + payload.Tag, Newer(current, payload.Tag), time.Now().UTC()}, nil
}
