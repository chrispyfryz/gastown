package beads

import "testing"

func TestCanAttachMoleculeToStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: StatusPinned, want: true},
		{status: StatusHooked, want: true},
		{status: "open", want: false},
		{status: "in_progress", want: false},
		{status: "closed", want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		if got := canAttachMoleculeToStatus(tt.status); got != tt.want {
			t.Errorf("canAttachMoleculeToStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}
