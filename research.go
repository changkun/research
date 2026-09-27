// Copyright 2022 Changkun Ou. All rights reserved.

// Command research renders changkun.de/research into this working tree,
// which a static file server then serves as a plain folder.
//
// It writes index.html from the README and the page template, and links
// each talk at talks/<name>.pdf, the address changkun.de/talk/<name>.pdf
// redirects to, next to the dated folder that holds it.
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
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
		return bytes.Buffer{}, fmt.Errorf("cannot convert README from markdown to html, err: %w", err)
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

func renderIndex(w io.Writer) error {
	b, err := os.ReadFile("assets/index.html")
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("main").Parse(string(b)))

	content, err := convertMD("README.md")
	if err != nil {
		return err
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
	return tmpl.Execute(w, research{
		Content:     template.HTML(out),
		CurrentYear: time.Now().Format("2006"),
		BuildTime:   t.Format("Jan 02, 2006"),
		BuildHash:   BuildHash,
	})
}

// linkTalks links every PDF below a subfolder of dir at dir/<name>.pdf,
// with a relative link, so it resolves wherever the tree is served from.
// Links left from an earlier run are replaced, and a name two talks share
// is an error, since one address cannot serve both.
func linkTalks(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}

	seen := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".pdf") || !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		name := d.Name()
		if other, ok := seen[name]; ok {
			return fmt.Errorf("talks %s and %s share the address talks/%s", other, rel, name)
		}
		seen[name] = rel
		return nil
	})
	if err != nil {
		return err
	}

	for name, rel := range seen {
		link := filepath.Join(dir, name)
		if _, err := os.Lstat(link); err == nil {
			continue // a talk kept at the top level serves itself
		}
		if err := os.Symlink(rel, link); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("research: ")

	var b bytes.Buffer
	if err := renderIndex(&b); err != nil {
		log.Fatalf("cannot render index.html: %v", err)
	}
	if err := os.WriteFile("index.html", b.Bytes(), 0o644); err != nil {
		log.Fatalf("cannot write index.html: %v", err)
	}
	if err := linkTalks("talks"); err != nil {
		log.Fatalf("cannot link talks: %v", err)
	}
}
