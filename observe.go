// observe.go — the signal noop emits about itself: one structured log line per
// request, and a /metrics page in Prometheus exposition. Standard library
// only, so the image stays a single static binary with no module fetch.
//
// Why this exists (2026-09-22, the ipsa-sre lane): noop is the demo's fault
// target — chaos in, a typed verdict out, a root cause found from evidence.
// Before this file noop's evidence was a few fmt.Printf lines and nothing a
// scraper could read, so a failure injected here was invisible everywhere
// except the client that injected it.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// version is stamped at build time: go build -ldflags "-X main.version=<stamp>".
var version = "dev"

var startedAt = time.Now()

// ── logging ──────────────────────────────────────────────────────────────────

// logger is JSON on stdout. LOG_LEVEL=debug|info|warn|error (default info).
var logger = newLogger()

func newLogger() *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

var cidSeq atomic.Uint64

// correlationID returns the caller's id (either header spelling) or mints one.
// A minted id is unique per process, which the old per-second stamp was not.
func correlationID(r *http.Request) string {
	if v := r.Header.Get("X-Correlation-Id"); v != "" {
		return v
	}
	if v := r.Header.Get("x-correlationId"); v != "" {
		return v
	}
	return fmt.Sprintf("cid_%d_%d", time.Now().Unix(), cidSeq.Add(1))
}

// ── metrics ──────────────────────────────────────────────────────────────────

// durationBuckets are the histogram's upper bounds in seconds. Wide on the
// right because /latency and /throughput are meant to be slow.
var durationBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

type reqKey struct {
	method, path string
	status       int
}

type histogram struct {
	counts []uint64 // one per bucket, cumulative at render time
	sum    float64
	count  uint64
}

// metrics is every series noop exposes. One mutex; the handlers are cheap
// and a request touches it twice.
type metricsStore struct {
	mu        sync.Mutex
	requests  map[reqKey]uint64
	bytes     map[reqKey]uint64
	durations map[string]*histogram // by path
	inflight  atomic.Int64
	panics    atomic.Uint64
	// chaos
	leakBytes      atomic.Uint64
	leakActive     atomic.Int64
	spinActive     atomic.Int64
	crashArmed     atomic.Int64
	latencyInduced atomic.Uint64 // milliseconds of /latency sleep served
}

var metrics = &metricsStore{
	requests:  map[reqKey]uint64{},
	bytes:     map[reqKey]uint64{},
	durations: map[string]*histogram{},
}

func (m *metricsStore) observe(method, path string, status int, n int64, d time.Duration) {
	k := reqKey{method, path, status}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[k]++
	if n > 0 {
		m.bytes[k] += uint64(n)
	}
	h := m.durations[path]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(durationBuckets))}
		m.durations[path] = h
	}
	s := d.Seconds()
	for i, ub := range durationBuckets {
		if s <= ub {
			h.counts[i]++
			break
		}
	}
	h.sum += s
	h.count++
}

