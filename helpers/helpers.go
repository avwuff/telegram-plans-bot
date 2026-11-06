package helpers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// richPreBlock finds the blocks in the text that can't be in a paragraph, so they are kept as they are.
var richPreBlock = regexp.MustCompile(`(?s)<pre>.*?</pre>`)

// Tuple provides a simple 2-item type
type Tuple struct {
	DisplayText string
	Key         string
}

// CalenFeedMD5 combines the ID and the sale to make a hash.
// TODO: This should no longer use MD5.
// Keeping this for compatibility.
func CalenFeedMD5(saltValue string, id int64) string {
	str := fmt.Sprintf("%v%v", id, saltValue)
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// StripHtmlRegex uses a regular expresion to remove HTML tags.
func StripHtmlRegex(s string) string {
	r := regexp.MustCompile(`<.*?>`)
	return html.UnescapeString(r.ReplaceAllString(s, ""))
}

// TelegramHtmlToRichHtml converts text in Telegram's classic HTML format to the format used by rich messages.
// Classic messages keep their line breaks and spacing, but rich messages are made of blocks, so the text
// goes in a paragraph with explicit line breaks. Pre-formatted text can't be in a paragraph, so it gets its own block.
func TelegramHtmlToRichHtml(s string) string {
	return richBlocks(s, "<p>", "</p>")
}

// TelegramHtmlToRichQuote is the same as TelegramHtmlToRichHtml, but the text goes in a quotation block instead.
func TelegramHtmlToRichQuote(s string) string {
	return richBlocks(s, "<blockquote>", "</blockquote>")
}

// richBlocks wraps the text between any pre-formatted blocks with the specified tags.
func richBlocks(s string, open string, closing string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")

	res := ""
	last := 0
	for _, loc := range richPreBlock.FindAllStringIndex(s, -1) {
		res += richTextBlock(s[last:loc[0]], open, closing) + s[loc[0]:loc[1]]
		last = loc[1]
	}
	return res + richTextBlock(s[last:], open, closing)
}

// richTextBlock wraps text in the specified tags, keeping its line breaks and the indentation of each line.
func richTextBlock(s string, open string, closing string) string {
	s = strings.Trim(s, "\n")
	if strings.TrimSpace(s) == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		lines[i] = strings.Repeat("&nbsp;", len(line)-len(trimmed)) + trimmed
	}
	return open + strings.Join(lines, "<br>") + closing
}

// HtmlEntities turns characters that can't be represented in ascii into html entities
func HtmlEntities(str string) string {
	str = html.EscapeString(str)
	res := ""
	runes := []rune(str)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 128 {
			res += string(r)
		} else {
			res += "&#" + strconv.FormatInt(int64(r), 10) + ";"
		}
	}
	return res
}

// ConvertEntitiesToHTML accepts the Telegram MessageEntity object and converts this funky format into HTML.
func ConvertEntitiesToHTML(inputText string, entities []tgbotapi.MessageEntity) string {
	if entities == nil {
		return HtmlEntities(inputText)
	}

	startTags := make(map[int][]tgbotapi.MessageEntity)
	endTags := make(map[int][]tgbotapi.MessageEntity)
	for _, entity := range entities {
		startTags[(entity.Offset)] = append(startTags[(entity.Offset)], entity)
		endTags[(entity.Offset + entity.Length)] = append(endTags[(entity.Offset+entity.Length)], entity)
	}
	html := ""
	text := utf16.Encode([]rune(inputText))

	var between []uint16

	for i, c := range text {

		if tags, ok := startTags[i]; ok {
			flush_between(&between, &html)
			for _, tag := range tags {
				html += startTagToText(tag, text)
			}
		}

		// add this to the between
		between = append(between, c)

		if tags, ok := endTags[i+1]; ok {
			flush_between(&between, &html)
			for j := len(tags) - 1; j >= 0; j-- {
				html += endTagToText(tags[j])
			}
		}
	}
	flush_between(&between, &html)
	return html
}

func flush_between(between *[]uint16, outHtml *string) {
	if len(*between) > 0 {
		*outHtml = *outHtml + HtmlEntities(string(utf16.Decode(*between)))
		*between = make([]uint16, 0) // reset
	}
}

func startTagToText(tag tgbotapi.MessageEntity, text []uint16) string {
	switch tag.Type {
	case "bold":
		return "<b>"
	case "italic":
		return "<i>"
	case "underline":
		return "<u>"
	case "strikethrough":
		return "<s>"
	case "code":
		return "<code>"
	case "pre":
		return "<pre>"
	case "text_link":
		return "<a href=\"" + tag.URL + "\">"
	case "mention":
		return "<a href=\"https://t.me/" + string(utf16.Decode(text[tag.Offset+1:(tag.Offset+tag.Length)])) + "\">"
	case "url":
		return "<a href=\"" + string(utf16.Decode(text[tag.Offset:(tag.Offset+tag.Length)])) + "\">"
	default:
		return ""
	}
}

func endTagToText(tag tgbotapi.MessageEntity) string {
	tags := map[string]string{
		"bold":          "</b>",
		"italic":        "</i>",
		"underline":     "</u>",
		"strikethrough": "</s>",
		"code":          "</code>",
		"pre":           "</pre>",
		"text_link":     "</a>",
		"mention":       "</a>",
		"url":           "</a>",
	}
	return tags[tag.Type]
}
