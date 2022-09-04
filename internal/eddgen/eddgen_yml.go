package eddgen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type YamlDesign struct {
	Namespace string                     `yaml:"namespace"`
	Structs   *MapSlice                  `yaml:"structs"`
	Channels  MapSliceType[*YamlChannel] `yaml:"channels"`
}

type YamlStruct struct {
	Alias  string               `yaml:"alias"`
	Fields map[string]yaml.Node `yaml:"fields"`
}

type YamlFullField struct {
	Alias string `yaml:"alias"`
	Type  string `yaml:"type"`
}

type YamlChannel struct {
	Tags   Tags
	Dual   YamlChannelEvents `yaml:"dual"`
	Server YamlChannelEvents `yaml:"server"`
	Client YamlChannelEvents `yaml:"client"`
}

func (yc *YamlChannel) UnmarshalYAML(node *yaml.Node) error {
	yc.Tags = ProcessTags(node)
	var m = struct {
		Dual   YamlChannelEvents `yaml:"dual"`
		Server YamlChannelEvents `yaml:"server"`
		Client YamlChannelEvents `yaml:"client"`
	}{}
	if err := node.Decode(&m); err != nil {
		return err
	}
	yc.Dual = m.Dual
	yc.Server = m.Server
	yc.Client = m.Client

	return nil
}

type YamlChannelEvents []YamlChannelEvent

type YamlChannelEvent struct {
	Tags  Tags
	Event string
}

func (yce *YamlChannelEvent) UnmarshalYAML(node *yaml.Node) error {
	yce.Tags = ProcessTags(node)
	yce.Event = node.Value
	return nil
}

func GetComments(n *yaml.Node) string {
	var comment string
	if len(n.HeadComment) > 0 {
		comment = strings.Trim(n.HeadComment, "# ")
	}
	if len(n.LineComment) > 0 {
		if len(comment) > 0 {
			comment += " "
		}
		comment += strings.Trim(n.LineComment, "# ")
	}
	if len(n.FootComment) > 0 {
		if len(comment) > 0 {
			comment += " "
		}
		comment += strings.Trim(n.FootComment, "# ")
	}
	return comment
}

func ParseAndValidateYamls(module string, filePaths ...string) (*Design, error) {
	var design, err = ParseYamls(module, filePaths...)
	if err != nil {
		return nil, err
	}
	if err := design.Validate(); err != nil {
		return nil, err
	}
	return design, nil
}

func ParseYamls(module string, filePaths ...string) (*Design, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("empty file list for yaml parsing")
	}
	var d = &Design{
		Module: module,
	}
	if err := d.ParseYaml(filePaths[0]); err != nil {
		return nil, fmt.Errorf("unable to parse %s: %w", filePaths[0], err)
	}
	for _, filePath := range filePaths[1:] {
		var d2 = &Design{
			Module: module,
		}
		if err := d2.ParseYaml(filePath); err != nil {
			return nil, fmt.Errorf("unable to parse %s: %w", filePath, err)
		}
		if err := d.Merge(d2); err != nil {
			return nil, fmt.Errorf("unable to merge %s: %w", filePath, err)
		}
	}
	return d, nil
}

func (design *Design) Merge(design2 *Design) error {
	if design.Name != design2.Name {
		return fmt.Errorf("namespace mismatch '%s' and '%s'", design.Name, design2.Name)
	}
	var stMap = design.StructsMap()
	for _, st := range design2.Structs {
		if _, ok := stMap[st.Name]; ok {
			return fmt.Errorf("struct '%s' declared previously", st.Name)
		}
		design.Structs = append(design.Structs, st)
	}

	var chMap = design.ChannelsMap()
	for _, ch := range design2.Channels {
		if _, ok := chMap[ch.Name]; ok {
			return fmt.Errorf("channel '%s' declared previously", ch.Name)
		}
		design.Channels = append(design.Channels, ch)
	}
	return nil
}

type Tags struct {
	Alias     string
	Direction Direction
}

