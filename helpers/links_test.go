package helpers

import (
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
)

func Test_ConvertEntitiesToHTML_escapesLinks(t *testing.T) {
	const attack = `https://a.example/"><script>alert(1)</script>`

	t.Run("text link", func(t *testing.T) {
		got := ConvertEntitiesToHTML("click", []tgbotapi.MessageEntity{{Type: "text_link", Length: 5, URL: attack}})

		assert.Equal(t, `<a href="https://a.example/&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;">click</a>`, got)
	})

	t.Run("url", func(t *testing.T) {
		got := ConvertEntitiesToHTML(attack, []tgbotapi.MessageEntity{{Type: "url", Length: len(attack)}})

		assert.NotContains(t, got, "<script>")
		assert.Contains(t, got, `href="https://a.example/&#34;&gt;`)
	})

	t.Run("mention", func(t *testing.T) {
		got := ConvertEntitiesToHTML(`@a"b`, []tgbotapi.MessageEntity{{Type: "mention", Length: 4}})

		assert.Contains(t, got, `href="https://t.me/a&#34;b"`)
	})

	t.Run("ampersands in a link", func(t *testing.T) {
		got := ConvertEntitiesToHTML("click", []tgbotapi.MessageEntity{{Type: "text_link", Length: 5, URL: "https://a.example/?a=1&b=2"}})

		assert.Equal(t, `<a href="https://a.example/?a=1&amp;b=2">click</a>`, got)
	})
}
