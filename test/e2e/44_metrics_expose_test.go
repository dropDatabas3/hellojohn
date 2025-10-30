package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"
)

// 44 - Metrics exposure and change after write
func Test_44_Metrics_Expose_And_Change(t *testing.T) {
	if os.Getenv("STORAGE_DRIVER") == "" || os.Getenv("STORAGE_DSN") == "" {
		t.Skip("requires DB to login admin for admin write")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Single embedded node is fine for this test
	n, base, err := startClusterNode(ctx, 19091, 19291, t.TempDir(), "mn1", map[string]string{"mn1": "127.0.0.1:19291"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if n != nil {
			n.stop()
		}
	})
	if err := waitReady(base, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	getMetric := func(name string) (string, error) {
		resp, err := newHTTPClient().Get(base + "/metrics")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `_count\s+(\d+)`)
		m := re.FindSubmatch(b)
		if len(m) < 2 {
			return "0", nil
		}
		return string(m[1]), nil
	}

	before, _ := getMetric("raft_apply_latency_ms")

	// login admin
	loginBody := map[string]string{
		"tenant_id": seed.Tenant.ID,
		"client_id": seed.Clients.Web.ClientID,
		"email":     seed.Users.Admin.Email,
		"password":  seed.Users.Admin.Password,
	}
	buf, _ := json.Marshal(loginBody)
	resp, err := newHTTPClient().Post(base+"/v1/auth/login", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("login failed: %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		t.Skip("no token")
	}

	// perform an admin write: upsert a tenant
	doc := map[string]any{"slug": "metrics-" + itoa(time.Now().Unix()%1e6), "name": "Metrics"}
	data, _ := json.Marshal(doc)
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/admin/tenants", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := newHTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		after, _ := getMetric("raft_apply_latency_ms")
		if after != before {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("raft_apply_latency_ms_count did not change after admin write (before=%s)", before)
}