// render writes the exposition. Series are sorted so two scrapes diff cleanly.
func (m *metricsStore) render(w *strings.Builder) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Fprintf(w, "# HELP noop_build_info Build information.\n# TYPE noop_build_info gauge\n")
	fmt.Fprintf(w, "noop_build_info{version=%q,go=%q} 1\n", version, runtime.Version())
	fmt.Fprintf(w, "# HELP noop_uptime_seconds Seconds since the process started.\n# TYPE noop_uptime_seconds gauge\n")
	fmt.Fprintf(w, "noop_uptime_seconds %.3f\n", time.Since(startedAt).Seconds())
	fmt.Fprintf(w, "# HELP noop_chaos_enabled 1 when ENABLE_CHAOS=true and the chaos endpoints are registered.\n# TYPE noop_chaos_enabled gauge\n")
	fmt.Fprintf(w, "noop_chaos_enabled %d\n", boolInt(chaosEnabled))

	fmt.Fprintf(w, "# HELP noop_http_requests_total Requests served, by method, path and status.\n# TYPE noop_http_requests_total counter\n")
	keys := make([]reqKey, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].status < keys[j].status
	})
	for _, k := range keys {
		fmt.Fprintf(w, "noop_http_requests_total{method=%q,path=%q,status=\"%d\"} %d\n", k.method, k.path, k.status, m.requests[k])
	}
	fmt.Fprintf(w, "# HELP noop_http_response_bytes_total Response body bytes written, by method, path and status.\n# TYPE noop_http_response_bytes_total counter\n")
	for _, k := range keys {
		if b, ok := m.bytes[k]; ok {
			fmt.Fprintf(w, "noop_http_response_bytes_total{method=%q,path=%q,status=\"%d\"} %d\n", k.method, k.path, k.status, b)
		}
	}
	fmt.Fprintf(w, "# HELP noop_http_request_duration_seconds Request duration, by path.\n# TYPE noop_http_request_duration_seconds histogram\n")
	paths := make([]string, 0, len(m.durations))
	for p := range m.durations {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		h := m.durations[p]
		var cum uint64
		for i, ub := range durationBuckets {
			cum += h.counts[i]
			fmt.Fprintf(w, "noop_http_request_duration_seconds_bucket{path=%q,le=%q} %d\n", p, strconv.FormatFloat(ub, 'g', -1, 64), cum)
		}
		fmt.Fprintf(w, "noop_http_request_duration_seconds_bucket{path=%q,le=\"+Inf\"} %d\n", p, h.count)
		fmt.Fprintf(w, "noop_http_request_duration_seconds_sum{path=%q} %.6f\n", p, h.sum)
		fmt.Fprintf(w, "noop_http_request_duration_seconds_count{path=%q} %d\n", p, h.count)
	}
	fmt.Fprintf(w, "# HELP noop_http_inflight_requests Requests currently being served.\n# TYPE noop_http_inflight_requests gauge\n")
	fmt.Fprintf(w, "noop_http_inflight_requests %d\n", m.inflight.Load())
	fmt.Fprintf(w, "# HELP noop_http_handler_panics_total Handler panics recovered by the server.\n# TYPE noop_http_handler_panics_total counter\n")
	fmt.Fprintf(w, "noop_http_handler_panics_total %d\n", m.panics.Load())
	fmt.Fprintf(w, "# HELP noop_counter_value The /count counter.\n# TYPE noop_counter_value gauge\n")
	fmt.Fprintf(w, "noop_counter_value %d\n", c.Load())

	// chaos: what has been injected, so a verdict can be read against it
	fmt.Fprintf(w, "# HELP noop_chaos_leak_bytes_total Bytes deliberately leaked by /memory-leak.\n# TYPE noop_chaos_leak_bytes_total counter\n")
	fmt.Fprintf(w, "noop_chaos_leak_bytes_total %d\n", m.leakBytes.Load())
	fmt.Fprintf(w, "# HELP noop_chaos_leaks_active Leak goroutines running.\n# TYPE noop_chaos_leaks_active gauge\n")
	fmt.Fprintf(w, "noop_chaos_leaks_active %d\n", m.leakActive.Load())
	fmt.Fprintf(w, "# HELP noop_chaos_cpu_spinners_active CPU-spin goroutines running.\n# TYPE noop_chaos_cpu_spinners_active gauge\n")
	fmt.Fprintf(w, "noop_chaos_cpu_spinners_active %d\n", m.spinActive.Load())
	fmt.Fprintf(w, "# HELP noop_chaos_crash_armed 1 while a /crash is counting down.\n# TYPE noop_chaos_crash_armed gauge\n")
	fmt.Fprintf(w, "noop_chaos_crash_armed %d\n", m.crashArmed.Load())
	fmt.Fprintf(w, "# HELP noop_chaos_latency_induced_ms_total Milliseconds of sleep served by /latency.\n# TYPE noop_chaos_latency_induced_ms_total counter\n")
	fmt.Fprintf(w, "noop_chaos_latency_induced_ms_total %d\n", m.latencyInduced.Load())

	// go runtime, the few that explain a chaos run
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "# HELP noop_go_goroutines Goroutines.\n# TYPE noop_go_goroutines gauge\n")
	fmt.Fprintf(w, "noop_go_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "# HELP noop_go_heap_alloc_bytes Heap bytes allocated and in use.\n# TYPE noop_go_heap_alloc_bytes gauge\n")
	fmt.Fprintf(w, "noop_go_heap_alloc_bytes %d\n", ms.HeapAlloc)
	fmt.Fprintf(w, "# HELP noop_go_sys_bytes Bytes obtained from the OS.\n# TYPE noop_go_sys_bytes gauge\n")
	fmt.Fprintf(w, "noop_go_sys_bytes %d\n", ms.Sys)
	fmt.Fprintf(w, "# HELP noop_go_gc_total Completed GC cycles.\n# TYPE noop_go_gc_total counter\n")
	fmt.Fprintf(w, "noop_go_gc_total %d\n", ms.NumGC)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	metrics.render(&b)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = fmt.Fprint(w, b.String())
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"version\":%q,\"go\":%q,\"chaos\":%t,\"uptime_seconds\":%.0f}\n", version, runtime.Version(), chaosEnabled, time.Since(startedAt).Seconds())
}

// ── the middleware ───────────────────────────────────────────────────────────

// recorder captures status and bytes and keeps the writer's Flusher and
// Hijacker (throughput streams; nothing hijacks today, but hiding the
// interface would silently break the first thing that does).
type recorder struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
}

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// observe wraps every handler: correlation id on the response, one JSON log
// line per request, the metrics above, and a recovered panic logged as a 500
// (net/http would otherwise drop the connection silently, which is exactly
// the kind of failure that leaves no evidence).
func observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cid := correlationID(r)
		w.Header().Set("x-correlation-id", cid)
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		metrics.inflight.Add(1)
		defer func() {
			metrics.inflight.Add(-1)
			if p := recover(); p != nil {
				metrics.panics.Add(1)
				if !rec.wrote {
					rec.WriteHeader(http.StatusInternalServerError)
					_, _ = rec.Write([]byte("handler panic\n"))
				}
				logger.Error("handler panic", "cid", cid, "method", r.Method, "path", r.URL.Path, "panic", fmt.Sprint(p))
			}
			d := time.Since(start)
			metrics.observe(r.Method, r.URL.Path, rec.status, rec.bytes, d)
			logger.Info("request",
				"cid", cid,
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", float64(d.Microseconds())/1000.0,
				"remote", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}
