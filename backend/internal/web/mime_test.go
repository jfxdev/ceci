package web

import "testing"

func TestMimeFor(t *testing.T) {
	cases := map[string]string{
		"index.html":        "text/html; charset=utf-8",
		"assets/app.js":      "application/javascript; charset=utf-8",
		"assets/app.css":     "text/css; charset=utf-8",
		"logo.svg":            "image/svg+xml",
		"manifest.json":       "application/json; charset=utf-8",
		"favicon.ico":         "application/octet-stream",
	}
	for path, want := range cases {
		if got := mimeFor(path); got != want {
			t.Errorf("mimeFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestTrimLeadingSlash(t *testing.T) {
	if got := trimLeadingSlash("/foo"); got != "foo" {
		t.Errorf("got %q, want foo", got)
	}
	if got := trimLeadingSlash("foo"); got != "foo" {
		t.Errorf("got %q, want foo", got)
	}
	if got := trimLeadingSlash(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
