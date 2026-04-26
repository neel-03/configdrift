package diff

// DriftResult holds the result of a drift comparison
type DriftResult struct {
	TargetName string

	IsDrifted bool

	Added   []KeyDiff
	Removed []KeyDiff
	Changed []KeyDiff
}

// KeyDiff holds the difference details for a single key
type KeyDiff struct {
	Key            string
	CanonicalValue interface{}
	TargetValue    interface{}
}
