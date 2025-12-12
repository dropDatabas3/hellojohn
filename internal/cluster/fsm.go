package cluster

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	ppath "path"
	"path/filepath"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	cpfsi "github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/hashicorp/raft"
)

// FSM mínima para compilar y permitir pruebas de bootstrapping.
type FSM struct{}

func NewFSM() *FSM { return &FSM{} }

// Apply decodifica la mutación y ejecuta la operación correspondiente sobre el provider FS.
func (f *FSM) Apply(l *raft.Log) interface{} {
	if l == nil || len(l.Data) == 0 {
		return nil
	}
	var m Mutation
	if err := json.Unmarshal(l.Data, &m); err != nil {
		return err
	}
	// rutas por tipo
	switch m.Type {
	case MutationUpsertClient:
		var dto UpsertClientDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		// mapear a ClientInput
		in := cp.ClientInput{
			Name:                     dto.Name,
			ClientID:                 dto.ClientID,
			Type:                     dto.Type,
			RedirectURIs:             dto.RedirectURIs,
			AllowedOrigins:           dto.AllowedOrigins,
			Providers:                dto.Providers,
			Scopes:                   dto.Scopes,
			Secret:                   dto.Secret,
			RequireEmailVerification: dto.RequireEmailVerification,
			ResetPasswordURL:         dto.ResetPasswordURL,
			VerifyEmailURL:           dto.VerifyEmailURL,
		}
		// Ejecutar contra el provider actual (FS en MVP)
		if cpctx.Provider == nil {
			return nil // en tests sin provider configurado devolvemos nil para no panicar
		}
		_, err := cpctx.Provider.UpsertClient(context.Background(), m.TenantSlug, in)
		if err != nil {
			return err
		}
		return nil
	case MutationDeleteClient:
		var dto DeleteClientDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		if err := cpctx.Provider.DeleteClient(context.Background(), m.TenantSlug, dto.ClientID); err != nil {
			return err
		}
		return nil

	case MutationUpsertTenant:
		var dto UpsertTenantDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		t := &cp.Tenant{
			ID:       dto.ID,
			Name:     dto.Name,
			Slug:     dto.Slug,
			Settings: dto.Settings,
			// CreatedAt/UpdatedAt se setean en provider
		}
		if err := cpctx.Provider.UpsertTenant(context.Background(), t); err != nil {
			return err
		}
		return nil

	case MutationUpdateTenantSettings:
		var dto UpdateTenantSettingsDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		// Necesitamos FS-specific para UpdateTenantSettings
		if fs, ok := cpfsi.AsFSProvider(cpctx.Provider); ok {
			if err := fs.UpdateTenantSettings(context.Background(), m.TenantSlug, &dto.Settings); err != nil {
				return err
			}
		}
		return nil

	case MutationDeleteTenant:
		// payload empty
		if cpctx.Provider == nil {
			return nil
		}
		if err := cpctx.Provider.DeleteTenant(context.Background(), m.TenantSlug); err != nil {
			return err
		}
		return nil

	case MutationUpsertScope:
		var dto UpsertScopeDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		s := cp.Scope{Name: dto.Name, Description: dto.Description, System: dto.System}
		if err := cpctx.Provider.UpsertScope(context.Background(), m.TenantSlug, s); err != nil {
			return err
		}
		return nil

	case MutationDeleteScope:
		var dto DeleteScopeDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		if err := cpctx.Provider.DeleteScope(context.Background(), m.TenantSlug, dto.Name); err != nil {
			return err
		}
		return nil

	case MutationRotateTenantKey:
		// Followers write the exact provided JSON blobs to keys/{tenant}/active.json and retiring.json
		var dto RotateTenantKeyDTO
		if err := json.Unmarshal(m.Payload, &dto); err != nil {
			return err
		}
		if cpctx.Provider == nil {
			return nil
		}
		// Only FS-backed keystore files are managed here; get FS root to resolve keys dir
		fsprov, ok := cpfsi.AsFSProvider(cpctx.Provider)
		if !ok {
			return nil
		}
		root := fsprov.FSRoot()
		keysDir := filepath.Join(root, "keys", m.TenantSlug)
		if err := os.MkdirAll(keysDir, 0o755); err != nil {
			return err
		}
		// Write active.json exactly as provided
		if strings.TrimSpace(dto.ActiveJSON) != "" {
			if err := os.WriteFile(filepath.Join(keysDir, "active.json"), []byte(dto.ActiveJSON), 0o600); err != nil {
				return err
			}
		}
		// Handle retiring
		retPath := filepath.Join(keysDir, "retiring.json")
		if strings.TrimSpace(dto.RetiringJSON) == "" {
			// remove if exists
			_ = os.Remove(retPath)
		} else {
			if err := os.WriteFile(retPath, []byte(dto.RetiringJSON), 0o600); err != nil {
				return err
			}
		}
		// Invalidate JWKS for tenant
		if cpctx.InvalidateJWKS != nil {
			cpctx.InvalidateJWKS(m.TenantSlug)
		}
		return nil
	default:
		// tipo desconocido: ignorar en MVP
		return nil
	}
}

