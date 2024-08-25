package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var c int64 = 0
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

	// chaos
	http.HandleFunc("/latency", latencyHandler)
	http.HandleFunc("/memory-leak", leakHandler)
	http.HandleFunc("/spin-cpu", cpuHandler)
	http.HandleFunc("/crash", crashHandler)

	log.Fatal(http.ListenAndServe(port, nil))
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
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
