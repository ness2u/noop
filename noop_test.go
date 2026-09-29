package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// serve stands the real routes up behind the real middleware on a test
// server, so a test exercises what a pod runs. Chaos on: the demo injects.
func serve(t *testing.T) *httptest.Server {
	t.Helper()
	chaosEnabled = true
	s := httptest.NewUnstartedServer(observe(newMux()))
	trackConnections(s.Config)
	s.Start()
	t.Cleanup(s.Close)
	return s
}

func get(t *testing.T, url string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func TestEveryEndpointAnswersWithItsStatusAndBody(t *testing.T) {
	s := serve(t)
	cases := []struct {
		path   string
		status int
		body   string // exact when non-empty
	}{
		{"/", 200, "nothing"},
		{"/liveness", 200, "nothing"},
		{"/healthcheck", 200, "nothing"},
		{"/healthz", 200, "OK"},
		{"/status?code=503", 503, "nothing"},
		{"/status?code=notanumber", 418, "nothing"},
		{"/latency?ms=20", 200, "nothing"}, // chaos is on, and still not here
		{"/version", 200, ""},
		{"/metrics", 200, ""},
	}
	for _, c := range cases {
		resp, body := get(t, s.URL+c.path, nil)
		if resp.StatusCode != c.status {
			t.Errorf("%s: status %d, want %d", c.path, resp.StatusCode, c.status)
		}
		if c.body != "" && body != c.body {
			t.Errorf("%s: body %q, want %q", c.path, body, c.body)
		}
	}
}

func TestCorrelationIdIsEchoedOrMintedOncePerRequest(t *testing.T) {
	s := serve(t)
	resp, _ := get(t, s.URL+"/", map[string]string{"X-Correlation-Id": "demo-42"})
	if got := resp.Header.Get("x-correlation-id"); got != "demo-42" {
		t.Fatalf("echo: got %q", got)
	}
	resp, _ = get(t, s.URL+"/", map[string]string{"x-correlationId": "old-spelling"})
	if got := resp.Header.Get("x-correlation-id"); got != "old-spelling" {
		t.Fatalf("old spelling: got %q", got)
	}
	a, _ := get(t, s.URL+"/", nil)
	b, _ := get(t, s.URL+"/", nil)
	ca, cb := a.Header.Get("x-correlation-id"), b.Header.Get("x-correlation-id")
	if ca == "" || cb == "" || ca == cb {
		t.Fatalf("minted ids must exist and differ: %q %q", ca, cb)
	}
}

func TestCountIsSafeUnderConcurrentRequests(t *testing.T) {
	s := serve(t)
	before := c.Load()
	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := get(t, s.URL+"/count", nil)
			if resp.StatusCode != 200 {
				t.Errorf("count: %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	if got := c.Load() - before; got != n {
		t.Fatalf("counter advanced by %d, want %d", got, n)
	}
}

func TestDownloadAndThroughputDeliverExactlyTheBytesAsked(t *testing.T) {
	s := serve(t)
	_, body := get(t, s.URL+"/download?size=5000", nil)
	if len(body) != 5000 {
		t.Fatalf("download: %d bytes", len(body))
	}
	start := time.Now()
	_, body = get(t, s.URL+"/throughput?bps=100000&size=20000", nil)
	if len(body) != 20000 {
		t.Fatalf("throughput: %d bytes", len(body))
	}
	// 20 000 B at 100 000 B/s with a 1 s token bucket: not instant, not minutes.
	if d := time.Since(start); d < 50*time.Millisecond || d > 5*time.Second {
		t.Fatalf("throughput pacing: took %v", d)
	}
}

func TestMirrorHidesCredentialHeaders(t *testing.T) {
	s := serve(t)
	_, body := get(t, s.URL+"/mirror", map[string]string{"Authorization": "Bearer secret", "X-Thing": "visible"})
	if strings.Contains(body, "secret") {
		t.Fatalf("mirror leaked the authorization header: %s", body)
	}
	if !strings.Contains(body, "X-Thing: visible") {
		t.Fatalf("mirror dropped an ordinary header: %s", body)
	}
}

func TestMetricsCarryRequestsByPathAndStatusAndTheChaosSeries(t *testing.T) {
	s := serve(t)
	get(t, s.URL+"/status?code=503", nil)
	get(t, s.URL+"/latency?ms=5", nil)
	_, body := get(t, s.URL+"/metrics", nil)
	want := []string{
		`noop_http_requests_total{method="GET",path="/status",status="503"} `,
		`noop_http_requests_total{method="GET",path="/latency",status="200"} `,
		`noop_http_request_duration_seconds_bucket{path="/latency",le="+Inf"} `,
		"noop_chaos_enabled 1",
		"noop_chaos_latency_induced_ms_total ",
		"noop_chaos_leaks_active 0",
		"noop_chaos_cpu_spinners_active ",
		"noop_chaos_crash_armed 0",
		"noop_build_info{",
		"noop_http_handler_panics_total ",
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("metrics missing %q", w)
		}
	}
	// Every non-comment line is `name{labels} value` or `name value`: the exposition parses.
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.LastIndexByte(line, ' '); i < 0 || i == len(line)-1 {
			t.Errorf("unparseable exposition line: %q", line)
		}
	}
}

