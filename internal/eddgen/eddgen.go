package eddgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strings"
	"text/template"
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
		typeNoArr = typeNoArr[arr:]
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
			return "object"
		}
	}

	return jsPrimitive(typeNoArr) + strings.Repeat("[]", arrRepeat)

}

type Struct struct {
	Name   string
	Fields []Field
	Doc    string
}

type Design struct {
	Module   string
	Name     string
	Channels []*Channel
	Structs  []*Struct
}

func NewDesign(module string) *Design {
	return &Design{Module: module}
}

func (design *Design) ParseAndValidate(filePath string) error {
	if err := design.Parse(filePath); err != nil {
		return err
	}
	return design.Validate()
}
func (design *Design) Parse(filePath string) error {

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file %s: %w\n", filePath, err)
	}
	defer func() { _ = file.Close() }()

	var fSet = token.NewFileSet() // positions are relative to fSet

	// Parse src but stop after processing the imports.
	f, err := parser.ParseFile(fSet, filePath, file, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("unable to parse file %s: %w", filePath, err)
	}

	design.Name = f.Name.Name

	for _, d := range f.Decls {
		var gen, ok = d.(*ast.GenDecl)
		if !ok {
			continue
		}

		if len(gen.Specs) == 0 {
			continue
		}

		t, ok := gen.Specs[0].(*ast.TypeSpec)
		if !ok {
			continue
		}

		switch tt := t.Type.(type) {
		case *ast.InterfaceType:
			var eddCh, err = ParseChannel(t.Name.Name, t.Doc.Text(), tt)
			if err != nil {
				return fmt.Errorf("unable to parse channel: %w", err)
			}
			design.Channels = append(design.Channels, eddCh)
		case *ast.StructType:
			var eddSt, err = ParseStruct(t.Name.Name, t.Doc.Text(), tt)
			if err != nil {
				return fmt.Errorf("unable to parse struct: %w", err)
			}
			design.Structs = append(design.Structs, eddSt)
		}

	}

	return nil

}

func (design *Design) Validate() error {
	//validate if direction of events in channels are ok
	for _, eddCh := range design.Channels {
		var mEnabled = eddCh.EnabledMap()
		for _, dir := range []Direction{ServerToClient, ClientToServer} {
			for t := range eddCh.Directions[dir] {
				if !mEnabled[t] {
					return fmt.Errorf("cannot define '%s' direction to not enabled message '%s'", dir, t)
				}
			}
		}
	}

	var structs = design.StructsMap()

	//validate if enabled type exists
	for _, eddCh := range design.Channels {
		for _, t := range eddCh.Enabled {
			if _, ok := structs[t]; !ok {
				return fmt.Errorf("unknown enabled '%s' type in channel '%s'", t, eddCh.Name)
			}
		}
	}

	//resolve type dependencies
	for _, s := range design.Structs {
		for _, f := range s.Fields {
			switch f.TypeName {
			case "bool", "byte", "complex128", "complex64", "error", "float32", "float64", "int", "int16", "int32", "int64", "int8", "rune", "string", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr":
				//builtin
			default:
				if strings.HasPrefix(f.TypeName, "map[") {
					//builtin
					continue
				}
				def, ok := structs[f.TypeName]
				if !ok {
					return fmt.Errorf("unknown type '%s' of field '%s' in struct '%s'", f.TypeName, f.Name, s.Name)
				}
				f.Type = Type{
					Name: f.TypeName,
					Def:  def,
				}
			}
		}
	}
	// entities named channel + "Context" and channel + "DefaultContext" and channel + "Recv" are reserved
	for _, eddCh := range design.Channels {
		var contextStructName = eddCh.Name + "Context"
		if _, ok := structs[contextStructName]; ok {
			return fmt.Errorf("invalid struct name '%s', because it is reserved for code generation", contextStructName)
		}
		contextStructName = eddCh.Name + "DefaultContext"
		if _, ok := structs[contextStructName]; ok {
			return fmt.Errorf("invalid struct name '%s', because it is reserved for code generation", contextStructName)
		}
		contextStructName = eddCh.Name + "Recv"
		if _, ok := structs[contextStructName]; ok {
			return fmt.Errorf("invalid struct name '%s', because it is reserved for code generation", contextStructName)
		}
	}

	return nil
}

