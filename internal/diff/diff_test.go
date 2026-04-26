package diff

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		name      string
		canonical map[string]interface{}
		target    map[string]interface{}
		wantDrift bool
		added     int
		removed   int
		changed   int
	}{
		{
			name: "identical configs",
			canonical: map[string]interface{}{
				"key1": "value1",
				"key2": 100,
			},
			target: map[string]interface{}{
				"key1": "value1",
				"key2": 100,
			},
			wantDrift: false,
		},
		{
			name: "numeric type mismatch - same value",
			canonical: map[string]interface{}{
				"port": 8080,
			},
			target: map[string]interface{}{
				"port": float64(8080),
			},
			wantDrift: false,
		},
		{
			name: "key added",
			canonical: map[string]interface{}{
				"key1": "value1",
			},
			target: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantDrift: true,
			added:     1,
		},
		{
			name: "key removed",
			canonical: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			target: map[string]interface{}{
				"key1": "value1",
			},
			wantDrift: true,
			removed:   1,
		},
		{
			name: "key changed",
			canonical: map[string]interface{}{
				"key1": "value1",
			},
			target: map[string]interface{}{
				"key1": "changed",
			},
			wantDrift: true,
			changed:   1,
		},
		{
			name: "nil values",
			canonical: map[string]interface{}{
				"key1": nil,
			},
			target: map[string]interface{}{
				"key1": nil,
			},
			wantDrift: false,
		},
		{
			name: "one side nil",
			canonical: map[string]interface{}{
				"key1": "value",
			},
			target: map[string]interface{}{
				"key1": nil,
			},
			wantDrift: true,
			changed:   1,
		},
		{
			name:      "empty maps",
			canonical: map[string]interface{}{},
			target:    map[string]interface{}{},
			wantDrift: false,
		},
		{
			name: "case sensitivity",
			canonical: map[string]interface{}{
				"key": "value",
			},
			target: map[string]interface{}{
				"Key": "value",
			},
			wantDrift: true,
			added:     1,
			removed:   1,
		},
		{
			name: "nested maps (not flattened)",
			canonical: map[string]interface{}{
				"db": map[string]interface{}{"host": "localhost"},
			},
			target: map[string]interface{}{
				"db": map[string]interface{}{"host": "remotehost"},
			},
			wantDrift: true,
			changed:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Compare(tt.name, tt.canonical, tt.target)
			if result.IsDrifted != tt.wantDrift {
				t.Errorf("Compare() IsDrifted = %v, want %v", result.IsDrifted, tt.wantDrift)
			}
			if len(result.Added) != tt.added {
				t.Errorf("Compare() Added count = %v, want %v", len(result.Added), tt.added)
			}
			if len(result.Removed) != tt.removed {
				t.Errorf("Compare() Removed count = %v, want %v", len(result.Removed), tt.removed)
			}
			if len(result.Changed) != tt.changed {
				t.Errorf("Compare() Changed count = %v, want %v", len(result.Changed), tt.changed)
			}
		})
	}
}

func TestValuesMatch(t *testing.T) {
	tests := []struct {
		name string
		a    interface{}
		b    interface{}
		want bool
	}{
		{"equal strings", "a", "a", true},
		{"unequal strings", "a", "b", false},
		{"equal ints", 1, 1, true},
		{"int vs int64", 1, int64(1), true},
		{"int vs float64", 1, 1.0, true},
		{"float32 vs float64", float32(1.1), 1.1, false},
		{"nil vs nil", nil, nil, true},
		{"nil vs string", nil, "s", false},
		{"slices equal", []int{1, 2}, []int{1, 2}, true},
		{"slices unequal", []int{1, 2}, []int{1, 3}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valuesMatch(tt.a, tt.b); got != tt.want {
				t.Errorf("valuesMatch() = %v, want %v for %v vs %v", got, tt.want, tt.a, tt.b)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	input := map[string]interface{}{
		"db": map[string]interface{}{
			"host": "localhost",
			"port": 3306,
			"users": []interface{}{
				"admin",
				"guest",
			},
		},
		"app": "my-app",
	}

	expected := map[string]interface{}{
		"db.host":     "localhost",
		"db.port":     3306,
		"db.users[0]": "admin",
		"db.users[1]": "guest",
		"app":         "my-app",
	}

	got := Flatten(input)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Flatten() = %v, want %v", got, expected)
	}
}

func TestFlattenDepthLimit(t *testing.T) {
	// create a map with a mix of shallow and deep keys
	input := map[string]interface{}{
		"shallow": "value",
		"deep": map[string]interface{}{
			"nested": map[string]interface{}{
				"very_nested": "value",
			},
		},
	}

	// FlattenWithDepth(input, 1) should only include "shallow"
	got := FlattenWithDepth(input, 1)
	if _, ok := got["shallow"]; !ok {
		t.Errorf("FlattenWithDepth() should have included 'shallow' key")
	}
	if len(got) != 1 {
		t.Errorf("FlattenWithDepth() expected size 1, got %d", len(got))
	}
}
