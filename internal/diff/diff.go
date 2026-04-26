package diff

import "reflect"

// Compare performs a deep comparison between canonical and target configurations
// and returns a DriftResult indicating differences.
func Compare(name string, canonical, target map[string]interface{}) DriftResult {
	result := DriftResult{
		TargetName: name,
	}

	// first, we check every key in the source of truth (canonical)
	for key, cVal := range canonical {
		tVal, exists := target[key]

		if !exists {
			// if the key is missing in the target, it's a removal
			result.Removed = append(result.Removed, KeyDiff{
				Key:            key,
				CanonicalValue: cVal,
			})
			continue
		}

		// check if the values actually differ using our optimized matcher
		if !valuesMatch(cVal, tVal) {
			result.Changed = append(result.Changed, KeyDiff{
				Key:            key,
				CanonicalValue: cVal,
				TargetValue:    tVal,
			})
		}
	}

	// then, we look for keys that exist in the target but not in canonical
	for key, tVal := range target {
		if _, exists := canonical[key]; !exists {
			// these are extra keys found on the remote/target side
			result.Added = append(result.Added, KeyDiff{
				Key:         key,
				TargetValue: tVal,
			})
		}
	}

	// finally, mark if any drift was found at all
	result.IsDrifted = len(result.Added) > 0 || len(result.Removed) > 0 || len(result.Changed) > 0

	return result
}

// valuesMatch is our "smart" comparator. it handles the annoying case where
// one parser (like yaml) returns an int while another (like toml) returns an int64.
func valuesMatch(a, b interface{}) bool {
	// simple nil check first
	if a == nil || b == nil {
		return a == b
	}

	// check types once
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)

	// optimization: if types match and are comparable, check equality immediately
	if typeA == typeB && typeA.Comparable() {
		return a == b
	}

	// production-grade: handle numeric type mismatches (int vs float64 etc)
	// this stops "port: 80" from triggering drift just because of type differences
	aNum, aOk := toFloat64(a)
	bNum, bOk := toFloat64(b)
	if aOk && bOk {
		return aNum == bNum
	}

	// if types differ and they weren't numbers, they can't be equal unless one is a named type of the other
	// but reflect.DeepEqual handles that and more complex cases (slices, maps)
	return reflect.DeepEqual(a, b)
}

// toFloat64 is a helper to normalize various numeric types for comparison.
func toFloat64(v interface{}) (float64, bool) {
	switch i := v.(type) {
	case float64:
		return i, true
	case int:
		return float64(i), true
	case int64:
		return float64(i), true
	case float32:
		return float64(i), true
	case int32:
		return float64(i), true
	case uint:
		return float64(i), true
	case uint64:
		return float64(i), true
	case int16:
		return float64(i), true
	case int8:
		return float64(i), true
	case uint32:
		return float64(i), true
	case uint16:
		return float64(i), true
	case uint8:
		return float64(i), true
	default:
		return 0, false
	}
}
