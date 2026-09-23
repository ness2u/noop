// dummy comment for tekton multiarch build test
package main

import (
	"context"
	"fmt"
	"math/big"
	rand "math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// c is the /count counter. Atomic: it was a plain int64 written from every
// request goroutine, which the race detector flags and a load test corrupts.
var c atomic.Int64

// chaosEnabled is ENABLE_CHAOS=true: the chaos endpoints are registered and
// noop_chaos_enabled reads 1.
var chaosEnabled bool
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// pattern for ASCII output
var asciiPattern = []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz\n")

func writePatternN(w http.ResponseWriter, n int64) error {
	if n <= 0 {
		return nil
	}
	remaining := n
	off := 0
	for remaining > 0 {
		chunk := int64(len(asciiPattern) - off)
		if chunk > remaining {
			chunk = remaining
		}
		if _, err := w.Write(asciiPattern[off : off+int(chunk)]); err != nil {
			return err
		}
		remaining -= chunk
		off += int(chunk)
		if off >= len(asciiPattern) {
			off = 0
		}
	}
	return nil
}

// per-connection throughput state
var (
	connStates sync.Map             // key: net.Conn, value: *ConnThroughputState
	defaultBps int64    = 1_000_000 // 1 Mbps by default
)

type ConnThroughputState struct {
	TargetBps int64
	Last      time.Time
	Tokens    float64
	Total     int64

	// rolling history of recent request payloads
	hist       []reqSample
	maxSamples int
	windowMax  time.Duration

	Mu sync.Mutex
}

type reqSample struct {
	T time.Time
	N int64
}

func (s *ConnThroughputState) pruneHistory(now time.Time) {
	cut := now.Add(-s.windowMax)
	// drop old entries at front
	i := 0
	for i < len(s.hist) && s.hist[i].T.Before(cut) {
		i++
	}
	if i > 0 {
		s.hist = append([]reqSample(nil), s.hist[i:]...)
	}
	if len(s.hist) > s.maxSamples {
		s.hist = append([]reqSample(nil), s.hist[len(s.hist)-s.maxSamples:]...)
	}
}

func (s *ConnThroughputState) addSample(n int64) {
	now := time.Now()
	s.pruneHistory(now)
	s.hist = append(s.hist, reqSample{T: now, N: n})
}

func (s *ConnThroughputState) effectiveBps() float64 {
	now := time.Now()
	s.pruneHistory(now)
	if len(s.hist) == 0 {
		return 0
	}
	var total int64
	oldest := s.hist[0].T
	for _, h := range s.hist {
		total += h.N
		if h.T.Before(oldest) {
			oldest = h.T
		}
	}
	dur := now.Sub(oldest).Seconds()
	if dur <= 0 {
		dur = 0.001
	}
	return float64(total) / dur
}

func (s *ConnThroughputState) bucketFill(now time.Time) {
	dt := now.Sub(s.Last).Seconds()
	if dt < 0 {
		dt = 0
	}
	s.Tokens += dt * float64(s.TargetBps)
	// cap to 1 second worth to smooth bursts
	capTokens := float64(s.TargetBps) * 1.0
	if s.Tokens > capTokens {
		s.Tokens = capTokens
	}
	s.Last = now
}

type ctxKey string

const connCtxKey ctxKey = "conn"

// newMux registers every endpoint. Separate from main so a test can serve
// the same routes on an httptest server.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	// no-op
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/liveness", rootHandler)
	mux.HandleFunc("/healthcheck", rootHandler)
	mux.HandleFunc("/healthz", healthzHandler)

	// about itself
	mux.HandleFunc("/metrics", metricsHandler)
	mux.HandleFunc("/version", versionHandler)

	// progress!
	mux.HandleFunc("/count", countHandler)
	mux.HandleFunc("/counter", countHandler)

	// request/response
	mux.HandleFunc("/mirror", mirrorHandler)
	mux.HandleFunc("/status", statusHandler)

	// data
	mux.HandleFunc("/download", downloadHandler)
	mux.HandleFunc("/throughput", throughputHandler)

	// chaos
	if chaosEnabled {
		mux.HandleFunc("/latency", latencyHandler)
		mux.HandleFunc("/memory-leak", leakHandler)
		mux.HandleFunc("/spin-cpu", cpuHandler)
		mux.HandleFunc("/crash", crashHandler)
	}
	return mux
}

