package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTOMLParser_Parse(t *testing.T) {
	parser := NewTOMLParser()

	t.Run("Valid TOML", func(t *testing.T) {
		data := []byte(`
key1 = "value1"
key2 = "value2"
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value1", parsed["key1"])
		assert.Equal(t, "value2", parsed["key2"])
	})

	t.Run("Nested TOML", func(t *testing.T) {
		data := []byte(`
[section]
key1 = "value1"
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		section, ok := parsed["section"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "value1", section["key1"])
	})

	t.Run("Empty TOML", func(t *testing.T) {
		data := []byte("")
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Empty(t, parsed)
	})

	t.Run("Invalid TOML", func(t *testing.T) {
		data := []byte(`
key1 = "value1"
invalid_toml
`)
		parsed, err := parser.Parse(data)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("TOML with Special Characters", func(t *testing.T) {
		data := []byte(`
key1 = "value with special characters: !@#$%^&*()"
key2 = "value with quotes: 'single' and \"double\""
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value with special characters: !@#$%^&*()", parsed["key1"])
		assert.Equal(t, "value with quotes: 'single' and \"double\"", parsed["key2"])
	})
}
