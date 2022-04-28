// Copyright 2020 Changkun Ou. All rights reserved.

package main

import (
	"context"
	"embed"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

//go:embed 20* cg1 gp
var static embed.FS

func main() {
	l := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile|log.Lmsgprefix)
	logger := logging(l)

	r := http.NewServeMux()
	r.Handle("/", http.StripPrefix("/talks/", http.FileServer(http.FS(static))))

	addr := os.Getenv("TALKS_ADDR")
	if len(addr) == 0 {
		addr = "0.0.0.0:9999"
	}
	s := &http.Server{
		Addr:         addr,
		Handler:      logger(r),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: time.Minute,
		IdleTimeout:  time.Minute,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		l.Println("talks is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		s.SetKeepAlivesEnabled(false)
		if err := s.Shutdown(ctx); err != nil {
			l.Fatalf("cannot gracefully shutdown talks: %v", err)
		}
		close(done)
	}()

	l.Printf("talks is serving on %s...", addr)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		l.Fatalf("cannot listen on %s, err: %v\n", addr, err)
	}

	l.Println("goodbye!")
	<-done
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				logger.Println(readIP(r), r.Method, r.URL.Path)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func readIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if clientIP != "" {
		return clientIP
	}
	if addr := r.Header.Get("X-Appengine-Remote-Addr"); addr != "" {
		return addr
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return "unknown" // use unknown to guarantee non empty string
	}
	return ip
}
