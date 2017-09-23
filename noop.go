package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

var c int64 = 0

func main() {
	fmt.Println("a simple no-op http server is running on localhost:9000")
	fmt.Println("get /")
	fmt.Println("get /count")
	fmt.Println("get /mirror")
	fmt.Println("get /slow")

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/count", countHandler)
	http.HandleFunc("/mirror", mirrorHandler)
	http.HandleFunc("/slow", slowHandler)

	log.Fatal(http.ListenAndServe(":9000", nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w)
	fmt.Fprint(w, "nothing")
}

func countHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w)
	c = c + 1
	fmt.Fprintf(w, "%d", c)
}

func mirrorHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w)

	fmt.Printf("make peace with the mirror, and watch yourself change\n")
	fmt.Fprintf(w, "%s %s\n", r.Method, r.URL)

	for header, values := range r.Header {
		if !strings.Contains(strings.ToLower(header), "authorization") && !strings.Contains(strings.ToLower(header), "jwt") {
			value := strings.Join(values, ";")
			fmt.Fprintf(w, "%s: %s\n", header, value)
		}
	}
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second * 1)
	fmt.Fprintf(w, "a slow response")
}

func addHeaders(w http.ResponseWriter) {
	w.Header().Add("x-correlationId", fmt.Sprintf("cid_%v", time.Now().Unix()))
}
