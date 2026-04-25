package utils

import (
	"reflect"
	"testing"
)

func TestParseYaml(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid yaml",
			data:    []byte("key: value"),
			want:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			data:    []byte("key: : value"),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]interface{}
			err := ParseYaml(tt.data, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseYaml() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseToml(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid toml",
			data:    []byte("key = \"value\""),
			want:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "invalid toml",
			data:    []byte("key = "),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]interface{}
			err := ParseToml(tt.data, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseToml() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEnv(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		target  interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid env",
			data:    []byte("KEY=VALUE"),
			target:  &map[string]interface{}{},
			want:    map[string]interface{}{"KEY": "VALUE"},
			wantErr: false,
		},
		{
			name:    "valid env nil map",
			data:    []byte("KEY=VALUE"),
			target:  new(map[string]interface{}),
			want:    map[string]interface{}{"KEY": "VALUE"},
			wantErr: false,
		},
		{
			name:    "invalid target",
			data:    []byte("KEY=VALUE"),
			target:  &struct{}{},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid env format",
			data:    []byte("!!!"),
			target:  &map[string]interface{}{},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseEnv(tt.data, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := *(tt.target.(*map[string]interface{}))
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ParseEnv() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
