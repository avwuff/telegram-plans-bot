package dbHelper

import "testing"

func Test_cleanOldSyntaxText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "No changes",
			text: "Hello World",
			want: "Hello World",
		},
		{
			name: "Simple example",
			text: `/$\ud83d/$\udd7a/$\ud83c/$\udfffFriday DANSE Party!!/$\ud83e/$\udec3/$\ud83c/$\udffd`,
			want: `&#128378;&#127999;Friday DANSE Party!!&#129731;&#127997;`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanOldSyntaxText(tt.text); got != tt.want {
				t.Errorf("cleanOldSyntaxText() = %v, want %v", got, tt.want)
			}
		})
	}
}
