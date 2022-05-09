// Copyright 2020 Changkun Ou. All rights reserved.

package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	//go:embed talks papers teach theses
	static embed.FS
)

func main() {
	l := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile|log.Lmsgprefix)
	logger := logging(l)

	r := http.NewServeMux()
	r.Handle("/", http.StripPrefix("/research/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "talks") {
			found := false
			fs.WalkDir(static, "talks", func(path string, d fs.DirEntry, err error) error {
				if d.IsDir() || !strings.HasPrefix(path, "talks") || !strings.HasSuffix(path, ".pdf") {
					return nil
				}

				names := strings.Split(path, "/")
				if len(names) > 2 {
					try := fmt.Sprintf("%s/%s", names[0], names[len(names)-1])
					if strings.Compare(r.URL.Path, try) == 0 {
						b, _ := fs.ReadFile(static, path)
						io.Copy(w, bytes.NewReader(b))
						found = true
					}
				}
				return nil
			})
			if found {
				return
			}
		}

		http.FileServer(http.FS(static)).ServeHTTP(w, r)
	})))

	addr := os.Getenv("RESEARCH_ADDR")
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
		l.Println("research is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		s.SetKeepAlivesEnabled(false)
		if err := s.Shutdown(ctx); err != nil {
			l.Fatalf("cannot gracefully shutdown research: %v", err)
		}
		close(done)
	}()

	l.Printf("research is serving on %s...", addr)
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
