// Command stub is the endpoint the corpus simulation talks to while a recording is made.
//
// It is deliberately tiny and deterministic: /ok answers 200 at once, /fail answers 500 so a
// check fails, and /slow answers 200 after 1500 ms so the report's slowest response-time
// bucket is populated. Nothing here is part of the parsec module; it lives under testdata so
// a recording can be reproduced for a future Gatling version.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8089", "address to listen on")
	slow := flag.Duration("slow", 1500*time.Millisecond, "delay before /slow answers")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(*slow)
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("corpus stub listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
