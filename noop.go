package main

import "fmt"
import "net/http"
import "log"
import "strings"

var c int64 = 0

func main() {
	fmt.Println("a simple no-op http server is running on localhost:9000")
	fmt.Println("get /")
	fmt.Println("get /count")
	fmt.Println("get /mirror")

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/count", countHandler)
	http.HandleFunc("/mirror", mirrorHandler)

	log.Fatal(http.ListenAndServe(":9000", nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "nothing")
}
func countHandler(w http.ResponseWriter, r *http.Request) {
	c = c + 1
	fmt.Fprintf(w, "%d", c)
}

func mirrorHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "%s %s\n", r.Method, r.URL)

	for header, values := range r.Header {
		if !strings.Contains(strings.ToLower(header), "authorization") && !strings.Contains(strings.ToLower(header), "jwt") {
			value := strings.Join(values, ";")
			fmt.Fprintf(w, "%s: %s\n", header, value)
		}
	}

}
