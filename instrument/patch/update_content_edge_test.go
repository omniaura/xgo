package patch

import (
	"testing"
)

// Additional test for empty sequence
func TestUpdateContent_EmptySequence(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic with empty sequence but got none")
		}
	}()

	UpdateContent("content", "/*<begin>*/", "/*<end>*/", []string{}, 0, UpdatePosition_Before, "add")
}

// Test for out of bounds index
func TestUpdateContent_OutOfBoundsIndex(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic with out of bounds index but got none")
		}
	}()

	UpdateContent("content", "/*<begin>*/", "/*<end>*/", []string{"content"}, 1, UpdatePosition_Before, "add")
}
