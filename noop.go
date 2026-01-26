// dummy comment for tekton multiarch build test
package main

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"log"
	"math/big"
	"math/rand"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var c int64 = 0
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

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
	Mu        sync.Mutex
}

type ctxKey string

const connCtxKey ctxKey = "conn"

func main() {
	port := ":" + getenv("PORT", "8080")
	fmt.Println("a simple no-op http server is running on localhost" + port)

	// no-op
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/liveness", rootHandler)
	http.HandleFunc("/healthcheck", rootHandler)

	// progress!
	http.HandleFunc("/count", countHandler)
	http.HandleFunc("/counter", countHandler)

	// request/response
	http.HandleFunc("/mirror", mirrorHandler)
	http.HandleFunc("/status", statusHandler)

	// data
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/throughput", throughputHandler)

	// chaos
	if getenv("ENABLE_CHAOS", "false") == "true" {
		fmt.Println("CHAOS MODE ENABLED")
		http.HandleFunc("/latency", latencyHandler)
		http.HandleFunc("/memory-leak", leakHandler)
		http.HandleFunc("/spin-cpu", cpuHandler)
		http.HandleFunc("/crash", crashHandler)
	}

	server := &http.Server{
		Addr:    port,
		Handler: nil,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			// initialize per-connection state if not present
			if _, ok := connStates.Load(c); !ok {
				connStates.Store(c, &ConnThroughputState{TargetBps: defaultBps, Last: time.Now()})
			}
			return context.WithValue(ctx, connCtxKey, c)
		},
		ConnState: func(c net.Conn, state http.ConnState) {
			if state == http.StateClosed {
				connStates.Delete(c)
			}
		},
	}

	log.Fatal(server.ListenAndServe())
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

func countHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)
	c = c + 1
	fmt.Fprintf(w, "%d", c)
}

func mirrorHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)

	fmt.Printf("make peace with the mirror, and watch yourself change\n")
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

	addHeaders(w, r)
	time.Sleep(time.Millisecond * time.Duration(ms))
	fmt.Fprintf(w, "a slow response - %v ms", ms)
}

func addHeaders(w http.ResponseWriter, r *http.Request) {

	correlationId := r.Header.Get("x-correlationId")
	if len(correlationId) == 0 {
		correlationId = fmt.Sprintf("cid_%v", time.Now().Unix())
	}

	w.Header().Add("x-correlation-id", correlationId)
}

func leakHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	rate := readQueryInt(r, "rate", 1000)
	size := readQueryInt(r, "size", 1000000)

	addHeaders(w, r)
	fmt.Fprintf(w, "starting memory leak at %v bytes per %v ms", size, rate)

	leak := MemLeakStruct{time.Now().Unix(), []string{}}
	go leakMemory(leak, size, rate)
}

type MemLeakStruct struct {
	Timestamp int64
	Buffer    []string
}

func leakMemory(leak MemLeakStruct, size int, rate int) {
	newValue := randString(size)
	leak2 := MemLeakStruct{leak.Timestamp, append(leak.Buffer, newValue)}

	fmt.Printf("leaking %v bytes of memory\n", size)
	time.Sleep(time.Millisecond * time.Duration(rate))
	go leakMemory(leak2, size, rate)
}

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[mrand.Intn(len(letters))]
	}
	return string(b)
}

// downloadHandler streams random bytes of the requested size
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		panic(err)
	}
	addHeaders(w, r)
	size := readQueryInt(r, "size", 1024*1024)
	if size < 0 {
		size = 0
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(size))
	_, _ = io.CopyN(w, crand.Reader, int64(size))
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

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(remaining, 10))

	flusher, _ := w.(http.Flusher)

	const maxChunk = 32 * 1024 // 32KiB granularity

	for remaining > 0 {
		// compute available tokens
		s.Mu.Lock()
		now := time.Now()
		dt := now.Sub(s.Last).Seconds()
		if dt < 0 {
			dt = 0
		}
		s.Tokens += dt * float64(s.TargetBps)
		// Cap token bucket to 2 seconds of data to avoid unlimited bursts
		capTokens := float64(2 * s.TargetBps)
		if s.Tokens > capTokens {
			s.Tokens = capTokens
		}
		s.Last = now
		available := int64(s.Tokens)
		s.Mu.Unlock()

		if available <= 0 {
			// sleep briefly to accumulate tokens
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

		// send toSend bytes
		if _, err := io.CopyN(w, crand.Reader, toSend); err != nil {
			// client likely closed connection
			return
		}

		// decrement tokens
		s.Mu.Lock()
		s.Tokens -= float64(toSend)
		s.Total += toSend
		s.Mu.Unlock()

		remaining -= toSend
		if flusher != nil {
			flusher.Flush()
		}
	}
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
	fmt.Printf("Will spin the cpu with %d routines, for %d ms, in %d ms\n", count, time, delay)
	go spinCpu(delay, count, time)
}

func crashHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		panic(err)
	}

	delay := readQueryInt(r, "delay", 10000)

	fmt.Fprintf(w, "Will crash server in %d ms\n", delay)
	fmt.Printf("Will crash server in %d ms\n", delay)
	go func() {
		time.Sleep(time.Millisecond * time.Duration(delay))
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

	var counter = 0
	for i := 0; i < count; i++ {
		go func() {
			var a, b big.Int
			a.SetInt64(rand.Int63())
			for timeMs <= 0 || time.Since(startTime) < duration {
				b.SetInt64(rand.Int63())
				counter = counter + 1
				a.Mul(&a, &b)
			}
			fmt.Printf("done wasting cpu, counter: %d\n", counter)
		}()
	}
}
