package say

import (
	"path"
	"strings"
	"testing"
)

func TestContinuousTalkCataloguesHaveExplicitTranslationsAndPlaceholders(t *testing.T) {
	raw, _ := files.ReadFile("messages/en.yaml")
	english, e := read(raw)
	if e != nil {
		t.Fatal(e)
	}
	entries, e := files.ReadDir("messages")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		raw, e := files.ReadFile(path.Join("messages", entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		translated, e := read(raw)
		if e != nil {
			t.Fatal(e)
		}
		for key, text := range english {
			if !strings.HasPrefix(key, "clawtalk.") {
				continue
			}
			got := translated[key]
			if strings.TrimSpace(got) == "" {
				t.Errorf("%s missing %s", entry.Name(), key)
			}
			if strings.Count(text, "{{.N}}") != strings.Count(got, "{{.N}}") {
				t.Errorf("%s placeholder mismatch %s", entry.Name(), key)
			}
		}
	}
}
