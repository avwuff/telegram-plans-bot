package tgWrapper

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"testing"
)

func TestTelegram_ConvertEntitiesToHTML(t1 *testing.T) {
	type args struct {
		Text     string
		Entities *[]tgbotapi.MessageEntity
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
				Entities: &[]tgbotapi.MessageEntity{
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
				Entities: &[]tgbotapi.MessageEntity{{Type: "bold", Offset: 10, Length: 3}},
			},
			want: "I am a 🧡 <b>dog</b>.",
		},
		{
			name: "Flag emoji",
			args: args{
				Text: "🇨🇦: What kind of DOOOOOOOG are you? \n😈: I'm a man!",
				Entities: &[]tgbotapi.MessageEntity{
					{Type: "bold", Offset: 11, Length: 4},
					{Type: "italic", Offset: 11, Length: 4},
					{Type: "italic", Offset: 20, Length: 2},
				},
			},
			want: "🇨🇦: What <b><i>kind</i></b> of D<i>OO</i>OOOOOG are you?\n😈: I'm a man!",
		},
	}

	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			fmt.Println("Running test", tt.name)
			t := &Telegram{}
			if got := t.ConvertEntitiesToHTML(tt.args.Text, tt.args.Entities); got != tt.want {
				t1.Errorf("ConvertEntitiesToHTML() = GOT:\n%v\n, WANT:\n%v\n", got, tt.want)
			}
		})
	}
}
