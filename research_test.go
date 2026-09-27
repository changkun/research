// Copyright 2026 Changkun Ou. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLinkTalks(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "202603", "hial.pdf"))
	write(t, filepath.Join(dir, "201709", "lmuhci.pdf"))
	write(t, filepath.Join(dir, "top.pdf"))
	write(t, filepath.Join(dir, "202603", "notes.txt"))
	// A link a removed talk left behind.
	if err := os.Symlink("199901/gone.pdf", filepath.Join(dir, "gone.pdf")); err != nil {
		t.Fatal(err)
	}

	if err := linkTalks(dir); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{
		"hial.pdf":   filepath.Join("202603", "hial.pdf"),
		"lmuhci.pdf": filepath.Join("201709", "lmuhci.pdf"),
	} {
		got, err := os.Readlink(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s links to %q, want %q", name, got, want)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !strings.HasSuffix(string(b), want) {
			t.Errorf("%s does not resolve to its talk: %q, %v", name, b, err)
		}
	}
	if fi, err := os.Lstat(filepath.Join(dir, "top.pdf")); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("a talk kept at the top level was replaced: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "gone.pdf")); !os.IsNotExist(err) {
		t.Errorf("a stale link survived: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("a file that is not a PDF was linked: %v", err)
	}

	// A second run replaces its own links rather than failing on them.
	if err := linkTalks(dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

func TestLinkTalksRejectsSharedName(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "202010", "talk.pdf"))
	write(t, filepath.Join(dir, "202103", "talk.pdf"))

	err := linkTalks(dir)
	if err == nil || !strings.Contains(err.Error(), "talks/talk.pdf") {
		t.Fatalf("linkTalks = %v, want an error naming the shared address", err)
	}
}

func TestRenderIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "index.html"),
		[]byte(`<main>{{.Content}}</main><footer>{{.BuildTime}} {{.BuildHash}}</footer>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("# Research\n<!--begin-->\n## Papers\n\n- A paper [PDF](https://changkun.de/paper/a.pdf) [GitHub](https://github.com/changkun)\n<!--end-->\nnot rendered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	BuildTime, BuildHash = "2026-09-27", "abc1234"
	var b strings.Builder
	if err := renderIndex(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`<h2 id="papers">Papers</h2>`,
		`<a href="https://changkun.de/paper/a.pdf"><i class="fa-solid fa-file-pdf"></i></a>`,
		`<a href="https://github.com/changkun"><i class="fa-brands fa-github"></i></a>`,
		`<footer>Sep 27, 2026 abc1234</footer>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered page lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not rendered") {
		t.Errorf("rendered text outside the begin and end markers:\n%s", out)
	}
}
