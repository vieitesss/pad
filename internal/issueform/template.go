package issueform

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type FieldType string

const (
	FieldMarkdown   FieldType = "markdown"
	FieldTextarea   FieldType = "textarea"
	FieldInput      FieldType = "input"
	FieldCheckboxes FieldType = "checkboxes"
)

type Option struct {
	Label string
}

type Field struct {
	Type        FieldType
	ID          string
	Label       string
	Description string
	Placeholder string
	Required    bool
	Options     []Option
	Markdown    string
}

type Template struct {
	Path   string
	Name   string
	Title  string
	Fields []Field
}

type rawTemplate struct {
	Name  string     `yaml:"name"`
	Title string     `yaml:"title"`
	Body  []rawField `yaml:"body"`
}

type rawField struct {
	Type        string        `yaml:"type"`
	ID          string        `yaml:"id"`
	Attributes  rawAttributes `yaml:"attributes"`
	Validations rawValidation `yaml:"validations"`
}

type rawAttributes struct {
	Label       string      `yaml:"label"`
	Description string      `yaml:"description"`
	Placeholder string      `yaml:"placeholder"`
	Value       string      `yaml:"value"`
	Options     []rawOption `yaml:"options"`
}

type rawOption struct {
	Label string `yaml:"label"`
}

type rawValidation struct {
	Required bool `yaml:"required"`
}

func Parse(path string, data []byte) (Template, error) {
	var raw rawTemplate
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Template{}, fmt.Errorf("decode YAML: %w", err)
	}

	tmpl := Template{
		Path:  strings.TrimSpace(path),
		Name:  strings.TrimSpace(raw.Name),
		Title: strings.TrimSpace(raw.Title),
	}

	for index, item := range raw.Body {
		field, err := parseField(item)
		if err != nil {
			return Template{}, fmt.Errorf("body[%d]: %w", index, err)
		}
		tmpl.Fields = append(tmpl.Fields, field)
	}

	if len(tmpl.EditableFields()) == 0 {
		return Template{}, fmt.Errorf("template has no editable fields")
	}

	return tmpl, nil
}

func (t Template) EditableFields() []Field {
	fields := make([]Field, 0, len(t.Fields))
	for _, field := range t.Fields {
		if field.Type == FieldMarkdown {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func (t Template) FieldByID(id string) (Field, bool) {
	for _, field := range t.Fields {
		if field.ID == id {
			return field, true
		}
	}
	return Field{}, false
}

func (t Template) WithAppendedField(field Field) Template {
	t.Fields = append(append([]Field{}, t.Fields...), field)
	return t
}

func parseField(raw rawField) (Field, error) {
	fieldType := FieldType(strings.TrimSpace(raw.Type))
	field := Field{
		Type:        fieldType,
		ID:          strings.TrimSpace(raw.ID),
		Label:       strings.TrimSpace(raw.Attributes.Label),
		Description: strings.TrimSpace(raw.Attributes.Description),
		Placeholder: strings.TrimRight(raw.Attributes.Placeholder, "\n"),
		Required:    raw.Validations.Required,
		Markdown:    strings.TrimSpace(raw.Attributes.Value),
	}

	switch fieldType {
	case FieldMarkdown:
		return field, nil
	case FieldTextarea, FieldInput:
		if field.ID == "" {
			return Field{}, fmt.Errorf("%s field is missing id", fieldType)
		}
		if field.Label == "" {
			return Field{}, fmt.Errorf("%s field %q is missing label", fieldType, field.ID)
		}
		return field, nil
	case FieldCheckboxes:
		if field.ID == "" {
			return Field{}, fmt.Errorf("checkboxes field is missing id")
		}
		if field.Label == "" {
			return Field{}, fmt.Errorf("checkboxes field %q is missing label", field.ID)
		}
		for _, option := range raw.Attributes.Options {
			label := strings.TrimSpace(option.Label)
			if label == "" {
				continue
			}
			field.Options = append(field.Options, Option{Label: label})
		}
		if len(field.Options) == 0 {
			return Field{}, fmt.Errorf("checkboxes field %q has no options", field.ID)
		}
		if len(field.Options) > 1 {
			return Field{}, fmt.Errorf("checkboxes field %q has %d options; pad supports one option per checkbox field", field.ID, len(field.Options))
		}
		return field, nil
	default:
		return Field{}, fmt.Errorf("unsupported field type %q", raw.Type)
	}
}
