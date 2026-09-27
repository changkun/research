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
	"regexp"
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
	Intro       template.HTML // the README before its first section
	Body        template.HTML // the sections
	Sections    []section     // one entry per section, for the page's menu
	CurrentYear string
	BuildTime   string
	BuildHash   string
}

type section struct{ ID, Title string }

var (
	// A link the README closes an entry with, such as [PDF](...), becomes a
	// labelled chip. The period that separates two of them in the README
	// goes, since the chips separate themselves.
	chipLink = regexp.MustCompile(`<a href="([^"]*)">(PDF|GitHub|YouTube|OSF|Website)</a>\.?`)

	// The flags an entry opens with name the languages it is in.
	leadingFlags = regexp.MustCompile(`<li>((?:(?:🇬🇧|🇨🇳|🇩🇪) ?)+)`)
	flagName     = map[string]string{"🇬🇧": "EN", "🇨🇳": "中文", "🇩🇪": "Deutsch"}

	sectionHeading = regexp.MustCompile(`<h2 id="([^"]+)">([^<]+)</h2>`)
)

// tagLanguages replaces the flags an entry opens with by a small tag
// naming its languages. English is the page's own language, so an entry
// only in English carries no tag.
func tagLanguages(html string) string {
	return leadingFlags.ReplaceAllStringFunc(html, func(m string) string {
		var names []string
		for _, f := range strings.Fields(strings.TrimPrefix(m, "<li>")) {
			names = append(names, flagName[f])
		}
		if len(names) == 1 && names[0] == "EN" {
			return "<li>"
		}
		return `<li><span class="lang">` + strings.Join(names, " · ") + `</span> `
	})
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

	out := chipLink.ReplaceAllString(content.String(), `<a class="chip" href="$1">$2</a>`)
	out = tagLanguages(out)

	intro, body := out, ""
	if i := strings.Index(out, "<h2"); i >= 0 {
		intro, body = out[:i], out[i:]
	}
	var sections []section
	for _, m := range sectionHeading.FindAllStringSubmatch(body, -1) {
		sections = append(sections, section{ID: m[1], Title: m[2]})
	}

	t, _ := time.Parse("2006-01-02", BuildTime)
	return tmpl.Execute(w, research{
		Intro:       template.HTML(intro),
		Body:        template.HTML(body),
		Sections:    sections,
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
