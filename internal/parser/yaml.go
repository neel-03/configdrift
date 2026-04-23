package parser

import "github.com/neel-03/configdrift/internal/utils"

type YAMLParser struct{}

func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

// Parse unmarshals YAML data into the provided target structure.
func (yp *YAMLParser) Parse(data []byte) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := utils.ParseYaml(data, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
