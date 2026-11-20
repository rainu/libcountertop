package setting

import "testing"

func TestMetadataNeverReadsConfigurationOrOtherFields(t *testing.T) {
	type cfg struct{ Value string }
	reads, saves, secretReads := 0, 0, 0
	c := &Controls[cfg]{
		Table: NewTable("test", []Group{"test"}, []Setting[cfg]{
			{Name: "safe", Kind: Choice, Group: "test", Options: []Option{{Value: "wire", Label: "Localized"}}, Read: func(c *cfg) string { return c.Value }, Write: func(c *cfg, v string) error { c.Value = v; return nil }},
			{Name: "private", Kind: Secret, Group: "test", Read: func(*cfg) string { secretReads++; return "synthetic" }, Write: func(*cfg, string) error { return nil }},
		}, Messages{}),
		Read: func() cfg { reads++; return cfg{Value: "wire"} },
		Save: func(Setting[cfg], string) error { saves++; return nil },
	}
	f, ok := c.Metadata("safe")
	if !ok || f.Value != "" || f.Choices[0].Value != "wire" || reads != 0 || secretReads != 0 {
		t.Fatal("metadata read values")
	}
	v, err := c.RawValue("safe")
	if err != nil || v != "wire" || secretReads != 0 {
		t.Fatal("raw value was localized or read unrelated field")
	}
	if c.Validate("safe", "next") != nil || saves != 0 || secretReads != 0 {
		t.Fatal("validation had effects")
	}
	if _, ok := c.Metadata("missing"); ok {
		t.Fatal("unknown admitted")
	}
}

func TestRawChangeDoesNotInterpretWireValueAsALabel(t *testing.T) {
	type cfg struct{ Value string }
	at := cfg{Value: "first"}
	c := &Controls[cfg]{
		Table: NewTable("test", []Group{"test"}, []Setting[cfg]{
			{Name: "mode", Kind: Choice, Group: "test", NoEntity: true, Options: []Option{{Value: "first", Label: "second"}, {Value: "second", Label: "Other"}}, Read: func(c *cfg) string { return c.Value }, Write: func(c *cfg, v string) error { c.Value = v; return nil }},
		}, Messages{}),
		Read: func() cfg { return at },
		Save: func(s Setting[cfg], v string) error { return s.Write(&at, v) },
	}
	if err := c.ChangeRaw("mode", "second"); err != nil || at.Value != "second" {
		t.Fatal("canonical value translated as label")
	}
}
