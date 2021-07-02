package eddgen

import (
	"fmt"
	"regexp"
	"strings"
)

type Direction string

const (
	ServerToClient = "ServerToClient"
	ClientToServer = "ClientToServer"
)

type Channel struct {
	Name       string
	Doc        string
	Enabled    []string
	Directions map[Direction]map[string]bool
}

func (c *Channel) GoName() string {
	return strings.Title(c.Name)
}
func (c *Channel) EnabledMap() map[string]bool {
	var ret = make(map[string]bool)
	for _, k := range c.Enabled {
		ret[k] = true
	}
	return ret
}

func (c *Channel) GetDirectionEvents(direction Direction) map[string]bool {
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
	TypeName    string
	TypePointer bool
	Type        Type
	Doc         string
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
	return f.TypeName
}

func (f *Field) GoName() string {
	return strings.Title(f.Name)
}

func (f *Field) GoAnnotation() string {
	if f.TypePointer {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", f.Name)
	}
	return fmt.Sprintf("`json:\"%s\"`", f.Name)
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

type Struct struct {
	Name   string
	Fields []Field
	Doc    string
}

func (s *Struct) GoName() string {
	return strings.Title(s.Name)
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
