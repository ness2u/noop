package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var c int64 = 0

func main() {
	fmt.Println("a simple no-op http server is running on localhost:8080")

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/count", countHandler)
	http.HandleFunc("/counter", countHandler)
	http.HandleFunc("/mirror", mirrorHandler)
	http.HandleFunc("/slow", slowHandler)
	http.HandleFunc("/status", statusHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
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

	var status int
	if statusValues := r.Form["code"]; len(statusValues) > 0 {
		statusValue := statusValues[0]
		if status, err = strconv.Atoi(statusValue); err != nil {
			status = http.StatusBadRequest
		}
	} else {
		status = 418 // i'm a teapot
	}

	addHeaders(w, r)
	w.WriteHeader(status)
	fmt.Fprint(w, "nothing")
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	addHeaders(w, r)
	time.Sleep(time.Second * 1)
	fmt.Fprintf(w, "a slow response")
}

func addHeaders(w http.ResponseWriter, r *http.Request) {

	correlationId := r.Header.Get("x-correlationId")
	if len(correlationId) == 0 {
		correlationId = fmt.Sprintf("cid_%v", time.Now().Unix())
	}

	w.Header().Add("x-correlationId", correlationId)
}