func (design *Design) SkeletonServer(w io.Writer) error {

	var serverTmpl = `package main
{{ $Name := .Name }}
import (
	"log"

	"{{ .Module }}/gen/{{ $Name }}"

	"github.com/exelr/eddwise"
)

{{ range $ch := .Channels }}
type {{ $ch.Name }}Channel struct {
	{{ $Name }}.{{ $ch.Name }}
}

func (ch *{{ $ch.Name }}Channel) Connected(c *eddwise.Client) error {
	log.Println("User connected", c.GetId())
	return nil
}

func (ch *{{ $ch.Name }}Channel) Disconnected(c *eddwise.Client) error {
	log.Println("User disconnected", c.GetId())
	return nil
}

{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
func (ch *{{ $ch.Name }}Channel) On{{ $ev }} (ctx {{ $Name }}.{{ $ch.Name }}Context, {{ $ev | lower }} *{{ $Name }}.{{ $ev }}) error {
	log.Println("received event {{ $ev }}:", {{ $ev | lower }}, "from", ctx.GetClient().GetId() ) 
	return nil
}
{{ end }}
{{ end }}
func main() {
	var server = eddwise.NewServer()
	var ch eddwise.ImplChannel
{{ range $ch := .Channels }}
	ch = &{{ $ch.Name }}Channel{}
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service {{ $ch.Name }}: ", err)
	}
{{ end }}
	log.Fatalln(server.StartWS("/{{ .Name }}", 3000))
}

`

	tmpl, err := template.New("serverTmpl").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(serverTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, design)
	if err != nil {
		return err
	}
	return nil
}
func (design *Design) SkeletonClient(w io.Writer) error {

	var clientTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{ .Name }}</title>
