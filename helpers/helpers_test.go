package helpers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"testing"
)

func TestStripHtmlRegex(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "basic",
			s:    "hello world",
			want: "hello world",
		},
		{
			name: "simple tags",
			s:    "hello <b>world</b>",
			want: "hello world",
		},
		{
			name: "simple unclosed tag",
			s:    "hello <br>world",
			want: "hello world",
		},
		{
			name: "img tag",
			s:    "hello <img src=\"woo\">world",
			want: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHtmlRegex(tt.s); got != tt.want {
				t.Errorf("StripHtmlRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ConvertEntitiesToHTML(t1 *testing.T) {
	type args struct {
		Text     string
		Entities []tgbotapi.MessageEntity
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Simple",
			args: args{
				Text: "What if we tossed under the stars?",
				Entities: []tgbotapi.MessageEntity{
					{Type: "bold", Offset: 18, Length: 5},
					{Type: "italic", Offset: 18, Length: 5},
					{Type: "bold", Offset: 28, Length: 5},
				},
			},
			want: "What if we tossed <b><i>under</i></b> the <b>stars</b>?",
		},
		{
			name: "One emoji",
			args: args{
				Text:     "I am a 🧡 dog.",
				Entities: []tgbotapi.MessageEntity{{Type: "bold", Offset: 10, Length: 3}},
			},
			want: "I am a &#129505; <b>dog</b>.",
		},
		{
			name: "Flag emoji",
			args: args{
				Text: "🇨🇦: What kind of DOOOOOOOG are you? \n😈: I'm a man!",
				Entities: []tgbotapi.MessageEntity{
					{Type: "bold", Offset: 11, Length: 4},
					{Type: "italic", Offset: 11, Length: 4},
					{Type: "italic", Offset: 20, Length: 2},
				},
			},
			want: "&#127464;&#127462;: What <b><i>kind</i></b> of D<i>OO</i>OOOOOG are you? \n&#128520;: I&#39;m a man!",
		},
	}

	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t1.Log("Running test", tt.name)
			if got := ConvertEntitiesToHTML(tt.args.Text, tt.args.Entities); got != tt.want {
				t1.Errorf("ConvertEntitiesToHTML() = GOT:\n%v\n, WANT:\n%v\n", got, tt.want)
			}
		})
	}
}
