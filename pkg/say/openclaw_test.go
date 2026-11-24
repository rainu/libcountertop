package say

import (
	"path"
	"strings"
	"testing"
)

func TestOpenClawCataloguesAreComplete(t *testing.T) {
	raw, err := files.ReadFile("messages/en.yaml")
	if err != nil {
		t.Fatal(err)
	}
	english, err := read(raw)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for key := range english {
		if strings.HasPrefix(key, "openclaw.") {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		t.Fatal("OpenClaw messages are missing")
	}
	entries, err := files.ReadDir("messages")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		raw, err = files.ReadFile(path.Join("messages", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		translated, err := read(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s has no explicit translation for %s", entry.Name(), key)
			}
		}
	}
}

func TestOpenClawLanguageSelection(t *testing.T) {
	before := Chosen()
	t.Cleanup(func() { Use(before) })
	for _, tt := range []struct{ tag, save string }{{"en", "Save"}, {"de", "Speichern"}, {"it", "Salva"}, {"vi", "Lưu"}} {
		Use(tt.tag)
		if got := T("openclaw.action.save"); got != tt.save {
			t.Errorf("%s save: got %q, want %q", tt.tag, got, tt.save)
		}
	}
}
