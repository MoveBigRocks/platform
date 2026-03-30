package servicedomain

import "testing"

func TestAttachmentStatusFromScan(t *testing.T) {
	tests := []struct {
		name      string
		isScanned bool
		result    string
		want      AttachmentStatus
	}{
		{name: "pending when not scanned", want: AttachmentStatusPending},
		{name: "clean result", isScanned: true, result: "clean", want: AttachmentStatusClean},
		{name: "mock clean result", isScanned: true, result: "No threats detected (mock)", want: AttachmentStatusClean},
		{name: "infected result", isScanned: true, result: "infected", want: AttachmentStatusInfected},
		{name: "error fallback", isScanned: true, result: "failed", want: AttachmentStatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AttachmentStatusFromScan(tt.isScanned, tt.result); got != tt.want {
				t.Fatalf("AttachmentStatusFromScan(%v, %q) = %q, want %q", tt.isScanned, tt.result, got, tt.want)
			}
		})
	}
}
