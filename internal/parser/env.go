package parser

import "github.com/neel-03/configdrift/internal/utils"

type EnvParser struct{}

func NewEnvParser() *EnvParser {
	return &EnvParser{}
}

// Parse Env data into the provided target structure.
func (ep *EnvParser) Parse(data []byte) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := utils.ParseEnv(data, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
