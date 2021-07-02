package eddgen

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
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
	Fields map[string]yaml.Node `yaml:"fields"`
}

type YamlChannel struct {
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
	var data, err = os.ReadFile(filePath)
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

	for name, stYaml := range yamlDesign.Structs {
		var st = &Struct{
			Name: name,
		}
		for fieldName, fieldYaml := range stYaml.Fields {
			var field = Field{
				Name:     fieldName,
				TypeName: fieldYaml.Value,
				Type:     Type{},
				Doc:      GetComments(fieldYaml),
			}

			switch fieldYaml.Tag {
			case "!!str":
			case "!!server":
				field.Direction = ServerToClient
			case "!!client":
				field.Direction = ClientToServer
			default:
				return fmt.Errorf("unknown tag '%s' in field '%s'", fieldYaml.Tag, fieldName)
			}

			st.Fields = append(st.Fields, field)
		}
		design.Structs = append(design.Structs, st)
	}

	for name, chYaml := range yamlDesign.Channels {
		var ch = &Channel{
			Name:    name,
			Doc:     "",
			Enabled: nil,
			Directions: map[Direction]map[string]bool{
				ServerToClient: {},
				ClientToServer: {},
			},
		}
		for _, node := range chYaml.Enable.Content {
			ch.Enabled = append(ch.Enabled, node.Value)
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
