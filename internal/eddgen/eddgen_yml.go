package eddgen

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"strings"
)

type YamlDesign struct {
	Namespace string                 `yaml:"namespace"`
	Structs   map[string]YamlStruct  `yaml:"structs"`
	Channels  map[string]YamlChannel `yaml:"channels"`
}

type YamlDesignRaw struct {
	Structs  yaml.Node `yaml:"structs"`
	Channels yaml.Node `yaml:"channels"`
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
	Alias  string    `yaml:"alias"`
	Enable yaml.Node `yaml:"enable"`
}

func GetComments(n yaml.Node) string {
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

func (design *Design) ParseYaml(filePath string) error {
	var data, err = ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	var yamlDesignRaw YamlDesignRaw
	err = yaml.Unmarshal(data, &yamlDesignRaw)
	if err != nil {
		return fmt.Errorf("unable to parse yaml: %w", err)
	}
	var yamlDesign YamlDesign
	err = yaml.Unmarshal([]byte(data), &yamlDesign)
	if err != nil {
		return fmt.Errorf("unable to parse yaml: %w", err)
	}

	design.Name = yamlDesign.Namespace

	var structAliasMap = make(map[string]*Struct)
	var structMap = make(map[string]*Struct)
	for name, stYaml := range yamlDesign.Structs {
		var st = &Struct{
			Name:  name,
			Alias: stYaml.Alias,
		}
		var fieldAliasMap = make(map[string]bool)
		for fieldName, fieldYaml := range stYaml.Fields {
			var field = Field{
				Name:     fieldName,
				TypeName: fieldYaml.Value,
				Type:     Type{},
				Doc:      GetComments(fieldYaml),
			}
			if fieldYaml.Kind == yaml.MappingNode {
				var ff = YamlFullField{}
				if err = fieldYaml.Decode(&ff); err != nil {
					return fmt.Errorf("unexpected object in field %s: %w", fieldName, err)
				}
				field.Alias = ff.Alias
				field.TypeName = ff.Type
			}

			switch fieldYaml.Tag {
			case "!!map":
			case "!!str":
			case "!!server":
				field.Direction = ServerToClient
			case "!!client":
				field.Direction = ClientToServer
			default:
				return fmt.Errorf("unknown tag '%s' in field '%s'", fieldYaml.Tag, fieldName)
			}
			if fieldAliasMap[field.ProtocolAlias()] {
				return fmt.Errorf("field alias `%s` already used in struct `%s`", field.ProtocolAlias(), st.Name)
			}
			fieldAliasMap[field.ProtocolAlias()] = true
			st.Fields = append(st.Fields, field)
		}

		if structAliasMap[st.ProtocolAlias()] != nil {
			return fmt.Errorf("struct alias `%s` already used", st.ProtocolAlias())
		}
		structAliasMap[st.ProtocolAlias()] = st
		structMap[st.Name] = st
		design.Structs = append(design.Structs, st)
	}

	for name, chYaml := range yamlDesign.Channels {
		var ch = &Channel{
			Name:    name,
			Alias:   chYaml.Alias,
			Doc:     "",
			Enabled: nil,
			Directions: map[Direction]map[string]bool{
				ServerToClient: {},
				ClientToServer: {},
			},
		}
		for _, node := range chYaml.Enable.Content {
			ch.Enabled = append(ch.Enabled, structMap[node.Value])
			if node.Tag == "!!server" {
				ch.Directions[ServerToClient][node.Value] = true
			} else if node.Tag == "!!client" {
				ch.Directions[ClientToServer][node.Value] = true
			} else if node.Tag != "!!str" {
				return fmt.Errorf("unknown tag '%s' in channel '%s'", node.Tag, name)
			}
		}
		design.Channels = append(design.Channels, ch)
	}
	var stMap = design.StructsMap()
	for i := 0; i < len(yamlDesignRaw.Structs.Content); i += 2 {
		var node = yamlDesignRaw.Structs.Content[i]
		stMap[node.Value].Doc = GetComments(*node)
	}
	return nil
}
