package parser

import "github.com/neel-03/configdrift/internal/utils"

// TOMLParser handles parsing of TOML files.
type TOMLParser struct{}

// NewTOMLParser creates a new TOMLParser.
func NewTOMLParser() *TOMLParser {
	return &TOMLParser{}
}

// Parse parses the given TOML data into a map.
func (tp *TOMLParser) Parse(data []byte) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := utils.ParseToml(data, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
