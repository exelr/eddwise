package eddgen

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

type Direction string

const (
	Any            = ""
	ServerToClient = "ServerToClient"
	ClientToServer = "ClientToServer"
)

func GoName(str string) string {
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(str, "_", " ")), " ", "")
}

type Channel struct {
	Name       string
	Alias      string
	Doc        string
	Enabled    []*Struct
	Directions map[Direction]map[string]bool
}

func (c *Channel) GoName() string {
	return GoName(c.Name)
}

func (c *Channel) EnabledMap() map[string]*Struct {
	var ret = make(map[string]*Struct)
	for _, k := range c.Enabled {
		ret[k.Name] = k
	}
	return ret
}

func (c *Channel) ProtocolAlias() string {
	if len(c.Alias) > 0 {
		return c.Alias
	}
	return c.Name
}

func (c *Channel) GetDirectionEvents(direction Direction) map[string]*Struct {
	var available = c.EnabledMap()
	var revDirection Direction = ClientToServer
	if direction == ClientToServer {
		revDirection = ServerToClient
	}
	if c.Directions[revDirection] == nil {
		return available
	}
	var events = c.Directions[revDirection]
	for k := range events {
		delete(available, k)
	}
	return available
}

type TypeWithAttributes struct {
	Type
	Array int
}

func (twa *TypeWithAttributes) GoType() string {
	return strings.Repeat("[]", twa.Array) + twa.Type.GoType()
}

func (twa *TypeWithAttributes) JsType() string {
	return twa.Type.JsType() + strings.Repeat("[]", twa.Array)
}

type Type interface {
	GoType() string
	JsType() string
}

type PrimitiveType string

func (t PrimitiveType) GoType() string {
	return string(t)
}
func (t PrimitiveType) JsType() string {
	switch t {
	case "any":
		return "any"
	case "bool":
		return "boolean"
	case "byte", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr":
		return "uint"
	case "int", "int16", "int32", "int64", "int8":
		return "int"
	case "float32", "float64":
		return "float"
	case "error", "rune", "string":
		return "string"
	default:
		return "unknown"
	}
}

type NestedType struct {
	Struct *Struct
}

func (t *NestedType) GoType() string {
	return t.Struct.GoDef()
}

func (t *NestedType) JsType() string {
	return "object"
}

type RefType struct {
	ref       string
	refStruct *Struct
}

func (t *RefType) GoType() string {
	return GoName(t.ref)
}

func (t *RefType) JsType() string {
	return t.ref
}

type MapType struct {
	key, value string
	keyType    Type
	valueType  *TypeWithAttributes
}

func (t *MapType) GoType() string {
	return fmt.Sprintf("map[%s]%s", t.keyType.GoType(), t.valueType.GoType())
}

func (t *MapType) JsType() string {
	return fmt.Sprintf("Object.<%s, %s>", t.keyType.JsType(), t.valueType.JsType())
}

type Field struct {
	Tags Tags
	Name string
	Type *TypeWithAttributes
	Doc  string
}

func (f *Field) GoType() string {
	if f.Tags.Direction != Any {
		return "*" + f.Type.GoType()
	}
	return f.Type.GoType()
}

func (f *Field) GoName() string {
	return GoName(f.Name)
}

func (f *Field) GoAnnotation() string {
	if f.Tags.Direction != Any {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", f.ProtocolAlias())
	}
	return fmt.Sprintf("`json:\"%s\"`", f.ProtocolAlias())
}
func (f *Field) DirectionDoc() string {
	if f.Tags.Direction != Any {
		return "// " + string(f.Tags.Direction)
	}
	return ""
}
func (f *Field) HasDoc() bool {
	return len(f.Doc) > 0
}

func (f *Field) GoDoc() string {
	if f.HasDoc() {
		var docWithCorrectFieldName = regexp.MustCompile("\\b"+f.Name+"\\b").ReplaceAllString(f.Doc, f.GoName())
		return fmt.Sprintf("// %s", strings.TrimSpace(docWithCorrectFieldName))
	}
	return ""
}

func (f *Field) JsType() string {
	return f.Type.JsType()
}

func (f *Field) ProtocolAlias() string {
	if len(f.Tags.Alias) > 0 {
		return f.Tags.Alias
	}
	return f.Name
}

