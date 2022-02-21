package eddgen

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Direction string

const (
	Any            = ""
	ServerToClient = "ServerToClient"
	ClientToServer = "ClientToServer"
)

type Channel struct {
	Name       string
	Doc        string
	Enabled    []*Struct
	Directions map[Direction]map[string]bool
}

func (c *Channel) GoName() string {
	return strings.Title(c.Name)
}
func (c *Channel) EnabledMap() map[string]*Struct {
	var ret = make(map[string]*Struct)
	for _, k := range c.Enabled {
		ret[k.Name] = k
	}
	return ret
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

type Type struct {
	Name string
	Def  *Struct //if Def is nil then it is a primitive
}

type Field struct {
	Name        string
	Alias       string
	TypeName    string
	TypePointer bool
	Type        Type
	Doc         string
	Direction   Direction
}

func (f *Field) RawType() string {
	var arr = strings.LastIndex(f.TypeName, "[]")
	var typeNoArr = f.TypeName

	if arr > -1 {
		typeNoArr = typeNoArr[arr+2:]
	}
	return typeNoArr
}

func (f *Field) GoType() string {
	if f.TypePointer {
		return "*" + f.TypeName
	}
	if f.Direction != Any {
		return "*" + f.TypeName
	}
	return f.TypeName
}

func (f *Field) GoName() string {
	return strings.Title(f.Name)
}

func (f *Field) GoAnnotation() string {
	if f.TypePointer {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", f.ProtocolAlias())
	}
	if f.Direction != Any {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", f.ProtocolAlias())
	}
	return fmt.Sprintf("`json:\"%s\"`", f.ProtocolAlias())
}
func (f *Field) DirectionDoc() string {
	if f.Direction != Any {
		return "// " + string(f.Direction)
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
	var arr = strings.LastIndex(f.TypeName, "[]")
	var arrRepeat = 0
	var typeNoArr = f.TypeName

	if f.Type.Def != nil {
		return typeNoArr + strings.Repeat("[]", arrRepeat)
	}

	if arr > -1 {
		typeNoArr = typeNoArr[arr+2:]
		arrRepeat = arr/2 + 1
	}

	var jsPrimitive = func(t string) string {
		switch t {
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
			if strings.HasPrefix(typeNoArr, "map[") {
				return "object"
			}
			return typeNoArr
		}
	}

	return jsPrimitive(typeNoArr) + strings.Repeat("[]", arrRepeat)

}

func (f *Field) ProtocolAlias() string {
	if len(f.Alias) > 0 {
		return f.Alias
	}
	return f.Name
}

type Struct struct {
	Name   string
	Alias  string
	Fields []Field
	Doc    string
}

func (s *Struct) FieldsWithStrictDirection(direction Direction) []*Field {
	var ret = make([]*Field, 0)
	for i, f := range s.Fields {
		if f.Direction == direction {
			ret = append(ret, &s.Fields[i])
		}
	}
	return ret
}

func (s *Struct) GoName() string {
	return strings.Title(s.Name)
}

func (s *Struct) ProtocolAlias() string {
	if len(s.Alias) > 0 {
		return s.Alias
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
