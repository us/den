package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateVolumeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myvolume", false},
		{"valid with dots", "my.volume", false},
		{"valid with dashes", "my-volume", false},
		{"valid with underscores", "my_volume", false},
		{"valid alphanumeric", "vol123", false},
		{"empty", "", true},
		{"starts with dot", ".hidden", true},
		{"starts with dash", "-bad", true},
		{"single char", "a", true},
		{"contains slash", "a/b", true},
		{"contains space", "a b", true},
		{"too long", string(make([]byte, 65)), true},
		{"reserved prefix den-", "den-volume", true},
		{"reserved prefix DEN-", "DEN-volume", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNamespacedVolumeName(t *testing.T) {
	assert.Equal(t, "den-myvolume", NamespacedVolumeName("myvolume"))
	assert.Equal(t, "den-data", NamespacedVolumeName("data"))
}

func TestValidateVolumeMountPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute", "/mnt/data", false},
		{"valid nested", "/opt/app/data", false},
		{"relative path", "data", true},
		{"traversal", "/mnt/../etc/passwd", false}, // Clean resolves this safely
		{"null byte", "/mnt/da\x00ta", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeMountPath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
