package http

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsOnce sync.Once
	metricsErr  error

	// HTTP metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpInflight        *prometheus.GaugeVec

	// Tenant migration metrics
	tenantMigrationsTotal   *prometheus.CounterVec
	tenantMigrationDuration *prometheus.HistogramVec
	corsRejectsTotal        *prometheus.CounterVec
)

// MetricsConfig agrupa dependencias necesarias para exponer /metrics y capturar datos.
type MetricsConfig struct {
	Registry      prometheus.Registerer
	TenantManager *tenantsql.Manager
	GlobalPool    func() *pgxpool.Pool
}

// RegisterMetrics inicializa las métricas HTTP y, opcionalmente, registra un collector
// para pools de base de datos (global + tenants). Devuelve el handler para /metrics.
func RegisterMetrics(cfg MetricsConfig) (http.Handler, error) {
	registry := cfg.Registry
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	metricsOnce.Do(func() {
		httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Número total de requests procesadas",
		}, []string{"method", "path", "status"})

		httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Latencia de los requests HTTP",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"})

		httpInflight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "Requests en vuelo por método y ruta",
		}, []string{"method", "path"})

		// Tenant migration metrics
		tenantMigrationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tenant_migrations_total",
			Help: "Total de migraciones de tenant por resultado",
		}, []string{"tenant", "result"}) // result: applied|skipped|failed

		tenantMigrationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tenant_migration_duration_seconds",
			Help:    "Duración de migraciones de tenant",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
		}, []string{"tenant"})

		corsRejectsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cors_rejects_total",
			Help: "CORS requests rechazadas por origin no permitido",
		}, []string{"origin", "client_id"})

		if err := registerCollector(registry, httpRequestsTotal); err != nil {
			metricsErr = err
			return
		}
		if err := registerCollector(registry, httpRequestDuration); err != nil {
			metricsErr = err
			return
		}
		if err := registerCollector(registry, httpInflight); err != nil {
			metricsErr = err
			return
		}
		if err := registerCollector(registry, tenantMigrationsTotal); err != nil {
			metricsErr = err
			return
		}
		if err := registerCollector(registry, tenantMigrationDuration); err != nil {
			metricsErr = err
			return
		}
		if err := registerCollector(registry, corsRejectsTotal); err != nil {
			metricsErr = err
			return
		}
	})
	if metricsErr != nil {
		return nil, metricsErr
	}

	if cfg.TenantManager != nil || cfg.GlobalPool != nil {
		collector := newDBPoolCollector(cfg.GlobalPool, cfg.TenantManager)
		if err := registerCollector(registry, collector); err != nil {
			return nil, err
		}
	}

	// Usamos el gatherer global por compatibilidad, ya que las métricas se registran allí.
	return promhttp.Handler(), nil
}

// WithMetrics instrumenta requests HTTP con métricas Prometheus (contadores, latencia, inflight).
func WithMetrics(next http.Handler) http.Handler {
	if next == nil {
		return nil
	}
	if httpRequestsTotal == nil || httpRequestDuration == nil || httpInflight == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.ToUpper(r.Method)
		pathLabel := normalizePath(r.URL.Path)

		httpInflight.WithLabelValues(method, pathLabel).Inc()
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w}
		defer func() {
			httpInflight.WithLabelValues(method, pathLabel).Dec()
			duration := time.Since(start).Seconds()
			httpRequestDuration.WithLabelValues(method, pathLabel).Observe(duration)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			httpRequestsTotal.WithLabelValues(method, pathLabel, strconv.Itoa(status)).Inc()
		}()

		next.ServeHTTP(rec, r)
	})
}

// registerCollector registra el collector en el registry indicado, ignorando duplicados.
func registerCollector(reg prometheus.Registerer, collector prometheus.Collector) error {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if err := reg.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return nil
		}
		return err
	}
	return nil
}

// dbPoolCollector expone gauges para los pools globales y por tenant.
type dbPoolCollector struct {
	tenantMgr  *tenantsql.Manager
	globalPool func() *pgxpool.Pool

	tenantCountDesc    *prometheus.Desc
	tenantAcquiredDesc *prometheus.Desc
	tenantIdleDesc     *prometheus.Desc
	tenantTotalDesc    *prometheus.Desc

	globalAcquiredDesc *prometheus.Desc
	globalIdleDesc     *prometheus.Desc
	globalTotalDesc    *prometheus.Desc
}

