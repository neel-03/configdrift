package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// ParseYaml unmarshals raw YAML bytes into the provided target structure.
func ParseYaml(data []byte, target interface{}) error {
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("yaml parse failed: %w", err)
	}
	return nil
}

// ParseToml unmarshals raw TOML bytes into the provided target structure.
func ParseToml(data []byte, target interface{}) error {
	if err := toml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("toml parse failed: %w", err)
	}
	return nil
}

// ParseEnv parses .env style data into a map.
// It is tolerant to minor format issues and skips invalid lines.
func ParseEnv(data []byte, target *map[string]interface{}) error {
	if *target == nil {
		*target = make(map[string]interface{})
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip empty or comment lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// support "export KEY=VALUE"
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		// split key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// skip invalid line instead of failing hard
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if key == "" {
			continue
		}

		(*target)[key] = processValue(val)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("env scan error: %w", err)
	}

	return nil
}

func processValue(val string) string {
	raw := val
	// remove inline comments (only if unquoted)
	if !isQuoted(raw) {
		if idx := strings.Index(raw, " #"); idx != -1 {
			raw = raw[:idx]
		}
	}

	val = strings.TrimSpace(raw)
	val = stripQuotes(val)
	return unescape(val)
}

func isQuoted(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'')
}

func stripQuotes(s string) string {
	if isQuoted(s) {
		return s[1 : len(s)-1]
	}
	return s
}

func unescape(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
				i++
				continue
			case 't':
				result.WriteByte('\t')
				i++
				continue
			case 'r':
				result.WriteByte('\r')
				i++
				continue
			case '\\':
				result.WriteByte('\\')
				i++
				continue
			case '"':
				result.WriteByte('"')
				i++
				continue
			}
		}
		result.WriteByte(s[i])
	}

	return result.String()
}
