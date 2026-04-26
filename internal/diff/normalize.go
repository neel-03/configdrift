package diff

import (
	"log/slog"

	"github.com/neel-03/configdrift/internal/utils"
)

// DefaultMaxDepth is the default recursion depth for flattening.
const defaultMaxDepth = 20

// Flatten transforms nested configuration into a flat map
// with dot-separated keys using the maximum default depth (i.e., 20).
func Flatten(input map[string]interface{}) map[string]interface{} {
	return FlattenWithDepth(input, defaultMaxDepth)
}

// FlattenWithDepth transforms nested configuration into a flat map
// with dot-separated keys, respecting the provided max depth.
func FlattenWithDepth(input map[string]interface{}, maxDepth int) map[string]interface{} {
	output := make(map[string]interface{})
	recursiveFlatten(input, output, "", 0, maxDepth)
	return output
}

func recursiveFlatten(input interface{}, output map[string]interface{}, prefix string, depth int, maxDepth int) {
	if depth > maxDepth {
		// stopping recursion if depth limit is reached to prevent stack overflow
		slog.Warn("stopping recursion: depth limit reached", "depth", depth)
		return
	}

	switch in := input.(type) {
	// case 1: the input is a map, iterate over the keys and recursively flatten the values
	case map[string]interface{}:
		for key, subdir := range in {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			recursiveFlatten(subdir, output, newPrefix, depth+1, maxDepth)
		}
	// case 2: the input is a slice, iterate over the elements and recursively flatten the values
	case []interface{}:
		for i, ele := range in {
			key := prefix + utils.IndexToKey(i)
			recursiveFlatten(ele, output, key, depth+1, maxDepth)
		}
	// case 3: the input is a primitive type, add it to the output map
	default:
		if prefix != "" {
			output[prefix] = in
		}
	}
}
