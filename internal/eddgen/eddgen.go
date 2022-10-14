package eddgen

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"sort"
	"strings"
	"text/template"
)

type Design struct {
	Module   string
	Name     string
	Channels []*Channel
	Structs  []*Struct

	structMap map[string]*Struct
}

func (design *Design) ValidateType(_type Type) error {
	structs := design.StructsMap()
	switch t := _type.(type) {
	case PrimitiveType:
		switch t {
		default:
			return fmt.Errorf("unknown type '%s'", t)
		case "any", "bool", "byte", "complex128", "complex64", "error", "float32", "float64", "int", "int16", "int32", "int64", "int8", "rune", "string", "uint", "uint16", "uint32", "uint64", "uint8":

		}
	case *RefType:
		if _, ok := structs[t.ref]; !ok {
			return fmt.Errorf("ref type '%s' is not defined", t.ref)
		}
		t.refStruct = structs[t.ref]
		if err := design.ValidateStruct(t.refStruct); err != nil {
			return err
		}
	case *NestedType:
		if err := design.ValidateStruct(t.Struct); err != nil {
			return err
		}
	case *MapType:
		if err := design.ValidateType(t.keyType); err != nil {
			return err
		}
		if err := design.ValidateType(t.valueType.Type); err != nil {
			return err
		}
	}
	return nil
}

