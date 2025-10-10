package e2e

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

/* ============================================================================
   Proc & repo helpers
============================================================================ */

type serverProc struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

func (p *serverProc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	// Intento 1: señal controlada (Windows carece de SIGINT directa vía os.Process.Signal en go<1.21, hacemos Kill fallback)
	// Para Linux/Mac se podría usar: p.cmd.Process.Signal(os.Interrupt)
	// Aquí: intentamos Wait con timeout y luego Kill forzado.
	done := make(chan struct{})
	go func() {
		_, _ = p.cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-time.After(2 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	case <-done:
	}
	// Esperar liberación de puerto 8080/8081 (best-effort)
	waitPortFreed("8080", 2*time.Second)
	waitPortFreed("8081", 2*time.Second)
}

// waitPortFreed intenta conectar; cuando falla asume puerto liberado.
func waitPortFreed(port string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 150*time.Millisecond)
		if err != nil { // puerto cerrado
			return
		}
		_ = conn.Close()
		time.Sleep(150 * time.Millisecond)
	}
}

// findRepoRoot: sube directorios hasta encontrar go.mod (máx 8 niveles)
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod no encontrado desde %s", dir)
}

// pickFreePort binds to 127.0.0.1:0 to let the OS select a free port, then closes it and returns the port.
func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr := ln.Addr().String()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	return p, nil
}

func startServer(ctx context.Context, envFile string) (*serverProc, string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, "", err
	}
	// Choose a dedicated free port for this test run to avoid colliding with any pre-existing server
	port, err := pickFreePort()
	if err != nil {
		return nil, "", fmt.Errorf("pick free port: %w", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	// Bind on 127.0.0.1 but advertise and use localhost for baseURL to ensure cookies and host-based logic align
	base := "http://localhost:" + strconv.Itoa(port)
	args := []string{"run", "./cmd/service", "-env"}
	if envFile != "" {
		args = append(args, "-env-file", envFile)
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	// Ensure the spawned service binds to our chosen port and advertises the same base URL
	env := append(os.Environ(), "GOFLAGS=-count=1") // no cache durante e2e
	env = append(env,
		"SERVER_ADDR="+addr,
		"JWT_ISSUER="+base,
		"EMAIL_BASE_URL="+base,
		// Ensure cookies in dev attach to localhost
		"AUTH_SESSION_DOMAIN=localhost",
	)
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start service: %w", err)
	}
	return &serverProc{cmd: cmd, out: &out}, base, nil
}

// runCmd ejecuta "go <args...>" en el root del repo (go.mod)
func runCmd(ctx context.Context, _ string, args ...string) (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("run %v: %w\n%s", args, err, out.String())
	}
	return out.String(), nil
}

/* ============================================================================
   Seed loader
============================================================================ */

func mustLoadSeedYAML() (*seedData, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "cmd", "seed", "seed_data.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s seedData
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	// Backward compatibility: some seed files used 'sub' instead of 'id' for admin user
	if s.Users.Admin.ID == "" && s.Users.Admin.Sub != "" {
		// normalize
		s.Users.Admin.ID = s.Users.Admin.Sub
	}
	return &s, nil
}

/* ============================================================================
   HTTP utils
============================================================================ */

func waitReady(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/readyz")
		if err == nil && resp.StatusCode == 200 {
			_ = resp.Body.Close()
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("readyz timed out after %s", timeout)
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
	}
}

func readHeader(resp *http.Response, name string) string {
	for k, v := range resp.Header {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func mustJSON(r io.Reader, v any) error {
	return json.NewDecoder(bufio.NewReader(r)).Decode(v)
}

// Pequeño helper por si algún test quiere serializar rápido
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

/* ============================================================================
   Utils genéricos: email único, QS, base64url, PKCE, JWT decode
============================================================================ */

// e-mails únicos (plus addressing)
func uniqueEmail(base, tag string) string {
	suffix := time.Now().UnixNano() % 1_000_000_000
	if i := strings.IndexByte(base, '@'); i > 0 && i < len(base)-1 {
		local := base[:i]
		domain := base[i+1:]
		// si ya trae +..., recortar para no acumular tags
		if j := strings.IndexByte(local, '+'); j >= 0 {
			local = local[:j]
		}
		return local + "+" + tag + "-" + strconv.FormatInt(suffix, 10) + "@" + domain
	}
	return "e2e+" + tag + "-" + strconv.FormatInt(suffix, 10) + "@example.test"
}

// itoa simple para timestamps
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// querystring util
func qs(u, key string) string {
	uu, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return uu.Query().Get(key)
}

// base64url helpers
func b64url(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.TrimRight(s, "=")
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}
func b64urlDecode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}

// PKCE + at_hash
func pkceS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return b64url(h[:])
}
func newCodeVerifier() string {
	raw := "v" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sum := sha256.Sum256([]byte(raw))
	return b64url(sum[:]) // >=43 chars
}
func atHash(access string) string {
	h := sha256.Sum256([]byte(access))
	return b64url(h[:16])
}

// JWT decoders (sin verificar firma)
func decodeJWT(jwt string) (map[string]any, map[string]any, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil, nil, fmt.Errorf("invalid token format")
	}
	hb, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, nil, err
	}
	pb, err := b64urlDecode(parts[1])
	if err != nil {
		return nil, nil, err
	}
	var hdr, pld map[string]any
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(pb, &pld); err != nil {
		return nil, nil, err
	}
	return hdr, pld, nil
}

// DecodeJWTUnverified is a small exported wrapper used by some tests to parse header/payload without verifying.
func DecodeJWTUnverified(_ *testing.T, jwt string) (map[string]any, map[string]any) {
	h, p, _ := decodeJWT(jwt)
	return h, p
}
func jwtHeaderPayload(jwt string) (map[string]any, map[string]any) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil, nil
	}
	hb, _ := b64urlDecode(parts[0])
	pb, _ := b64urlDecode(parts[1])
	var hdr, pld map[string]any
	_ = json.Unmarshal(hb, &hdr)
	_ = json.Unmarshal(pb, &pld)
	return hdr, pld
}

// GetKID extracts the kid from a JWT header for tests.
func GetKID(_ *testing.T, jwt string) string {
	h, _ := jwtHeaderPayload(jwt)
	if h == nil {
		return ""
	}
	if s, _ := h["kid"].(string); s != "" {
		return s
	}
	return ""
}
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
