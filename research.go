// Copyright 2022 Changkun Ou. All rights reserved.

package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var (
	BuildTime string
	BuildHash string
	md        goldmark.Markdown = goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(),
	)
)

func convertMD(filename string) (bytes.Buffer, error) {
	d, err := os.ReadFile(filename)
	if err != nil {
		return bytes.Buffer{}, fmt.Errorf("cannot read README.md, err: %w", err)
	}
	d = []byte(strings.Split(strings.Split(string(d), "<!--begin-->")[1], "<!--end-->")[0])

	var b bytes.Buffer
	err = md.Convert(d, &b)
	if err != nil {
		log.Fatalf("Convert: cannot convert README from markdown to html, err: %v", err)
	}
	return b, nil
}

type research struct {
	// Navigation template.HTML
	Content     template.HTML
	CurrentYear string
	BuildTime   string
	BuildHash   string
}

func renderIndex(w http.ResponseWriter) {
	b, _ := os.ReadFile("assets/index.html")
	tmpl := template.Must(template.New("main").Parse(string(b)))

	content, err := convertMD("README.md")
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}

	iconPDF := `<i class="fa-solid fa-file-pdf"></i>`
	out := strings.Replace(content.String(), ">PDF</a>", ">"+iconPDF+"</a>", -1)

	iconGitHub := `<i class="fa-brands fa-github"></i>`
	out = strings.Replace(out, ">GitHub</a>", ">"+iconGitHub+"</a>", -1)

	iconYouTube := `<i class="fa-brands fa-youtube"></i>`
	out = strings.Replace(out, ">YouTube</a>", ">"+iconYouTube+"</a>", -1)

	iconOSF := `<i class="ai ai-osf"></i>`
	out = strings.Replace(out, ">OSF</a>", ">"+iconOSF+"</a>", -1)

	t, _ := time.Parse("2006-01-02", BuildTime)
	tmpl.Execute(w, research{
		Content:     template.HTML(out),
		CurrentYear: time.Now().Format("2006"),
		BuildTime:   t.Format("Jan 02, 2006"),
		BuildHash:   BuildHash,
	})
}

func reportUrlstat(path, ua string) {
	// Report stats to urlstat. Similar to this javascript code in
	// https://github.com/changkun/urlstat/blob/main/public/client.js
	//
	// 	let endpoint = 'https://www.changkun.de/urlstat'
	// 	const h = new Headers({'urlstat-url': window.location.href,'urlstat-ua': navigator.userAgent})
	// 	const r = new Request(endpoint, {method: 'GET', headers: h})
	req, err := http.NewRequest("GET", "https://www.changkun.de/urlstat?report=page+site", nil)
	if err != nil {
		log.Println("failed to create request: ", err)
		return
	}
	req.Header.Add("urlstat-url", "https://changkun.de/research/"+path)
	req.Header.Add("urlstat-ua", ua)
	log.Println("sending request to urlstat: ", req.URL.String(), req.Header)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("failed to send request: ", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("failed to send request: ", resp.Status)
		return
	}
	body := new(bytes.Buffer)
	body.ReadFrom(resp.Body)

	log.Printf("status: %s, urlstat: %s\n", resp.Status, body.String())
}

func main() {
	l := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile|log.Lmsgprefix)
	logger := logging(l)

	r := http.NewServeMux()
	r.Handle("/", http.StripPrefix("/research", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("accessing: ", r.URL.Path)
		// route / and /index.html
		if r.URL.Path == "" {
			http.Redirect(w, r, "/research/", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			renderIndex(w)
			return
		}
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/")
		go reportUrlstat(r.URL.Path, r.UserAgent())

		// If this path lead to a folder, serve the entire folder
		if d, err := os.Stat(r.URL.Path); err == nil && d.IsDir() {
			http.FileServer(http.Dir(".")).ServeHTTP(w, r)
			return
		}

		// route /talks/**/*.pdf to /talks/*.pdf
		if strings.HasPrefix(r.URL.Path, "talks") {
			found := false
			filepath.WalkDir("talks", func(path string, d fs.DirEntry, err error) error {
				if d.IsDir() || !strings.HasPrefix(path, "talks") || !strings.HasSuffix(path, ".pdf") {
					return nil
				}

				names := strings.Split(path, "/")
				if len(names) > 2 {
					try := fmt.Sprintf("%s/%s", names[0], names[len(names)-1])
					if r.URL.Path == try {
						b, _ := os.ReadFile(path)
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

		log.Println("access: ", r.URL.Path)

		// route /papers/* /teach/* /theses/*
		if strings.HasPrefix(r.URL.Path, "papers") ||
			strings.HasPrefix(r.URL.Path, "teach") ||
			strings.HasPrefix(r.URL.Path, "theses") ||
			strings.HasPrefix(r.URL.Path, "assets") {

			b, err := os.ReadFile(r.URL.Path)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(fmt.Sprintf("failed to find file: %v", r.URL.Path)))
				return
			}

			ext := filepath.Ext(r.URL.Path)
			switch ext {
			case ".css":
				w.Header().Add("Content-Type", "text/css")
			case ".js":
				w.Header().Add("Content-Type", "text/javascript")
			}
			io.Copy(w, bytes.NewReader(b))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("unsupported access!"))
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
