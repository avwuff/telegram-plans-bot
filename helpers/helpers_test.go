package helpers

import (
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
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
		{
			name: "url in text",
			args: args{
				Text: "FOOOO BAR\nLet's go here!\nFoobars event: https://foobars.com/events/1353073",
				Entities: []tgbotapi.MessageEntity{
					{Type: "url", Offset: 40, Length: 34},
				},
			},
			want: "FOOOO BAR\nLet&#39;s go here!\nFoobars event: <a href=\"https://foobars.com/events/1353073\">https://foobars.com/events/1353073</a>",
		},
	}

	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t1.Log("Running test", tt.name)
			got := ConvertEntitiesToHTML(tt.args.Text, tt.args.Entities)
			assert.Equal(t1, tt.want, got)
		})
	}
}

func TestTelegramHtmlToRichQuote(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{name: "empty", s: "", want: ""},
		{name: "line breaks", s: "one\n\n two", want: "<blockquote>one<br><br>&nbsp;two</blockquote>"},
		{name: "pre stays outside the quote", s: "before\n<pre>code</pre>\nafter", want: "<blockquote>before</blockquote><pre>code</pre><blockquote>after</blockquote>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TelegramHtmlToRichQuote(tt.s))
		})
	}
}

func TestTelegramHtmlToRichHtml(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{name: "empty", s: "", want: ""},
		{name: "line breaks", s: "<b>Title</b>\nline two\n\nafter blank", want: "<p><b>Title</b><br>line two<br><br>after blank</p>"},
		{name: "outer line breaks are trimmed", s: "\n\nhello\n", want: "<p>hello</p>"},
		{name: "windows line breaks", s: "one\r\ntwo", want: "<p>one<br>two</p>"},
		{name: "indentation is kept", s: " - name\n   - <i>guest</i>", want: "<p>&nbsp;- name<br>&nbsp;&nbsp;&nbsp;- <i>guest</i></p>"},
		{name: "formatting across a blank line stays in one block", s: "<b>one\n\ntwo</b>", want: "<p><b>one<br><br>two</b></p>"},
		{name: "pre gets its own block and keeps its contents", s: "before\n<pre>  code\n  more</pre>\nafter", want: "<p>before</p><pre>  code\n  more</pre><p>after</p>"},
		{name: "only pre", s: "<pre>code</pre>", want: "<pre>code</pre>"},
		{name: "entities are left alone", s: "a &amp; b &#128512;", want: "<p>a &amp; b &#128512;</p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TelegramHtmlToRichHtml(tt.s))
		})
	}
}
