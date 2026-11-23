package setting

type FieldChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FilePicker is presentation metadata, not authorization or a format validator.
// Selection is submitted through the same setting parser as typed input.
type FilePicker struct {
	Extensions []string          `json:"extensions"`
	StartDir   string            `json:"start_dir"`
	Text       map[string]string `json:"text"`
}
type Field struct {
	FilePicker *FilePicker `json:"file_picker,omitempty"`

	Help       string        `json:"help,omitempty"`
	ResetLabel string        `json:"reset_label,omitempty"`
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Title      string        `json:"title"`
	Group      string        `json:"group"`
	Kind       string        `json:"kind"`
	Value      string        `json:"value"`
	Example    string        `json:"example,omitempty"`
	Unit       string        `json:"unit,omitempty"`
	Min        int           `json:"min,omitempty"`
	Max        int           `json:"max,omitempty"`
	Choices    []FieldChoice `json:"choices,omitempty"`
}

type Described interface {
	Fields() []Field
}

func (c *Controls[T]) Fields() []Field {
	at := c.Read()
	rows := c.Table.Rows()
	out := make([]Field, 0, len(rows))
	for _, s := range rows {
		f := Field{
			ID:      s.Object(c.Table.Domain),
			Name:    s.Name,
			Title:   s.Title(),
			Group:   c.Table.Title(s.Group),
			Kind:    s.Kind.String(),
			Example: s.Example(),
			Help:    s.OptionalText("help"), ResetLabel: s.OptionalText("reset"),
			Unit: s.Unit,
			Min:  s.Min,
			Max:  s.Max,
		}
		if s.FilePicker != nil && s.Kind == Text {
			f.FilePicker = s.FilePicker(&at)
		}
		if s.Kind == Secret {
			f.Value = s.Visible(&at)
		} else {
			f.Value = s.Read(&at)
		}
		for _, o := range s.Choices() {
			f.Choices = append(f.Choices, FieldChoice{Value: o.Value, Label: s.Label(o)})
		}
		out = append(out, f)
	}
	return out
}