type Struct struct {
	Tags   Tags
	Name   string
	Fields []*Field
	Doc    string
}

func (s *Struct) TopFieldsWithStrictDirection(direction Direction) []*Field {
	var ret = make([]*Field, 0)
	for i, f := range s.Fields {
		if f.Tags.Direction == direction {
			ret = append(ret, s.Fields[i])
		}
	}
	return ret
}

func (s *Struct) FieldsWithStrictDirection(direction Direction) []string {
	var ret = make([]string, 0)
	for i, f := range s.Fields {
		if f.Tags.Direction == direction {
			ret = append(ret, s.Fields[i].GoName())
		} else if f.Tags.Direction == Any {
			switch t := f.Type.Type.(type) {
			case *RefType:
				var sub = t.refStruct.FieldsWithStrictDirection(direction)
				for _, v := range sub {
					ret = append(ret, f.GoName()+"."+v)
				}
			case *NestedType:
				var sub = t.Struct.FieldsWithStrictDirection(direction)
				for _, v := range sub {
					ret = append(ret, f.GoName()+"."+v)
				}

			}

		}
	}
	return ret
}

func (s *Struct) GoDef() string {
	var tmplStruct = `
{{- if .HasDoc }}
{{ .GoDoc }}
{{ end -}}
{{- if .GoName }}
type {{ .GoName }} struct {
{{- else -}}
struct {
{{- end -}}
{{- range $field := .Fields -}}
	{{- if $field.HasDoc }}
	{{ $field.GoDoc }}{{ end }}
	{{ $field.GoName }} {{ $field.GoType }} {{ $field.GoAnnotation }} {{ $field.DirectionDoc }}
{{- end }}
}`

	tmpl, err := template.New("structTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
		"goname":    strings.Title,
	}).Parse(tmplStruct)

	if err != nil {
		panic(err)
	}
	var buf = bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, s)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

type FieldWithPath struct {
	*Field
	Path string
}

func (s *Struct) UnnestFieldMap() []*FieldWithPath {
	var ret = make([]*FieldWithPath, 0, len(s.Fields))
	for _, field := range s.Fields {
		ret = append(ret, &FieldWithPath{
			Field: field,
			Path:  field.Name,
		})
		if t, ok := field.Type.Type.(*NestedType); ok {
			sub := t.Struct.UnnestFieldMap()
			for _, s := range sub {
				s.Path = field.Name + "." + s.Path
				ret = append(ret, s)
			}
		}
	}
	return ret
}

func (s *Struct) JsDef() string {

	var allFields = s.UnnestFieldMap()
	var clientTmpl = `
/**
 * @typedef {{ .Struct.Name }}
{{- range $field := .Fields }}
 * @property {{ "{" }}{{ $field.JsType }}{{ "}" }} {{ if or (ne $field.Tags.Direction  "") }}[{{ $field.Path }}]{{ else }}{{ $field.Path }}{{ end }}{{ if gt (len $field.Doc) 0 }} - {{ $field.Doc | TrimSpace }}{{ end }} {{ $field.DirectionDoc }}
{{- end }}
{{- if gt (len .Struct.Doc) 0 }}
 * @description {{ .Struct.Doc }}
{{- end }}
*/`
	var tmpl, err = template.New("clientTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
	}).Parse(clientTmpl)
	if err != nil {
		panic(err)
	}
	var buf = bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, map[string]interface{}{
		"Struct": s,
		"Fields": allFields,
	})
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func (s *Struct) GoName() string {
	return GoName(s.Name)
}

func (s *Struct) ProtocolAlias() string {
	if len(s.Tags.Alias) > 0 {
		return s.Tags.Alias
	}
	return s.Name
}

func (s *Struct) HasDoc() bool {
	return len(s.Doc) > 0
}
func (s *Struct) GoDoc() string {
	if s.HasDoc() {
		var docWithCorrectFieldName = regexp.MustCompile("\\b"+s.Name+"\\b").ReplaceAllString(s.Doc, s.GoName())
		return fmt.Sprintf("// %s", strings.TrimSpace(docWithCorrectFieldName))
	}
	return ""
}

func LowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
