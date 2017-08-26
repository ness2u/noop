package main

import "fmt"
import "net/http"
import "html"
import "log"

func main() {
	fmt.Println("a simple no-op http server is running on localhost:9000")
	fmt.Println("get /count")

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/count", countHandler)

	log.Fatal(http.ListenAndServe(":9000", nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "nothing")
}
func countHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
}