func main() {
	port := ":" + getenv("PORT", "8080")
	chaosEnabled = getenv("ENABLE_CHAOS", "false") == "true"
	logger.Info("noop starting", "addr", port, "version", version, "chaos", chaosEnabled, "log_level", getenv("LOG_LEVEL", "info"))

	server := &http.Server{
		Addr:    port,
		Handler: observe(newMux()),
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			// initialize per-connection state if not present
			if _, ok := connStates.Load(c); !ok {
				connStates.Store(c, &ConnThroughputState{
					TargetBps:  defaultBps,
					Last:       time.Now(),
					Tokens:     0,
					Total:      0,
					hist:       make([]reqSample, 0, 10),
					maxSamples: 10,
					windowMax:  10 * time.Second,
				})
			}
			return context.WithValue(ctx, connCtxKey, c)
		},
		ConnState: func(c net.Conn, state http.ConnState) {
			if state == http.StateClosed {
				connStates.Delete(c)
			}
		},
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error("noop stopped", "err", err.Error())
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)
	fmt.Fprint(w, "nothing")
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func countHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)
	fmt.Fprintf(w, "%d", c.Add(1))
}

func mirrorHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)

	logger.Debug("make peace with the mirror, and watch yourself change")
	fmt.Fprintf(w, "%s %s\n", r.Method, r.URL)

	for header, values := range r.Header {
		if !strings.Contains(strings.ToLower(header), "authorization") && !strings.Contains(strings.ToLower(header), "jwt") {
			value := strings.Join(values, ";")
			fmt.Fprintf(w, "%s: %s\n", header, value)
		}
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {

	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	status := readQueryInt(r, "code", 418)

	addHeaders(w, r)
	w.WriteHeader(status)
	fmt.Fprint(w, "nothing")
}

func latencyHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	ms := readQueryInt(r, "ms", 1000)
	if ms < 0 {
		ms = 0
	}

	addHeaders(w, r)
	logger.Info("chaos: latency", "ms", ms)
	time.Sleep(time.Millisecond * time.Duration(ms))
	metrics.latencyInduced.Add(uint64(ms))
	fmt.Fprintf(w, "a slow response - %v ms", ms)
}

// addHeaders is kept for the handlers that call it; the correlation id is
// now set once for every request by the observe middleware, so this only
// guarantees the header exists for a handler served without it.
func addHeaders(w http.ResponseWriter, r *http.Request) {
	if w.Header().Get("x-correlation-id") == "" {
		w.Header().Set("x-correlation-id", correlationID(r))
	}
}

func leakHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	rate := readQueryInt(r, "rate", 1000)
	size := readQueryInt(r, "size", 1000000)

	addHeaders(w, r)
	logger.Warn("chaos: memory leak started", "bytes_per_leak", size, "ms_between_leaks", rate)
	fmt.Fprintf(w, "starting memory leak at %v bytes per %v ms", size, rate)

	leak := MemLeakStruct{time.Now().Unix(), []string{}}
	metrics.leakActive.Add(1)
	go leakMemory(leak, size, rate)
}

type MemLeakStruct struct {
	Timestamp int64
	Buffer    []string
}

func leakMemory(leak MemLeakStruct, size int, rate int) {
	newValue := randString(size)
	leak2 := MemLeakStruct{leak.Timestamp, append(leak.Buffer, newValue)}

	metrics.leakBytes.Add(uint64(size))
	logger.Debug("chaos: leaking", "bytes", size, "held", len(leak2.Buffer))
	time.Sleep(time.Millisecond * time.Duration(rate))
	go leakMemory(leak2, size, rate)
}

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// downloadHandler streams text bytes of the requested size
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic(err)
	}
	addHeaders(w, r)
	size := readQueryInt(r, "size", 1024*1024)
	if size < 0 {
		size = 0
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(size))
	_ = writePatternN(w, int64(size))
}

func getConnFromCtx(ctx context.Context) (net.Conn, bool) {
	c, ok := ctx.Value(connCtxKey).(net.Conn)
	return c, ok
}

