package setting

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFilePickerMetadataIsOptInAndUsesCurrentValue(t *testing.T) {
	type value struct{ Path string }
	c := &Controls[value]{Read: func() value { return value{Path: "/images"} }}
	c.Table = NewTable("assets", []Group{"model"}, []Setting[value]{
		{Name: "image", Kind: Text, Read: func(v *value) string { return v.Path }, FilePicker: func(v *value) *FilePicker {
			return &FilePicker{StartDir: v.Path, Extensions: []string{".png"}, Text: map[string]string{"choose_image": "Choose"}}
		}},
		{Name: "other", Kind: Text, Read: func(*value) string { return "" }},
		{Name: "secret", Kind: Secret, Read: func(*value) string { return "hidden" }, FilePicker: func(*value) *FilePicker { t.Fatal("secret picker called"); return nil }},
	}, Messages{})
	fields := c.Fields()
	if len(fields) != 3 || fields[0].FilePicker == nil || fields[0].FilePicker.StartDir != "/images" || fields[1].FilePicker != nil || fields[2].FilePicker != nil {
		t.Fatal(fields)
	}
	b, _ := json.Marshal(fields[0])
	if !strings.Contains(string(b), "file_picker") {
		t.Fatal(string(b))
	}
}
