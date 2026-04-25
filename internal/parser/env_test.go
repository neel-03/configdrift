package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvParser_Parse(t *testing.T) {
	parser := NewEnvParser()

	t.Run("Valid Env", func(t *testing.T) {
		data := []byte(`
KEY1=value1
KEY2="value2"
KEY3='value3'
export EXPORT_KEY=exported_value
# This is a comment
  # This is also a comment
KEY4=value with spaces
KEY5="value with # hash"
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "value1", parsed["KEY1"])
		assert.Equal(t, "value2", parsed["KEY2"])
		assert.Equal(t, "value3", parsed["KEY3"])
		assert.Equal(t, "exported_value", parsed["EXPORT_KEY"])
		assert.Equal(t, "value with spaces", parsed["KEY4"])
		assert.Equal(t, "value with # hash", parsed["KEY5"])
	})

	t.Run("Empty Env", func(t *testing.T) {
		data := []byte("")
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Empty(t, parsed)
	})

	t.Run("Invalid Env", func(t *testing.T) {
		// godotenv.Unmarshal is specific about its format.
		// A line without = is generally considered invalid if it's not a comment or empty.
		data := []byte("INVALID_LINE\nANOTHER_INVALID")
		parsed, err := parser.Parse(data)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("Multiline Env", func(t *testing.T) {
		data := []byte(`
KEY="line1
line2"
`)
		parsed, err := parser.Parse(data)
		assert.NoError(t, err)
		assert.Equal(t, "line1\nline2", parsed["KEY"])
	})
}