func TestVersionIsJsonAndCarriesTheBuildStamp(t *testing.T) {
	s := serve(t)
	_, body := get(t, s.URL+"/version", nil)
	var v struct {
		Version string  `json:"version"`
		Go      string  `json:"go"`
		Uptime  float64 `json:"uptime_seconds"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("version is not json: %v (%s)", err, body)
	}
	if v.Version != version || v.Go == "" {
		t.Fatalf("version body: %+v", v)
	}
}

func TestAHandlerPanicIsALoggedFiveHundredNotADroppedConnection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	s := httptest.NewServer(observe(mux))
	defer s.Close()
	before := metrics.panics.Load()
	resp, body := get(t, s.URL+"/boom", nil)
	if resp.StatusCode != 500 || !strings.Contains(body, "handler panic") {
		t.Fatalf("panic: status %d body %q", resp.StatusCode, body)
	}
	if metrics.panics.Load() != before+1 {
		t.Fatalf("panic not counted")
	}
}

// The public listener is the one a Service or an Ingress can name, so the chaos
// routes are never on it, chaos on or off. Checked by the mux's own routing
// (no request is served, so no crash is ever armed in the test binary).
func TestChaosRoutesAreNeverOnThePublicListener(t *testing.T) {
	chaosEnabled = true
	t.Cleanup(func() { chaosEnabled = false })
	public := newMux()
	for _, p := range []string{"/latency", "/memory-leak", "/spin-cpu", "/crash"} {
		if _, pattern := public.Handler(httptest.NewRequest(http.MethodGet, p, nil)); pattern != "/" {
			t.Errorf("the public listener routes %s to %q with chaos on; only the catch-all may answer it", p, pattern)
		}
	}
}

// The private listener serves the injections. /latency is the one safe to
// exercise here; the rest are checked by routing, so nothing is armed.
func TestTheChaosListenerServesTheInjections(t *testing.T) {
	private := newChaosMux()
	for _, p := range []string{"/latency", "/memory-leak", "/spin-cpu", "/crash"} {
		if _, pattern := private.Handler(httptest.NewRequest(http.MethodGet, p, nil)); pattern != p {
			t.Errorf("the chaos listener routes %s to %q", p, pattern)
		}
	}
	s := httptest.NewServer(observe(private))
	defer s.Close()
	before := metrics.latencyInduced.Load()
	if _, body := get(t, s.URL+"/latency?ms=20", nil); body != "a slow response - 20 ms" {
		t.Fatalf("latency on the chaos listener: %q", body)
	}
	if metrics.latencyInduced.Load() != before+20 {
		t.Fatalf("latency not counted")
	}
}

// Off by default opens no second port; on, it binds loopback only, and
// CHAOS_PORT moves the port, never the address.
func TestTheChaosListenerIsLoopbackOnlyAndAbsentWhenOff(t *testing.T) {
	t.Cleanup(func() { chaosEnabled = false })
	chaosEnabled = false
	if a := chaosAddr(); a != "" {
		t.Fatalf("chaos off still opens %q", a)
	}
	chaosEnabled = true
	if a := chaosAddr(); a != "127.0.0.1:8081" {
		t.Fatalf("default chaos addr %q", a)
	}
	t.Setenv("CHAOS_PORT", "9099")
	if a := chaosAddr(); a != "127.0.0.1:9099" {
		t.Fatalf("CHAOS_PORT addr %q", a)
	}
}

// /version is served on the public listener: it says what build runs, not
// whether the process can be made to fail.
func TestVersionDoesNotAdvertiseChaos(t *testing.T) {
	s := serve(t)
	_, body := get(t, s.URL+"/version", nil)
	if strings.Contains(body, "chaos") {
		t.Errorf("/version names chaos on the public listener: %s", body)
	}
}

func TestChaosEndpointsAreNotRegisteredWhenChaosIsOff(t *testing.T) {
	// The root route is a catch-all, so an unregistered /latency answers the
	// root's "nothing" — never a slow response, never an injection.
	chaosEnabled = false
	s := httptest.NewServer(observe(newMux()))
	defer s.Close()
	before := metrics.latencyInduced.Load()
	resp, body := get(t, s.URL+"/latency?ms=50", nil)
	if resp.StatusCode != 200 || body != "nothing" {
		t.Fatalf("latency with chaos off: %d %q", resp.StatusCode, body)
	}
	if metrics.latencyInduced.Load() != before {
		t.Fatalf("latency was induced with chaos off")
	}
	_, body = get(t, s.URL+"/metrics", nil)
	if !strings.Contains(body, "noop_chaos_enabled 0") {
		t.Fatalf("chaos flag not reported off")
	}
}
