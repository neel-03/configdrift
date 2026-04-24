package parser

import "github.com/neel-03/configdrift/internal/utils"

type TOMLParser struct{}

func NewTOMLParser() *TOMLParser {
	return &TOMLParser{}
}

func (tp *TOMLParser) Parse(data []byte) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := utils.ParseToml(data, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