func (design *Design) ValidateStruct(s *Struct) error {
	for _, f := range s.Fields {
		if err := design.ValidateType(f.Type.Type); err != nil {
			return err
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
				if mEnabled[t] == nil {
					return fmt.Errorf("cannot define '%s' direction to not enabled message '%s'", dir, t)
				}
			}
		}
	}

	var structs = design.StructsMap()

	//validate if enabled type exists
	for _, eddCh := range design.Channels {
		for _, t := range eddCh.Enabled {
			if _, ok := structs[t.Name]; !ok {
				return fmt.Errorf("unknown enabled '%s' type in channel '%s'", t.Name, eddCh.Name)
			}
		}
	}

	//resolve type dependencies
	for _, s := range design.Structs {
		if err := design.ValidateStruct(s); err != nil {
			return err
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

	sort.Slice(design.Structs, func(i, j int) bool {
		return design.Structs[i].Name < design.Structs[j].Name
	})
	sort.Slice(design.Channels, func(i, j int) bool {
		return design.Channels[i].Name < design.Channels[j].Name
	})

	return nil
}

func (design *Design) SkeletonServer(wMain io.Writer, wPackage io.Writer, wPackageTest io.Writer) error {

	var serverSkeletonMainTmpl = `package main
{{ $Name := .Name }}
import (
	"log"

	"{{ .Module }}/internal/{{ $Name }}"

	"github.com/exelr/eddwise"
)

func main() {
	var server = eddwise.NewServer()
	var ch eddwise.ImplChannel
{{ range $ch := .Channels }}
	ch = {{ $Name }}.New{{ $ch.GoName }}Channel()
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service {{ $ch.GoName }}: ", err)
	}
{{ end }}
	log.Fatalln(server.StartWS("/{{ .Name }}", 3000))
}
`

	tmpl, err := template.New("serverSkeletonMainTmpl").Funcs(template.FuncMap{
		"LowerFirst": LowerFirst,
		"goname":     GoName,
	}).Parse(serverSkeletonMainTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(wMain, design)
	if err != nil {
		return err
	}

	var serverSkeletonPackageTmpl = `package {{ .Name}}
{{ $Name := .Name }}
import (
	"log"

	"{{ .Module }}/gen/{{ $Name }}"

	"github.com/exelr/eddwise"
)

{{ range $ch := .Channels }}
type {{ $ch.GoName }}Channel struct {
	{{ $Name }}.{{ $ch.GoName }}
}

func New{{ $ch.GoName }}Channel() *{{ $ch.GoName }}Channel {
	return &{{ $ch.GoName }}Channel{}
}

func (ch *{{ $ch.GoName }}Channel) Connected(c eddwise.Client) error {
	log.Println("User connected", c.GetId())
	return nil
}

func (ch *{{ $ch.GoName }}Channel) Disconnected(c eddwise.Client) error {
	log.Println("User disconnected", c.GetId())
	return nil
}

{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
func (ch *{{ $ch.GoName }}Channel) On{{ $ev | goname }} (ctx eddwise.Context, {{ $ev | LowerFirst }} *{{ $Name }}.{{ $ev | goname }}) error {
	log.Println("received event {{ $ev | goname }}:", {{ $ev | LowerFirst }}, "from", ctx.GetClient().GetId() ) 
	return nil
}
{{ end }}
{{ end }}
`

	tmpl, err = template.New("serverSkeletonPackageTmpl").Funcs(template.FuncMap{
		"LowerFirst": LowerFirst,
		"goname":     GoName,
	}).Parse(serverSkeletonPackageTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(wPackage, design)
	if err != nil {
		return err
	}

	var serverSkeletonPackageTestTmpl = `package {{ .Name }}
{{ $Name := .Name }}
import (
	"testing"

	"{{ .Module }}/gen/{{ .Name }}/behave"

	"github.com/exelr/eddwise"
)

//Note: for the best BDD use command 'goconvey'
{{ range $ch := .Channels }}
func TestBasicScenario{{ $ch.GoName }}(t *testing.T) {
	var behave = {{ $Name }}behave.New{{ $ch.GoName }}Behave(t)
	behave.Given("an empty {{ $ch.Name }} channel", func() eddwise.ImplChannel { return New{{ $ch.GoName }}Channel() }, func() {
		var ch = behave.Recv().(*{{ $ch.GoName }}Channel)
		_ = ch // check ch status in test!
		behave.ThenClientJoins(1, func() {
			// after client joins, something would happen...
			// behave.ThenClientShouldReceiveEvent("with id 1", 1, &{{ $Name }}.Welcome{})
			// you can also use goconvey.Convey() to test more complex behavioural patterns
		})

	})
}
{{ end }}
`

	tmpl, err = template.New("serverSkeletonPackageTestTmpl").Funcs(template.FuncMap{
		"LowerFirst": LowerFirst,
		"goname":     GoName,
	}).Parse(serverSkeletonPackageTestTmpl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(wPackageTest, design)
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
<script type="module">
  import {EddClient} from '//localhost:3000/{{ .Name }}/edd.js'

  var wsUri = "ws://localhost:3000/{{ .Name }}"
  var client = new EddClient(wsUri)
{{ range $ch := .Channels }}
  import {{"{"}}{{ $ch.Name }}Channel{{"}"}} from '../../gen/{{ .Name }}/channel.js'

  var ch{{ $ch.Name }} = new {{ $ch.Name }}Channel()
  window.ch{{ $ch.Name }} = ch{{ $ch.Name }}

  ch{{ $ch.Name }}.connected(function(){
      document.getElementById("output").innerHTML += "[{{ $ch.Name }}] <span style='color: darkgreen'>Connected</span><br>"
  })

  ch{{ $ch.Name }}.disconnected(function(){
      document.getElementById("output").innerHTML += "[{{ $ch.Name }}] <span style='color: darkred'>Disconnected</span><br>"
  })
{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
  ch{{ $ch.Name }}.on{{ $ev }}(function ({{ $ev | LowerFirst }}){
      document.getElementById("output").innerHTML += "[{{ $ch.Name }}] Event '{{ $ev }}' received: " + JSON.stringify({{ $ev | LowerFirst }}) + "<br>"
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
		"LowerFirst": LowerFirst,
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
func ExecuteTemplateWithGoFmt(tmpl *template.Template, w io.Writer, data interface{}) error {
	var buf = bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, data); err != nil {
		return err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	_, err = w.Write(formatted)
	return err
}
func (design *Design) GenerateServer(w io.Writer) error {
	var serverTmpl = `// Code generated by eddwise, DO NOT EDIT.

package {{ .Name }}

import(
	"errors"

	"github.com/exelr/eddwise"
)
{{ range $ch := .Channels }}
var _ eddwise.ImplChannel = (*{{ $ch.GoName }})(nil)
var _ {{ $ch.GoName }}Recv = (*{{ $ch.GoName }})(nil)
{{ end }}
{{ range $ch := .Channels }}
type {{ $ch.GoName }}Recv interface {
{{- range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
	On{{ $ev | goname }}(eddwise.Context, *{{ $ev | goname }}) error
{{- end }}
}

type {{ $ch.GoName }} struct {
	server eddwise.Server
	recv {{ $ch.GoName }}Recv
}

func (ch *{{ $ch.GoName }}) Name() string {
	return "{{ $ch.Name }}"
}

func (ch *{{ $ch.GoName }}) Alias() string {
	return "{{ $ch.ProtocolAlias }}"
}

func (ch *{{ $ch.GoName }}) Bind(server eddwise.Server) error {
	ch.server = server
	return nil
}

func (ch *{{ $ch.GoName }}) SetReceiver(chr eddwise.ImplChannel) error {
	if _, ok := chr.({{ $ch.GoName }}Recv); !ok {
		return errors.New("unexpected channel type while SetReceiver on '{{ $ch.GoName }}' channel")
	}
	ch.recv = chr.({{ $ch.GoName }}Recv)
	return nil
}

func (ch *{{ $ch.GoName }}) GetServer() eddwise.Server {
	return ch.server
}

func (ch *{{ $ch.GoName }}) Route(ctx eddwise.Context, evt *eddwise.EventMessage) error {
	switch evt.Name {
	default:
		return eddwise.ErrMissingServerHandler(evt.Channel, evt.Name)
{{ range $ev, $evData := $ch.GetDirectionEvents "ClientToServer" }}
	// {{ $ev }}
	case "{{ $evData.ProtocolAlias }}":
		var msg = &{{ $ev | goname }}{}
		if err := ch.server.Codec().Decode(evt.Body, msg); err != nil {
			return err
		}
		if err := msg.CheckReceivedFields(); err != nil {
			return err
		}
		return ch.recv.On{{ $ev | goname }}(ctx, msg)
{{- end }}
	}
}

{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
func (ch *{{ $ch.GoName }}) On{{ $ev | goname }}(eddwise.Context, *{{ $ev | goname }}) error {
	return errors.New("event '{{ $ev | goname }}' is not handled on server")
}
{{ end }}

{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
func (ch *{{ $ch.GoName }}) Send{{ $ev | goname }}(client eddwise.Client, msg *{{ $ev | goname }}) error {
	return client.Send(ch.Alias(), msg)
}
{{ end }}
{{ range $ev, $_ := $ch.GetDirectionEvents "ServerToClient" }}
func (ch *{{ $ch.GoName }}) Broadcast{{ $ev | goname }}(clients []eddwise.Client, msg *{{ $ev | goname }}) error {
	return eddwise.Broadcast(ch.Alias(), msg, clients)
}
{{ end }}

{{ end }}

// Event structures
{{ range $st := .Structs }}
{{ $st.GoDef }}

func (evt *{{ $st.GoName }}) GetEventName() string {
	return "{{ $st.Name }}"
}

func (evt *{{ $st.GoName }}) ProtocolAlias() string {
	return "{{ $st.ProtocolAlias }}"
}

func (evt *{{ $st.GoName }}) CheckSendFields() error {
{{- range $field := $st.FieldsWithStrictDirection "ClientToServer" }} 
	if evt.{{ $field }} != nil {
		return errors.New("{{ $st.GoName }}.{{ $field }} must not be set")
	}
{{- end }}
	return nil
}

func (evt *{{ $st.GoName }}) CheckReceivedFields() error {
{{- range $field := $st.FieldsWithStrictDirection "ServerToClient" }} 
	if evt.{{ $field }} != nil {
		return errors.New("{{ $st.GoName }}.{{ $field }} is an invalid field")
	}
{{- end }}
	return nil
}

{{- range $field := $st.TopFieldsWithStrictDirection "ServerToClient" }} 
func (evt *{{ $st.GoName }}) Set{{ $field.GoName }}({{ $field.Name }} {{ $field.Type.GoType }}) *{{ $st.GoName }} {
	evt.{{ $field.GoName }} = &{{ $field.Name }}
	return evt
}
{{- end }}

{{ end }}
`

	tmpl, err := template.New("serverTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
		"goname":    GoName,
	}).Parse(serverTmpl)
	if err != nil {
		return err
	}

	err = ExecuteTemplateWithGoFmt(tmpl, w, design)
	if err != nil {
		return err
	}
	return nil
}

func (design *Design) GenerateServerTest(w io.Writer) error {
	var serverTmpl = `// Code generated by eddwise, DO NOT EDIT.
{{ $Name := .Name }}
package {{ .Name }}behave

import (
	"testing"

	"{{ .Module }}/gen/{{ .Name }}"
	
	"github.com/exelr/eddwise"
	"github.com/exelr/eddwise/mock"
)
{{ range $ch := .Channels }}
type {{ $ch.GoName }}Behave struct {
	*mock.ChannelBehave
}

func New{{ $ch.GoName }}Behave(t *testing.T) *{{ $ch.GoName }}Behave {
	return &{{ $ch.GoName }}Behave{
		ChannelBehave: mock.NewBehaveChannel(t),
	}
}

func (cb *{{ $ch.GoName }}Behave) Recv() {{ $Name }}.{{ $ch.GoName }}Recv {
	return cb.ChannelBehave.Recv().({{ $Name }}.{{ $ch.GoName }}Recv)
}

{{ range $ev, $_ := $ch.GetDirectionEvents "ClientToServer" }}
func (cb *{{ $ch.GoName }}Behave) On{{ $ev | goname }}(clientId uint64, evt *{{ $Name }}.{{ $ev | goname }}, f ...func()) {
	cb.On(clientId,
		func(ctx eddwise.Context) error {
			return cb.Recv().On{{ $ev | goname }}(ctx, evt)
		}, evt, f...)
}
{{ end }}

{{ end }}
`

	tmpl, err := template.New("serverTmpl").Funcs(template.FuncMap{
		"TrimSpace": strings.TrimSpace,
		"goname":    GoName,
	}).Parse(serverTmpl)
	if err != nil {
		return err
	}
	err = ExecuteTemplateWithGoFmt(tmpl, w, design)
	if err != nil {
		return err
	}
	return nil
}

func (design *Design) GenerateClient(w io.Writer) error {

	var clientTmpl = `// Code generated by eddwise, DO NOT EDIT.

{{ range $struct := .Structs -}}
{{ $struct.JsDef }}
{{ end -}}
import {EddChannel} from "/{{ .Name }}/edd.js";
{{ range $ch := .Channels }}
class {{ .Name }}Channel extends EddChannel {
	constructor() {
		super("{{ $ch.ProtocolAlias }}")
		Object.defineProperty(this, "getName", { configurable: false, writable: false, value: this.getName });
		Object.defineProperty(this, "setClient", { configurable: false, writable: false, value: this.setClient });
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
	getAlias() {
		return "{{ $ch.ProtocolAlias }}"
	}
	setClient(client) {
		this.client = client
	}
	route(name, body) {
		if(super.route(name,body,this.getAlias())){
            return
        }
		switch(name) {
			default:
				console.log("unexpected event ", name, "in channel {{ $ch.Name }}")
				break
{{ range $event, $eventData := $ch.GetDirectionEvents "ServerToClient"  }}
			// {{ $event }}
			case "{{ $eventData.ProtocolAlias }}":
				return this.on{{ $event }}Fn(body)
{{- end }}
        }
    }

{{ range $event, $eventData := $ch.GetDirectionEvents "ServerToClient"  }}
	/**
	 * @function {{ $ch.Name }}Channel#on{{ $event }}Fn
	 * @param {{ "{" }}{{ $event }}{{ "}" }} event
	*/
    on{{ $event }}Fn(event) {
        if(this._on{{ $event }}Fn == null) {
            console.log("unhandled message '{{ $event }}' received")
            return
        }
		{{- range $field := $eventData.Fields -}}
			{{- if ne $field.Name $field.ProtocolAlias }}
		Object.defineProperty(event, "{{ $field.Name }}", Object.getOwnPropertyDescriptor(event, "{{ $field.ProtocolAlias }}")); delete event["{{ $field.ProtocolAlias }}"];
			{{- end }}
		{{- end }}
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
{{ range $event, $eventData := $ch.GetDirectionEvents "ClientToServer"  }}
    /**
     * @function {{ $ch.Name }}Channel#send{{ $event }}
     * @param {{ "{" }}{{ $event }}{{ "}" }} message
     */
    send{{ $event }} = function(message) {
		{{- range $field := $eventData.Fields -}}
			{{- if ne $field.Name $field.ProtocolAlias }}
		Object.defineProperty(message, "{{ $field.ProtocolAlias }}", Object.getOwnPropertyDescriptor(message, "{{ $field.Name }}")); delete message["{{ $field.Name }}"];
			{{- end }}
		{{- end }}
        return this.client.send({channel:this.getAlias(), name:"{{ $eventData.ProtocolAlias }}", body: message});
    }
{{ end }}
}
{{ end }}
export {
{{- range $i, $ch := .Channels -}}
	{{- if eq $i 0 -}}
	{{ .Name }}Channel
	{{- else -}}{{ .Name }}Channel,
	{{- end -}}
{{- end -}}
}
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

func (design *Design) ChannelsMap() map[string]*Channel {
	var ret = make(map[string]*Channel)
	for _, k := range design.Channels {
		ret[k.Name] = k
	}
	return ret
}

func (design *Design) StructsMap() map[string]*Struct {
	if design.structMap == nil {
		design.structMap = make(map[string]*Struct)
		for _, k := range design.Structs {
			design.structMap[k.Name] = k
		}
	}
	return design.structMap
}