// Snapshot: snapshot vacío.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	// We only support FS provider: package tenants/ and keys/ under FS root as tar.gz
	if cpctx.Provider == nil {
		return &fsSnap{root: ""}, nil
	}
	fsprov, ok := cpfsi.AsFSProvider(cpctx.Provider)
	if !ok {
		return &fsSnap{root: ""}, nil
	}
	root := fsprov.FSRoot()
	return &fsSnap{root: root}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	if rc == nil {
		return nil
	}
	defer rc.Close()
	if cpctx.Provider == nil {
		// consume stream to avoid breaking raft, but ignore
		_, _ = io.Copy(io.Discard, rc)
		return nil
	}
	fsprov, ok := cpfsi.AsFSProvider(cpctx.Provider)
	if !ok {
		_, _ = io.Copy(io.Discard, rc)
		return nil
	}
	root := fsprov.FSRoot()
	// staging dir under root/restore.tmp.<ts>
	staging := filepath.Join(root, "restore.tmp")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}

	// Extract tar.gz into staging
	gz, err := gzip.NewReader(rc)
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
		// Normalize header name to forward slashes to avoid Windows path issues
		n := strings.ReplaceAll(hdr.Name, "\\", "/")
		n = ppath.Clean(n)
		// Restrict to tenants/ and keys/ (allow exact dir names too)
		if !(n == "tenants" || n == "keys" || strings.HasPrefix(n, "tenants/") || strings.HasPrefix(n, "keys/")) {
			continue
		}
		// Convert back to OS-specific path when writing to disk
		target := filepath.Join(staging, filepath.FromSlash(n))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fs.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		default:
			// skip other types (symlinks etc.) for safety
		}
	}

	// Atomic-ish swap: move existing dirs aside, then replace
	tenantsDst := filepath.Join(root, "tenants")
	keysDst := filepath.Join(root, "keys")
	tenantsSt := filepath.Join(staging, "tenants")
	keysSt := filepath.Join(staging, "keys")

	// Ensure staging subdirs exist to avoid removing current data accidentally
	// If missing, create empty directories in staging
	_ = os.MkdirAll(tenantsSt, 0o755)
	_ = os.MkdirAll(keysSt, 0o755)

	// Replace tenants
	tmpTenants := tenantsDst + ".bak"
	_ = os.RemoveAll(tmpTenants)
	if _, err := os.Stat(tenantsDst); err == nil {
		_ = os.Rename(tenantsDst, tmpTenants)
	}
	if err := os.Rename(tenantsSt, tenantsDst); err != nil {
		// rollback best-effort
		_ = os.RemoveAll(tenantsSt)
		if _, stErr := os.Stat(tmpTenants); stErr == nil {
			_ = os.Rename(tmpTenants, tenantsDst)
		}
		return err
	}
	_ = os.RemoveAll(tmpTenants)

	// Replace keys
	tmpKeys := keysDst + ".bak"
	_ = os.RemoveAll(tmpKeys)
	if _, err := os.Stat(keysDst); err == nil {
		_ = os.Rename(keysDst, tmpKeys)
	}
	if err := os.Rename(keysSt, keysDst); err != nil {
		_ = os.RemoveAll(keysSt)
		if _, stErr := os.Stat(tmpKeys); stErr == nil {
			_ = os.Rename(tmpKeys, keysDst)
		}
		return err
	}
	_ = os.RemoveAll(tmpKeys)

	// Invalidate JWKS cache (global and all tenants)
	if cpctx.InvalidateJWKS != nil {
		cpctx.InvalidateJWKS("") // global
		// best-effort: enumerate tenants under new dir
		entries, _ := os.ReadDir(tenantsDst)
		for _, e := range entries {
			if e.IsDir() {
				cpctx.InvalidateJWKS(e.Name())
			}
		}
	}

	// Cleanup staging (now mostly empty after renames)
	_ = os.RemoveAll(staging)
	return nil
}

type fsSnap struct{ root string }

func (s *fsSnap) Persist(sink raft.SnapshotSink) error {
	// Create gzip writer over sink
	gw := gzip.NewWriter(sink)
	tw := tar.NewWriter(gw)
	// Helper to add a file
	addFile := func(rel string, info os.FileInfo, full string) error {
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(full)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	}
	// Walk tenants/ and keys/
	for _, sub := range []string{"tenants", "keys"} {
		base := filepath.Join(s.root, sub)
		// Skip if missing
		if st, err := os.Stat(base); err != nil || !st.IsDir() {
			continue
		}
		filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Rel name with Unix slashes in tar
			rel, _ := filepath.Rel(s.root, path)
			rel = filepath.ToSlash(rel)
			if info.IsDir() {
				// ensure explicit dir header for empty dirs
				if rel == "" {
					return nil
				}
				if err := addFile(rel+"/", info, path); err != nil {
					return err
				}
				return nil
			}
			return addFile(rel, info, path)
		})
	}
	if err := tw.Close(); err != nil {
		_ = gw.Close()
		_ = sink.Cancel()
		return err
	}
	if err := gw.Close(); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsSnap) Release() {}