func newDBPoolCollector(global func() *pgxpool.Pool, mgr *tenantsql.Manager) *dbPoolCollector {
	return &dbPoolCollector{
		tenantMgr:          mgr,
		globalPool:         global,
		tenantCountDesc:    prometheus.NewDesc("tenant_pool_count", "Cantidad de pools de tenants activos", nil, nil),
		tenantAcquiredDesc: prometheus.NewDesc("tenant_pgxpool_acquired", "Conexiones adquiridas por tenant", []string{"tenant"}, nil),
		tenantIdleDesc:     prometheus.NewDesc("tenant_pgxpool_idle", "Conexiones inactivas por tenant", []string{"tenant"}, nil),
		tenantTotalDesc:    prometheus.NewDesc("tenant_pgxpool_total", "Conexiones totales configuradas por tenant", []string{"tenant"}, nil),
		globalAcquiredDesc: prometheus.NewDesc("pg_global_acquired", "Conexiones globales adquiridas", nil, nil),
		globalIdleDesc:     prometheus.NewDesc("pg_global_idle", "Conexiones globales inactivas", nil, nil),
		globalTotalDesc:    prometheus.NewDesc("pg_global_total", "Conexiones globales totales", nil, nil),
	}
}

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tenantCountDesc
	ch <- c.tenantAcquiredDesc
	ch <- c.tenantIdleDesc
	ch <- c.tenantTotalDesc
	ch <- c.globalAcquiredDesc
	ch <- c.globalIdleDesc
	ch <- c.globalTotalDesc
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	var tenantStats map[string]tenantsql.PoolStat
	if c.tenantMgr != nil {
		tenantStats = c.tenantMgr.Stats()
	}
	ch <- prometheus.MustNewConstMetric(c.tenantCountDesc, prometheus.GaugeValue, float64(len(tenantStats)))
	for slug, snapshot := range tenantStats {
		ch <- prometheus.MustNewConstMetric(c.tenantAcquiredDesc, prometheus.GaugeValue, float64(snapshot.Acquired), slug)
		ch <- prometheus.MustNewConstMetric(c.tenantIdleDesc, prometheus.GaugeValue, float64(snapshot.Idle), slug)
		ch <- prometheus.MustNewConstMetric(c.tenantTotalDesc, prometheus.GaugeValue, float64(snapshot.Total), slug)
	}

	if c.globalPool != nil {
		if pool := c.globalPool(); pool != nil {
			if stat := pool.Stat(); stat != nil {
				ch <- prometheus.MustNewConstMetric(c.globalAcquiredDesc, prometheus.GaugeValue, float64(stat.AcquiredConns()))
				ch <- prometheus.MustNewConstMetric(c.globalIdleDesc, prometheus.GaugeValue, float64(stat.IdleConns()))
				ch <- prometheus.MustNewConstMetric(c.globalTotalDesc, prometheus.GaugeValue, float64(stat.TotalConns()))
			}
		}
	}
}

var (
	uuidSegmentRE  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F-]{4}-[0-9a-fA-F-]{4,}$`)
	hexSegmentRE   = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	tokenSegmentRE = regexp.MustCompile(`^[A-Za-z0-9_-]{24,}$`)
)

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	clean := strings.SplitN(p, "?", 2)[0]
	if clean == "" {
		return "/"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}

	segments := strings.Split(clean, "/")
	var out []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if isDynamicSegment(seg) {
			out = append(out, ":param")
		} else {
			out = append(out, seg)
		}
	}
	if len(out) == 0 {
		return "/"
	}
	return "/" + strings.Join(out, "/")
}

func isDynamicSegment(seg string) bool {
	if len(seg) > 48 {
		return true
	}
	if uuidSegmentRE.MatchString(seg) {
		return true
	}
	if hexSegmentRE.MatchString(seg) {
		return true
	}
	if tokenSegmentRE.MatchString(seg) {
		return true
	}
	if _, err := strconv.Atoi(seg); err == nil {
		return true
	}
	return false
}

// RecordTenantMigration registra el resultado de una migración de tenant
func RecordTenantMigration(tenant, result string, duration time.Duration) {
	if tenantMigrationsTotal != nil {
		tenantMigrationsTotal.WithLabelValues(tenant, result).Inc()
	}
	if tenantMigrationDuration != nil {
		tenantMigrationDuration.WithLabelValues(tenant).Observe(duration.Seconds())
	}
}

// RecordCORSReject registra un rechazo de CORS
func RecordCORSReject(origin, clientID string) {
	if corsRejectsTotal != nil {
		corsRejectsTotal.WithLabelValues(origin, clientID).Inc()
	}
}
