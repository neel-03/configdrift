package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYAMLParser_Parse(t *testing.T) {
	parser := NewYAMLParser()

	t.Run("Valid YAML", func(t *testing.T) {
		data := []byte(`
key1: value1
key2:
  nestedKey: nestedValue
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value1", parsed["key1"])
		assert.Equal(t, "nestedValue", parsed["key2"].(map[string]interface{})["nestedKey"])
	})

	t.Run("Valid YML", func(t *testing.T) {
		data := []byte(`
key1: value1
key2:
  nestedKey: nestedValue
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value1", parsed["key1"])
		assert.Equal(t, "nestedValue", parsed["key2"].(map[string]interface{})["nestedKey"])
	})

	t.Run("Empty YAML", func(t *testing.T) {
		data := []byte("")
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Empty(t, parsed)
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		data := []byte(`
key1: value1
key2
  nestedKey: nestedValue
`)
		parsed, err := parser.Parse(data)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("Non-YAML Content", func(t *testing.T) {
		data := []byte("This is not a YAML file.")
		parsed, err := parser.Parse(data)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("YAML with Special Characters", func(t *testing.T) {
		data := []byte(`
key1: "value with special characters: !@#$%^&*()"
key2: "value with quotes: 'single' and \"double\""
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value with special characters: !@#$%^&*()", parsed["key1"])
		assert.Equal(t, "value with quotes: 'single' and \"double\"", parsed["key2"])
	})
}
