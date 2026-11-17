package say

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestEnglishIsThere(t *testing.T) {
	ids, err := Ids()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 50 {
		t.Fatalf("%d identifiers, which is fewer than the panel shows", len(ids))
	}

	for _, id := range ids {
		if got := T(id); got == id {
			t.Errorf("%s reads as its own identifier", id)
		}
	}
}

func TestIdentifiersAreDotted(t *testing.T) {
	ids, err := Ids()
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		if !strings.Contains(id, ".") {
			t.Errorf("%q has no screen in front of it", id)
		}
		if strings.ToLower(id) != id {
			t.Errorf("%q is not lower case", id)
		}
	}
}

func TestAMissingIdentifierIsItsOwnName(t *testing.T) {
	if got := T("nothing.defines.this"); got != "nothing.defines.this" {
		t.Errorf("a missing identifier came back as %q", got)
	}
	if !Missing("nothing.defines.this") {
		t.Error("a missing identifier does not report as missing")
	}
	if Missing("settings.title") {
		t.Error("a defined identifier reports as missing")
	}
}

func TestAPartialTranslationFallsBackPerString(t *testing.T) {
	if err := Load("pl", "settings.title: Ustawienia\n"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Use(English.String()) })

	if got := Use("pl"); got != "pl" {
		t.Fatalf("asking for pl settled on %q", got)
	}

	if got := T("settings.title"); got != "Ustawienia" {
		t.Errorf("the translated string is %q", got)
	}
	if got := T("settings.display"); got != "Display" {
		t.Errorf("an untranslated string is %q, want the English", got)
	}
}

func TestAnUnknownLanguageIsEnglish(t *testing.T) {
	t.Cleanup(func() { Use(English.String()) })

	for _, want := range []string{"zz", "klingon", "", "!!"} {
		if got := Use(want); got != "en" {
			t.Errorf("asking for %q settled on %q, want en", want, got)
		}
		if got := T("settings.title"); got != "Settings" {
			t.Errorf("after asking for %q the title is %q", want, got)
		}
	}
}

func TestUntranslatedReportsWhatIsLeft(t *testing.T) {
	if err := Load("fi", "settings.title: Asetukset\n"); err != nil {
		t.Fatal(err)
	}

	left, err := Untranslated("fi")
	if err != nil {
		t.Fatal(err)
	}

	ids, _ := Ids()
	if len(left) != len(ids)-1 {
		t.Errorf("%d identifiers left of %d, want all but the one translated", len(left), len(ids))
	}
	for _, id := range left {
		if id == "settings.title" {
			t.Error("the one translated string is reported as untranslated")
		}
	}
}

func TestEnglishIsOfferedFirst(t *testing.T) {
	if got := Languages(); len(got) == 0 || got[0] != "en" {
		t.Errorf("the languages are %v", got)
	}
}


func TestGermanCatalogueIsCompleteAndSelectable(t *testing.T) {
	t.Cleanup(func() { Use(English.String()) })
	if got := Name("de"); got != "Deutsch" {
		t.Fatalf("German language name is %q, want Deutsch", got)
	}
	if got := Use("de"); got != "de" {
		t.Fatalf("German was not selected: got %q", got)
	}
	if got := T("settings.title"); got != "Einstellungen" {
		t.Errorf("German settings title is %q", got)
	}
	for _, tag := range []string{"en", "it", "vi", "de"} {
		if !slices.Contains(Languages(), tag) {
			t.Errorf("language %q is missing from the selector", tag)
		}
	}
	left, err := Untranslated("de")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("German catalogue is missing %d English identifiers, e.g. %v", len(left), left[:min(5, len(left))])
	}
	englishRaw, err := files.ReadFile("messages/en.yaml")
	if err != nil {
		t.Fatal(err)
	}
	germanRaw, err := files.ReadFile("messages/de.yaml")
	if err != nil {
		t.Fatal(err)
	}
	english, err := read(englishRaw)
	if err != nil {
		t.Fatal(err)
	}
	german, err := read(germanRaw)
	if err != nil {
		t.Fatal(err)
	}
	variables := regexp.MustCompile("{{\\\\.[^{}]+}}")
	for id, source := range english {
		want := variables.FindAllString(source, -1)
		got := variables.FindAllString(german[id], -1)
		sort.Strings(want)
		sort.Strings(got)
		if !slices.Equal(want, got) {
			t.Errorf("%s placeholders are %v, want %v", id, got, want)
		}
	}
}

func TestEnglishItalianAndVietnameseRemainSelectable(t *testing.T) {
	t.Cleanup(func() { Use(English.String()) })
	for _, tc := range []struct{ tag, title string }{
		{"en", "Settings"},
		{"it", "Impostazioni"},
		{"vi", "Cài đặt"},
	} {
		if got := Use(tc.tag); got != tc.tag {
			t.Errorf("selecting %s settled on %s", tc.tag, got)
		}
		if got := T("settings.title"); got != tc.title {
			t.Errorf("%s settings title is %q, want %q", tc.tag, got, tc.title)
		}
	}
}
