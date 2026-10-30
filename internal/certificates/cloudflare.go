package certificates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type record struct{ Zone, ID string }
type cloudflare struct {
	token   string
	records map[string]record
}

func (c *cloudflare) call(method, path string, payload any, result any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://api.cloudflare.com/client/v4/"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("Cloudflare API 网络请求失败")
	}
	defer res.Body.Close()
	var response struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&response); err != nil {
		return err
	}
	if !response.Success {
		return fmt.Errorf("Cloudflare 请求失败（HTTP %d），请检查 Zone Read / DNS Edit 权限", res.StatusCode)
	}
	if result != nil {
		return json.Unmarshal(response.Result, result)
	}
	return nil
}
func (c *cloudflare) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)
	name := strings.TrimSuffix(info.EffectiveFQDN, ".")
	var zoneID string
	parts := strings.Split(name, ".")
	for i := 0; i < len(parts)-1; i++ {
		var zones []struct {
			ID string `json:"id"`
		}
		if err := c.call("GET", "zones?name="+url.QueryEscape(strings.Join(parts[i:], ".")), nil, &zones); err != nil {
			return err
		}
		if len(zones) > 0 {
			zoneID = zones[0].ID
			break
		}
	}
	if zoneID == "" {
		return errors.New("Token 未授权对应 DNS Zone")
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := c.call("POST", "zones/"+zoneID+"/dns_records", map[string]any{"type": "TXT", "name": name, "content": info.Value, "ttl": 120}, &created); err != nil {
		return err
	}
	c.records[token] = record{zoneID, created.ID}
	return nil
}
func (c *cloudflare) CleanUp(domain, token, keyAuth string) error {
	r, ok := c.records[token]
	if !ok {
		return nil
	}
	return c.call("DELETE", "zones/"+r.Zone+"/dns_records/"+r.ID, nil, nil)
}
func (c *cloudflare) Timeout() (time.Duration, time.Duration) {
	return 3 * time.Minute, 3 * time.Second
}
