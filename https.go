package main

import (
	"io"
	"net"
	"net/http"
	"time"
)

func HandleHttps(w http.ResponseWriter, r *http.Request) {
	dest_conn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(client_conn, dest_conn)
	go transfer(dest_conn, client_conn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func main() {
	server := &http.Server{
		Addr: ":6699",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HandleHttps(w, r)
		}),
	}
	server.ListenAndServe()
}