func ProcessTags(node *yaml.Node) (t Tags) {
	t.Direction = Any
	var raw = node.Tag
	if !strings.HasPrefix(raw, "!!") {
		return
	}
	raw = raw[2:]
	var kvps = strings.Split(raw, ",")
	for _, kvp := range kvps {
		var key = kvp
		var value = ""
		if i := strings.Index(key, "="); i != -1 {
			key = kvp[:i]
			value = kvp[i+1:]
		}
		switch key {
		case "client":
			t.Direction = ClientToServer
		case "server":
			t.Direction = ServerToClient
		case "alias":
			t.Alias = value
		}
	}
	return
}

type MapItem struct {
	Key   string
	Value *yaml.Node
}

type MapSlice struct {
	Items []MapItem
}

func (ms *MapSlice) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("node at line %d is not a map", node.Line)
	}
	if len(node.Content)%2 != 0 {
		return fmt.Errorf("unable to read the map at line %d", node.Line)
	}
	ms.Items = make([]MapItem, 0, len(node.Content)/2)
	var unique = make(map[string]bool)

	for i := 0; i < len(node.Content); i += 2 {
		var keyNode = node.Content[i]
		var valueNode = node.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("unable to read the map key at line %d", keyNode.Line)
		}
		if unique[keyNode.Value] {
			return fmt.Errorf("duplicate map key detected at line %d", keyNode.Line)
		}
		unique[keyNode.Value] = true
		ms.Items = append(ms.Items, MapItem{
			Key:   keyNode.Value,
			Value: valueNode,
		})
	}
	return nil
}

type MapItemType[T yaml.Unmarshaler] struct {
	Key   string
	Value T
}

type MapSliceType[T yaml.Unmarshaler] struct {
	Items []MapItemType[T]
}

func (mss *MapSliceType[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("node at line %d is not a map", node.Line)
	}
	if len(node.Content)%2 != 0 {
		return fmt.Errorf("unable to read the map at line %d", node.Line)
	}
	mss.Items = make([]MapItemType[T], 0, len(node.Content)/2)
	var unique = make(map[string]bool)
	//var ms = make(MapSlice, 0, len(node.Content))
	for i := 0; i < len(node.Content); i += 2 {
		var keyNode = node.Content[i]
		var valueNode = node.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("unable to read the map key at line %d", keyNode.Line)
		}
		if unique[keyNode.Value] {
			return fmt.Errorf("duplicate map key detected at line %d", keyNode.Line)
		}
		unique[keyNode.Value] = true
		var t T
		if reflect.TypeOf(t).Kind() == reflect.Ptr {
			z := reflect.New(reflect.TypeOf(t).Elem())
			t = z.Interface().(T)

		}
		if err := t.UnmarshalYAML(valueNode); err != nil {
			return err
		}
		mss.Items = append(mss.Items, MapItemType[T]{
			Key:   keyNode.Value,
			Value: t,
		})
	}
	return nil
}

func (design *Design) ParseStruct(node *yaml.Node) (*Struct, error) {
	var st = &Struct{
		Tags: ProcessTags(node),
	}

	var fieldMap MapSlice
	err := fieldMap.UnmarshalYAML(node)
	if err != nil {
		return nil, err
	}
	for _, node := range fieldMap.Items {
		f, err := design.ParseField(node.Value)
		if err != nil {
			return nil, err
		}
		f.Name = node.Key
		st.Fields = append(st.Fields, f)
	}
	return st, nil
}

func ParseFieldStringType(value string) (*TypeWithAttributes, error) {
	var ret = &TypeWithAttributes{}
	for strings.HasPrefix(value, "+") {
		ret.Array++
		value = value[1:]
	}
	if i := strings.Index(value, "->"); i >= 0 {
		var mt = &MapType{}
		key := value[:i]
		if strings.HasPrefix(key, ".") {
			//mt.keyType = &RefType{ref: key[1:]}
			return nil, fmt.Errorf("map keys can only be primitives ('%s')", key)
		} else {
			mt.keyType = PrimitiveType(key)
		}
		vt, err := ParseFieldStringType(value[i+2:])
		if err != nil {
			return nil, err
		}
		mt.valueType = vt
		ret.Type = mt
	} else {
		if strings.HasPrefix(value, ".") {
			ret.Type = &RefType{ref: value[1:]}
		} else {
			ret.Type = PrimitiveType(value)
		}
	}
	return ret, nil
}

