package parser

// init registers all the supported parsers
func init() {
	RegisterParser(".yaml", func() Parser { return NewYAMLParser() })
	RegisterParser(".yml", func() Parser { return NewYAMLParser() })
	RegisterParser(".toml", func() Parser { return NewTOMLParser() })
	RegisterParser(".env", func() Parser { return NewEnvParser() })
}