func getConnState(c net.Conn) *ConnThroughputState {
	if v, ok := connStates.Load(c); ok {
		if s, ok2 := v.(*ConnThroughputState); ok2 {
			return s
		}
	}
	s := &ConnThroughputState{TargetBps: defaultBps, Last: time.Now()}
	connStates.Store(c, s)
	return s
}

// throughputHandler streams bytes paced to a per-connection target Bps
// Query:
// - bps: desired bytes-per-second (optional; updates connection state)
// - size: bytes to send this request (default 1MiB)
func throughputHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic(err)
	}
	addHeaders(w, r)

	conn, ok := getConnFromCtx(r.Context())
	if !ok {
		// Fallback: treat as stateless if conn not available
		downloadHandler(w, r)
		return
	}

	s := getConnState(conn)

	// Optional update of target bps
	if vals, ok := r.Form["bps"]; ok && len(vals) > 0 {
		if b, err := strconv.ParseInt(vals[0], 10, 64); err == nil && b > 0 {
			s.Mu.Lock()
			s.TargetBps = b
			s.Mu.Unlock()
		}
	}

	size := readQueryInt(r, "size", 1024*1024)
	if size < 0 {
		size = 0
	}
	remaining := int64(size)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(remaining, 10))

	flusher, _ := w.(http.Flusher)

	const maxChunk = 32 * 1024 // 32KiB granularity

	for remaining > 0 {
		s.Mu.Lock()
		s.bucketFill(time.Now())
		available := int64(s.Tokens)
		s.Mu.Unlock()

		if available <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		toSend := available
		if toSend > remaining {
			toSend = remaining
		}
		if toSend > maxChunk {
			toSend = maxChunk
		}

		if err := writePatternN(w, toSend); err != nil {
			return
		}

		s.Mu.Lock()
		s.Tokens -= float64(toSend)
		s.Total += toSend
		s.Mu.Unlock()

		remaining -= toSend
		if flusher != nil {
			flusher.Flush()
		}
	}

	// record sample for this request
	s.Mu.Lock()
	s.addSample(int64(size))
	s.Mu.Unlock()
}

func cpuHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	count := readQueryInt(r, "count", 1)
	delay := readQueryInt(r, "delay", 1000)
	time := readQueryInt(r, "time", 10000)

	fmt.Fprintf(w, "Will spin the cpu with %d routines, for %d ms, in %d ms\n", count, time, delay)
	logger.Warn("chaos: cpu spin scheduled", "routines", count, "for_ms", time, "in_ms", delay)
	go spinCpu(delay, count, time)
}

func crashHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	delay := readQueryInt(r, "delay", 10000)

	fmt.Fprintf(w, "Will crash server in %d ms\n", delay)
	logger.Warn("chaos: crash armed", "in_ms", delay)
	metrics.crashArmed.Store(1)
	go func() {
		time.Sleep(time.Millisecond * time.Duration(delay))
		logger.Error("chaos: crashing now", "armed_ms", delay)
		panic("This server has been intentionally crashed!")
	}()
}

func readQueryInt(r *http.Request, queryArg string, fallback int) int {
	var err error
	var result int
	if queryValues := r.Form[queryArg]; len(queryValues) > 0 {
		queryValue := queryValues[0]
		if result, err = strconv.Atoi(queryValue); err != nil {
			result = fallback
		}
	} else {
		result = fallback
	}
	return result
}

func spinCpu(delayMs int, count int, timeMs int) {
	time.Sleep(time.Millisecond * time.Duration(delayMs))
	startTime := time.Now()
	duration := time.Duration(timeMs) * time.Millisecond

	var counter atomic.Int64
	for i := 0; i < count; i++ {
		metrics.spinActive.Add(1)
		go func() {
			defer metrics.spinActive.Add(-1)
			var a, b big.Int
			a.SetInt64(rand.Int63())
			for timeMs <= 0 || time.Since(startTime) < duration {
				b.SetInt64(rand.Int63())
				counter.Add(1)
				a.Mul(&a, &b)
			}
			logger.Info("chaos: cpu spinner done", "counter", counter.Load())
		}()
	}
}