func (design *Design) ParseField(node *yaml.Node) (*Field, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		t, err := ParseFieldStringType(node.Value)
		if err != nil {
			return nil, err
		}
		var f = &Field{
			Tags: ProcessTags(node),
			Type: t,
			Doc:  GetComments(node),
		}
		return f, nil
	case yaml.MappingNode:
		var f = &Field{
			Tags: ProcessTags(node),
			Doc:  "",
		}
		nested, err := design.ParseStruct(node)
		if err != nil {
			return nil, err
		}
		f.Type = &TypeWithAttributes{
			Type:  &NestedType{Struct: nested},
			Array: 0,
		}
		return f, nil
	}

	return nil, errors.New("unknown field yaml type")
}

func (design *Design) ParseYaml(filePath string) error {
	var data, err = os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var yamlDesign YamlDesign
	err = yaml.Unmarshal(data, &yamlDesign)
	if err != nil {
		return fmt.Errorf("unable to parse yaml: %w", err)
	}

	design.Name = yamlDesign.Namespace

	var structAliasMap = make(map[string]*Struct)
	var structMap = make(map[string]*Struct)
	for _, stYaml := range yamlDesign.Structs.Items {
		st, err := design.ParseStruct(stYaml.Value)
		if err != nil {
			return err
		}
		st.Name = stYaml.Key

		if structAliasMap[st.ProtocolAlias()] != nil {
			return fmt.Errorf("struct alias `%s` already used", st.ProtocolAlias())
		}
		structAliasMap[st.ProtocolAlias()] = st
		structMap[st.Name] = st
		design.Structs = append(design.Structs, st)
	}

	for _, chYaml := range yamlDesign.Channels.Items {
		var ch = &Channel{
			Name:    chYaml.Key,
			Alias:   chYaml.Value.Tags.Alias,
			Doc:     "",
			Enabled: nil,
			Directions: map[Direction]map[string]bool{
				ServerToClient: {},
				ClientToServer: {},
			},
		}
		var uniqueSet = map[string]bool{}
		var dualWithDirection bool
		for _, node := range chYaml.Value.Dual {
			if _, ok := structMap[node.Event]; !ok {
				return fmt.Errorf("unknown dual event '%s' in channel '%s'", node.Event, ch.Name)
			}
			if uniqueSet[node.Event] {
				return fmt.Errorf("'%s' dual event is already registered to the channel '%s'", node.Event, ch.Name)
			}
			uniqueSet[node.Event] = true
			ch.Enabled = append(ch.Enabled, structMap[node.Event])
			if node.Tags.Direction != Any {
				dualWithDirection = true
				ch.Directions[node.Tags.Direction][node.Event] = true
			}
		}
		if dualWithDirection && (len(chYaml.Value.Server) > 0 || len(chYaml.Value.Client) > 0) {
			log.Println("you are mixing 'dual' tag direction and server/client")
		}
		for _, node := range chYaml.Value.Server {
			if _, ok := structMap[node.Event]; !ok {
				return fmt.Errorf("unknown server event '%s' in channel '%s'", node.Event, ch.Name)
			}
			if uniqueSet[node.Event] {
				return fmt.Errorf("'%s' server event is already registered to the channel '%s'", node.Event, ch.Name)
			}
			uniqueSet[node.Event] = true
			ch.Enabled = append(ch.Enabled, structMap[node.Event])
			ch.Directions[ServerToClient][node.Event] = true
		}

		for _, node := range chYaml.Value.Client {
			if _, ok := structMap[node.Event]; !ok {
				return fmt.Errorf("unknown client event '%s' in channel '%s'", node.Event, ch.Name)
			}
			if uniqueSet[node.Event] {
				return fmt.Errorf("'%s' client event is already registered to the channel '%s'", node.Event, ch.Name)
			}
			uniqueSet[node.Event] = true
			ch.Enabled = append(ch.Enabled, structMap[node.Event])
			ch.Directions[ClientToServer][node.Event] = true
		}

		design.Channels = append(design.Channels, ch)
	}

	return nil
}