</head>
<body>
<div id="controls">
{{ range $ch := .Channels }}
{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
<button onclick="ch{{ $ch.Name }}.send{{ $ev }}({})">Send {{ $ch.Name }}.{{ $ev }}</button>
{{ end }}
{{ end }}
</div>
<div id="output"></div>
<script src="../../eddclient.js"></script>
<script src="../../gen/{{ .Name }}/channel.js"></script>
<script>
  var wsUri = "ws://localhost:3000/{{ .Name }}"
  var client = new EddClient(wsUri)
{{ range $ch := .Channels }}
  var ch{{ $ch.Name }} = new {{ $ch.Name }}Channel()
{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
  ch{{ $ch.Name }}.on{{ $ev }}(function (event){
      document.getElementById("output").innerHTML += "Event '{{ $ev }}' received: " + JSON.stringify(event) + "<br>"
  })
{{ end }}
  client.register(ch{{ $ch.Name }})
{{ end }}
  
  client.start()

</script>
</body>
</html>

`

	tmpl, err := template.New("clientTmpl").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(clientTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, design)
	if err != nil {
		return err
	}
	return nil
}

func (design *Design) GenerateServer(w io.Writer) error {
	var serverTmpl = `// Code generated by eddwise, DO NOT EDIT.

package {{ .Name }}

import(
	"errors"

	"github.com/exelr/eddwise"
)
{{ range $ch := .Channels }}
var _ eddwise.ImplChannel = (*{{ $ch.Name }})(nil)
{{ end }}
{{ range $ch := .Channels }}
type {{ $ch.Name }}Context interface {
	eddwise.Context
	GetChannel() *{{ $ch.Name }}
}

type {{ $ch.Name }}DefaultContext struct {
	eddwise.Context
	channel * {{ $ch.Name }}
}

func (ctx *{{ $ch.Name }}DefaultContext) GetChannel() *{{ $ch.Name }} {
	return ctx.channel
}

type {{ $ch.Name }}Recv interface {
{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
	On{{ $ev }}({{ $ch.Name }}Context, *{{ $ev }}) error
{{ end }}
}

type {{ $ch.Name }} struct {
	server *eddwise.Server
	recv {{ $ch.Name }}Recv
}

func (ch *{{ $ch.Name }}) Name() string {
	return "{{ $ch.Name }}"
}

func (ch *{{ $ch.Name }}) Bind(server *eddwise.Server) error {
	ch.server = server
	return nil
}

func (ch *{{ $ch.Name }}) SetReceiver(chr interface{}) {
	ch.recv = chr.({{ $ch.Name }}Recv)
}

func (ch *{{ $ch.Name }}) GetServer() *eddwise.Server {
	return ch.server
}

func (ch *{{ $ch.Name }}) Route(ctx eddwise.Context, evt *eddwise.Event) error {
	var ctx2 = &{{ $ch.Name }}DefaultContext{
		Context: ctx,
		channel: ch,
	}
	switch evt.Name {
	default:
		return eddwise.ErrMissingServerHandler(evt.Channel, evt.Name)
{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
	case "{{ $ev }}":
		var msg = &{{ $ev }}{}
		if err := ch.server.Serializer.Deserialize(evt.Body, msg); err != nil {
			return err
		}
		return ch.recv.On{{ $ev }}(ctx2, msg)
{{ end }}
	}
}

{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
func (ch *{{ $ch.Name }}) On{{ $ev }}({{ $ch.Name }}Context, *{{ $ev }}) error {
	return errors.New("event '{{ $ev }}' is not handled on server")
}
{{ end }}

{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
func (ch *{{ $ch.Name }}) Send{{ $ev }}(client *eddwise.Client, msg *{{ $ev }}) error {
	return client.Send(ch.Name(), "{{ $ev }}", msg)
}
{{ end }}
{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
func (ch *{{ $ch.Name }}) Broadcast{{ $ev }}(clients []*eddwise.Client, msg *{{ $ev }}) error {
	return ch.server.Broadcast(ch.Name(), "{{ $ev }}", msg, clients)
}
{{ end }}

{{ end }}

// Event structures

{{ range $st := .Structs }}
type {{ $st.Name }} struct {
{{- range $field := $st.Fields -}}
	{{- if $field.HasDoc }}
	{{ $field.GoDoc }}{{ end }}
	{{ $field.GoName }} {{ $field.GoType }} {{ $field.GoAnnotation }}
{{- end }}
}

{{ end }}
`

	tmpl, err := template.New("serverTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
	}).Parse(serverTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, design)
	if err != nil {
		return err
	}
	return nil
}

func (design *Design) GenerateClient(w io.Writer) error {

	var clientTmpl = `// Code generated by eddwise, DO NOT EDIT.

{{ range $struct := .Structs -}}
/**
 * @typedef {{ $struct.Name }}
{{- range $field := $struct.Fields }}
 * @property {{ "{" }}{{ $field.JsType }}{{ "}" }} {{ if $field.TypePointer }}[{{ $field.Name }}]{{ else }}{{ $field.Name }}{{ end }} {{ $field.Doc | TrimSpace }}
{{ end -}}
*/

{{ end -}}
{{ range $ch := .Channels }}
class {{ .Name }}Channel {
	constructor() {
		Object.defineProperty(this, "getName", { configurable: false, writable: false, value: this.getName });
		Object.defineProperty(this, "setConn", { configurable: false, writable: false, value: this.setConn });
		Object.defineProperty(this, "route", { configurable: false, writable: false, value: this.route });
{{ range $event, $_ := $ch.GetDirectionEvents "ClientToServer"  }}
		Object.defineProperty(this, "{{ $event }}", { configurable: false, writable: false, value: this.send{{ $event }} });{{ end }}
{{ range $event, $_ := $ch.GetDirectionEvents "ServerToClient"  }}
		this._on{{ $event }}Fn = null;{{ end }}
		this._connectedFn = null;
		this._disconnectedFn = null;
	}
	/**
     * @callback connectedCb
     */
    /**
     * @function {{ $ch.Name }}Channel#connected
     * @param {connectedCb} callback
     */
	connected(callback){
		this._connectedFn = callback;
	}

	/**
     * @callback disconnectedCb
     */
    /**
     * @function {{ $ch.Name }}Channel#disconnected
     * @param {disconnectedCb} callback
     */
	disconnected(callback){
		this._disconnectedFn = callback;
	}
	getName() {
		return "{{ $ch.Name }}"
	}
	setConn(conn) {
		this.conn = conn
	}
	route(name, body) {
		switch(name) {
			default:
				console.log("unexpected event ", name, "in channel {{ $ch.Name }}")
				break
{{ range $event, $_ := $ch.GetDirectionEvents "ServerToClient"  }}
			case "{{ $event }}":
				return this.on{{ $event }}Fn(body)
{{ end }}
        }
    }

{{ range $event, $_ := $ch.GetDirectionEvents "ServerToClient"  }}
	/**
	 * @function {{ $ch.Name }}Channel#on{{ $event }}Fn
	 * @param {{ "{" }}{{ $event }}{{ "}" }} event
	*/
    on{{ $event }}Fn(event) {
        if(this._on{{ $event }}Fn == null) {
            console.log("unhandled message 'ChangeName' received")
            return
        }
        this._on{{ $event }}Fn(event)
    }
    /**
     * @callback on{{ $event }}Cb
     * @param {{ "{" }}{{ $event }}{{ "}" }} event
     */
    /**
     * @function {{ $ch.Name }}Channel#on{{ $event }}
     * @param {{ "{" }}on{{ $event }}Cb{{ "}" }} callback
     */
     on{{ $event }}(callback) {
        this._on{{ $event }}Fn = callback
    }
{{ end }}
{{ range $event, $_ := $ch.GetDirectionEvents "ClientToServer"  }}
    /**
     * @function {{ $ch.Name }}Channel#send{{ $event }}
     * @param {{ "{" }}{{ $event }}{{ "}" }} message
     */
    send{{ $event }} = function(message) {
        this.conn.send( JSON.stringify({channel:this.getName(), name:"{{ $event }}", body: message}) );
    }
{{ end }}
}
{{ end }}
`
	var tmpl, err = template.New("clientTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
	}).Parse(clientTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(w, design)
	if err != nil {
		return err
	}
	return nil
}

func (design *Design) StructsMap() map[string]*Struct {
	var ret = make(map[string]*Struct)
	for _, k := range design.Structs {
		ret[k.Name] = k
	}
	return ret
}

func ParseChannel(name, doc string, tt *ast.InterfaceType) (*Channel, error) {
	var eddCh = &Channel{
		Name:       name,
		Doc:        doc,
		Directions: make(map[Direction]map[string]bool),
	}
	if tt.Methods == nil {
		return eddCh, nil
	}

	for _, m := range tt.Methods.List {
		fnt, ok := m.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		if len(m.Names) == 0 {
			continue
		}
		directive := m.Names[0].Name
		switch directive {
		default:
			return nil, fmt.Errorf("unknown directive '%s' in channel %s", directive, eddCh.Name)
		case "Enable":
			if eddCh.Enabled != nil {
				return nil, fmt.Errorf("'Enable' declared twice in channel %s", eddCh.Name)
			}

			eddCh.Enabled = make([]string, 0)
			var mEnabled = make(map[string]bool)

			if fnt.Params == nil {
				break
			}

			for _, p := range fnt.Params.List {
				if ident, ok := p.Type.(*ast.Ident); ok {
					if mEnabled[ident.Name] {
						return nil, fmt.Errorf("try to enable '%s' in channel %s twice", ident.Name, eddCh.Name)
					}
					eddCh.Enabled = append(eddCh.Enabled, ident.Name)
					mEnabled[ident.Name] = true
				}
			}
		case ServerToClient, ClientToServer:
			if eddCh.Directions[Direction(directive)] != nil {
				return nil, fmt.Errorf("%s declared twice in channel %s", directive, eddCh.Name)
			}
			var m = make(map[string]bool)
			eddCh.Directions[Direction(directive)] = m
			if fnt.Params == nil {
				break
			}
			for _, p := range fnt.Params.List {
				if ident, ok := p.Type.(*ast.Ident); ok {
					if m[ident.Name] {
						return nil, fmt.Errorf("try to set direction %s for '%s' in channel %s twice", directive, ident.Name, eddCh.Name)
					}
					m[ident.Name] = true
				}
			}
		}
	}
	return eddCh, nil
}
func ParseStruct(name, doc string, tt *ast.StructType) (*Struct, error) {
	var eddSt = &Struct{
		Name: name,
		Doc:  doc,
	}
	var mfield = make(map[string]bool)
	if tt.Fields == nil {
		return eddSt, nil
	}
	for _, field := range tt.Fields.List {
		if len(field.Names) == 0 {
			return nil, fmt.Errorf("cannot parse anonymous fields in %s struct", name)
		}

		var fieldname = field.Names[0].Name
		//var fieldIdent *ast.Ident
		var IsPointer = false
		var typeName = ""
		switch t := field.Type.(type) {
		default:
			return nil, fmt.Errorf("cannot parse field %s in %s struct", fieldname, name)
		case *ast.MapType:
			typeName = "map[" + t.Key.(*ast.Ident).Name + "]"
			tv, ok := t.Value.(*ast.Ident)
			if !ok {
				return nil, fmt.Errorf("cannot parse field %s in %s struct (map value must not be a map)", fieldname, name)
			}
			typeName += tv.Name
		case *ast.Ident:
			typeName = t.Name
		case *ast.StarExpr:
			ti, ok := t.X.(*ast.Ident)
			if !ok {
				return nil, fmt.Errorf("cannot parse field %s in %s struct (multi level IsPointer not supported)", fieldname, name)
			}
			typeName = ti.Name
			IsPointer = true
		}

		if _, ok := mfield[fieldname]; ok {
			return nil, fmt.Errorf("field %s declared twice in %s", fieldname, name)
		}
		mfield[fieldname] = true
		eddSt.Fields = append(eddSt.Fields, Field{
			Name:        fieldname,
			TypeName:    typeName,
			TypePointer: IsPointer,
			Doc:         field.Doc.Text(),
		})
	}
	return eddSt, nil
}
