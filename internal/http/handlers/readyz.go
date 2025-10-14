package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// HealthStatus representa el estado de un componente específico
type HealthStatus struct {
	Status  string `json:"status"` // "ok" | "error" | "disabled"
	Message string `json:"message,omitempty"`
}

// HealthResponse representa la respuesta de salud completa
type HealthResponse struct {
	Status      string                  `json:"status"` // "ready" | "degraded" | "unavailable"
	Components  map[string]HealthStatus `json:"components"`
	Version     string                  `json:"version,omitempty"`
	Commit      string                  `json:"commit,omitempty"`
	ActiveKeyID string                  `json:"active_key_id,omitempty"`
	Timestamp   time.Time               `json:"timestamp"`
	Cluster     map[string]any          `json:"cluster,omitempty"`
}

func NewReadyzHandler(c *app.Container, checkRedis func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		response := HealthResponse{
			Components: make(map[string]HealthStatus),
			Timestamp:  time.Now().UTC(),
		}

		// Metadata de servicio
		if v := os.Getenv("SERVICE_VERSION"); v != "" {
			response.Version = v
		}
		if git := os.Getenv("SERVICE_COMMIT"); git != "" {
			response.Commit = git
		}
		if c != nil && c.Issuer != nil && c.Issuer.Keys != nil {
			if kid, err := c.Issuer.ActiveKID(); err == nil && kid != "" {
				response.ActiveKeyID = kid
			}
		}

		// Headers para compatibilidad
		w.Header().Set("Content-Type", "application/json")
		if response.Version != "" {
			w.Header().Set("X-Service-Version", response.Version)
		}
		if response.Commit != "" {
			w.Header().Set("X-Service-Commit", response.Commit)
		}
		if response.ActiveKeyID != "" {
			w.Header().Set("X-JWKS-KID", response.ActiveKeyID)
		}

		hasErrors := false
		hasCriticalErrors := false

		// 1) Control-plane FS (crítico)
		if cpctx.Provider != nil {
			if _, err := cpctx.Provider.ListTenants(ctx); err != nil {
				response.Components["control_plane"] = HealthStatus{
					Status:  "error",
					Message: fmt.Sprintf("unavailable: %v", err),
				}
				hasCriticalErrors = true
				log.Printf(`{"level":"error","component":"control_plane","msg":"unavailable","err":"%v"}`, err)
			} else {
				response.Components["control_plane"] = HealthStatus{Status: "ok"}
			}
		} else {
			response.Components["control_plane"] = HealthStatus{
				Status:  "error",
				Message: "provider not initialized",
			}
			hasCriticalErrors = true
		}

		// 2) JWT Keystore (crítico)
		if c != nil && c.Issuer != nil {
			now := time.Now().UTC()
			claims := jwtv5.MapClaims{
				"iss": c.Issuer.Iss,
				"sub": "selfcheck",
				"aud": "health",
				"iat": now.Unix(),
				"nbf": now.Unix(),
				"exp": now.Add(60 * time.Second).Unix(),
			}
			signed, _, err := c.Issuer.SignRaw(claims)
			if err != nil {
				response.Components["keystore"] = HealthStatus{
					Status:  "error",
					Message: fmt.Sprintf("sign failed: %v", err),
				}
				hasCriticalErrors = true
			} else {
				parsed, err := jwtv5.Parse(signed, c.Issuer.Keyfunc(),
					jwtv5.WithValidMethods([]string{"EdDSA"}),
					jwtv5.WithIssuer(c.Issuer.Iss),
				)
				if err != nil || !parsed.Valid {
					response.Components["keystore"] = HealthStatus{
						Status:  "error",
						Message: fmt.Sprintf("verify failed: %v", err),
					}
					hasCriticalErrors = true
				} else {
					response.Components["keystore"] = HealthStatus{Status: "ok"}
				}
			}
		} else {
			response.Components["keystore"] = HealthStatus{
				Status:  "error",
				Message: "issuer not initialized",
			}
			hasCriticalErrors = true
		}

		// 3) Global DB (no crítico en FS-only)
		if c.Store != nil {
			if err := c.Store.Ping(ctx); err != nil {
				response.Components["db_global"] = HealthStatus{
					Status:  "error",
					Message: fmt.Sprintf("unavailable: %v", err),
				}
				hasErrors = true
				log.Printf(`{"level":"error","component":"db_global","msg":"unavailable","err":"%v"}`, err)
			} else {
				response.Components["db_global"] = HealthStatus{Status: "ok"}
			}
		} else {
			response.Components["db_global"] = HealthStatus{Status: "disabled", Message: "FS-only mode"}
		}

		// 4) Redis/Cache (no crítico)
		if checkRedis != nil {
			if err := checkRedis(ctx); err != nil {
				response.Components["redis"] = HealthStatus{
					Status:  "error",
					Message: fmt.Sprintf("unavailable: %v", err),
				}
				hasErrors = true
				log.Printf(`{"level":"error","component":"redis","msg":"unavailable","err":"%v"}`, err)
			} else {
				response.Components["redis"] = HealthStatus{Status: "ok"}
			}
		} else {
			response.Components["redis"] = HealthStatus{Status: "disabled", Message: "memory cache only"}
		}

		// 5) Tenant Pools (informativo con métricas)
		if c.TenantSQLManager != nil {
			stats := c.TenantSQLManager.Stats()
			var acquired, idle, total int
			for _, st := range stats {
				acquired += int(st.Acquired)
				idle += int(st.Idle)
				total += int(st.Total)
			}
			response.Components["tenant_pools"] = HealthStatus{
				Status:  "ok",
				Message: fmt.Sprintf("active pools: %d (acquired=%d idle=%d total=%d)", len(stats), acquired, idle, total),
			}
		} else {
			response.Components["tenant_pools"] = HealthStatus{Status: "disabled"}
		}

		// Cluster info (non-breaking enrichment)
		{
			// Determine mode based on container presence
			mode := "off"
			if c != nil && c.ClusterNode != nil {
				mode = "embedded"
			}

			clusterInfo := map[string]any{
				"mode": mode,
			}
			if mode == "embedded" && c != nil && c.ClusterNode != nil {
				role := "follower"
				if c.ClusterNode.IsLeader() {
					role = "leader"
				}
				st := c.ClusterNode.Stats()
				raftBlock := map[string]any{}
				if v, ok := st["applied_index"]; ok {
					raftBlock["applied_index"] = v
				}
				if v, ok := st["commit_index"]; ok {
					raftBlock["commit_index"] = v
				}
				if v, ok := st["last_log_index"]; ok {
					raftBlock["last_log_index"] = v
				}
				if v, ok := st["last_snapshot_index"]; ok {
					raftBlock["last_snapshot_index"] = v
				}
				if v, ok := st["num_peers"]; ok {
					raftBlock["num_peers"] = v
				}
				if v, ok := st["state"]; ok {
					raftBlock["state"] = v
				}
				if v, ok := st["last_contact"]; ok {
					raftBlock["last_contact"] = v
				}

				clusterInfo["role"] = role
				clusterInfo["leader_id"] = c.ClusterNode.LeaderID()
				// If stats exposes an id, include as node_id
				if v, ok := st["id"]; ok {
					clusterInfo["node_id"] = v
				}
				// Expected peers (static) if available
				if c.LeaderRedirects != nil && len(c.LeaderRedirects) > 0 {
					clusterInfo["leader_redirects"] = c.LeaderRedirects
				}
				// If Node exposes a peers map, surface its size for operator clarity
				if n := c.ClusterNode.KnownPeers(); n > 0 {
					clusterInfo["peers_configured"] = n
				}
				if v, ok := st["num_peers"]; ok {
					clusterInfo["peers_connected"] = v
				}
				if len(raftBlock) > 0 {
					clusterInfo["raft"] = raftBlock
				}
			}
			response.Cluster = clusterInfo
		}

		// Determinar estado general
		if hasCriticalErrors {
			response.Status = "unavailable"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if hasErrors {
			response.Status = "degraded"
			w.WriteHeader(http.StatusOK) // 200 pero degradado
		} else {
			response.Status = "ready"
			w.WriteHeader(http.StatusOK)
		}

		// Escribir respuesta JSON
		httpx.WriteJSON(w, http.StatusOK, response)
	}
}
