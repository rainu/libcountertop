package setting

// Inspectable separates metadata and vetted single-field reads from Fields,
// which intentionally contains values for UI consumers.
type Inspectable interface {
	Metadata(name string) (Field, bool)
	RawValue(name string) (string, error)
	Validate(name, value string) error
	ChangeRaw(name, value string) error
}

// Metadata does not read configuration or invoke any setting's Read callback.
// Callers must authorize the exact domain/name before requesting metadata,
// since dynamic choice providers can themselves contain private identifiers.
func (c *Controls[T]) Metadata(name string) (Field, bool) {
	s, ok := c.Table.Find(name)
	if !ok || s.Name != name {
		return Field{}, false
	}
	f := Field{ID: s.Object(c.Table.Domain), Name: s.Name, Title: s.Title(), Group: c.Table.Title(s.Group), Kind: s.Kind.String(), Example: s.Example(), Help: s.OptionalText("help"), ResetLabel: s.OptionalText("reset"), Unit: s.Unit, Min: s.Min, Max: s.Max}
	for _, o := range s.Choices() {
		f.Choices = append(f.Choices, FieldChoice{Value: o.Value, Label: s.Label(o)})
	}
	return f, true
}

// RawValue returns the accepted wire representation, not the localized UI label.
// Authorization and sensitive-field exclusion belong to the caller.
func (c *Controls[T]) RawValue(name string) (string, error) {
	s, err := c.Table.Row(name)
	if err != nil {
		return "", err
	}
	at := c.Read()
	return s.Read(&at), nil
}

// Validate checks the setting's real parser against a disposable configuration;
// it never saves, publishes or returns the parser's potentially sensitive input.
func (c *Controls[T]) Validate(name, value string) error {
	s, err := c.Table.Row(name)
	if err != nil {
		return err
	}
	probe := c.Read()
	return s.Write(&probe, value)
}

// ChangeRaw saves an already validated wire representation without translating
// UI labels. A label can equal a different option's canonical wire value.
func (c *Controls[T]) ChangeRaw(name, value string) error {
	s, err := c.Table.Row(name)
	if err != nil {
		return err
	}
	probe := c.Read()
	if err := s.Write(&probe, value); err != nil {
		return err
	}
	if err := c.Save(s, value); err != nil {
		return err
	}
	c.Publish()
	return nil
}
