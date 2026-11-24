package setting

import "testing"

func TestOptionalGuidanceUsesTableMessages(t *testing.T) {
	type value struct{ Path string }
	at := value{}
	messages := map[string]string{"avatar.help.path": "Device path, not browser computer", "avatar.reset.path": "Use default"}
	table := NewTable("avatar", []Group{"model"}, []Setting[value]{Words(Setting[value]{Name: "path", Group: "model"}, 255, func(v *value) *string { return &v.Path })}, Messages{Text: func(k string) string { return messages[k] }, Missing: func(k string) bool { _, ok := messages[k]; return !ok }})
	c := Controls[value]{Table: table, Read: func() value { return at }, Save: func(s Setting[value], v string) error { return s.Write(&at, v) }}
	f := c.Fields()[0]
	m, _ := c.Metadata("path")
	if f.Help != messages["avatar.help.path"] || f.ResetLabel != messages["avatar.reset.path"] || f.Help != m.Help || f.ResetLabel != m.ResetLabel {
		t.Fatal(f, m)
	}
	c.ChangeRaw("path", "/custom.vrm")
	if err := c.ChangeRaw("path", ""); err != nil || at.Path != "" {
		t.Fatal(at, err)
	}
}
