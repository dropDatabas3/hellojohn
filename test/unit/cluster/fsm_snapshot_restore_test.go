package cluster_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/fs"
)

// helper to expand tar.gz bytes into dir (used for assertion if ever needed)
func untarGz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

func TestFSM_SnapshotRestore_RoundTrip(t *testing.T) {
	tmpSrc, err := os.MkdirTemp("", "hj-fsm-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpSrc)
	tmpDst, err := os.MkdirTemp("", "hj-fsm-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDst)

	// Configure FS providers for src and dst
	src := cpfs.New(tmpSrc)
	dst := cpfs.New(tmpDst)

	cpctx.Provider = src

	// Seed src with a tenant, scope and client
	if err := src.UpsertTenant(context.Background(), &cp.Tenant{Name: "Acme Inc", Slug: "acme"}); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	_ = src.UpsertScope(context.Background(), "acme", cp.Scope{Name: "openid"})
	_, _ = src.UpsertClient(context.Background(), "acme", cp.ClientInput{
		Name:         "Web",
		ClientID:     "web",
		Type:         cp.ClientTypePublic,
		RedirectURIs: []string{"http://localhost/cb"},
		Scopes:       []string{"openid"},
	})

	// Create snapshot
	fsm := cluster.NewFSM()
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// Persist into an in-memory buffer via a fake sink
	sink := &bufSink{}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// Now restore into dst
	cpctx.Provider = dst
	// Provide the stream via a reader
	if err := fsm.Restore(io.NopCloser(bytesReader(sink.buf.Bytes()))); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Compare directories tenants/ and keys/
	if err := dirEqual(filepath.Join(tmpSrc, "tenants"), filepath.Join(tmpDst, "tenants")); err != nil {
		t.Fatalf("tenants dir mismatch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpSrc, "keys")); err == nil {
		if err := dirEqual(filepath.Join(tmpSrc, "keys"), filepath.Join(tmpDst, "keys")); err != nil {
			t.Fatalf("keys dir mismatch: %v", err)
		}
	}

	// Read back through provider to ensure functional equivalence
	sTenant, _ := src.GetTenantBySlug(context.Background(), "acme")
	dTenant, _ := dst.GetTenantBySlug(context.Background(), "acme")
	sj, _ := json.Marshal(sTenant)
	dj, _ := json.Marshal(dTenant)
	if string(sj) != string(dj) {
		t.Fatalf("tenant mismatch after restore:\nsrc=%s\ndst=%s", sj, dj)
	}
}

// Helpers below

type bufSink struct{ buf bytes.Buffer }

func (p *bufSink) ID() string                  { return "buf" }
func (p *bufSink) Cancel() error               { p.buf.Reset(); return nil }
func (p *bufSink) Close() error                { return nil }
func (p *bufSink) Write(b []byte) (int, error) { return p.buf.Write(b) }

// bytesReader wraps a byte slice as io.Reader
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// dirEqual recursively compares file structure and contents
func dirEqual(a, b string) error {
	return filepath.Walk(a, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(a, path)
		other := filepath.Join(b, rel)
		oi, oerr := os.Stat(other)
		if oerr != nil {
			return oerr
		}
		if info.IsDir() != oi.IsDir() {
			return fmt.Errorf("type mismatch for %s", rel)
		}
		if !info.IsDir() {
			ab, _ := os.ReadFile(path)
			bb, _ := os.ReadFile(other)
			if string(ab) != string(bb) {
				return fmt.Errorf("content mismatch for %s", rel)
			}
		}
		return nil
	})
}
