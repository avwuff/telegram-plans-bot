package localizer

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"time"
)

type Localizer struct {
	name    string
	printer *message.Printer
}

var locales = map[string]*Localizer{
	"de-DE": { // Germany
		name:    "Deutsch",
		printer: message.NewPrinter(language.MustParse("de-DE")),
	},
	"fr-CH": { // Switzerland (French speaking)
		name:    "Francais",
		printer: message.NewPrinter(language.MustParse("fr-CH")),
	},
	"en-US": { // United States
		name:    "English",
		printer: message.NewPrinter(language.MustParse("en-US")),
	},
}

// FromLanguage returns a localizer object from the specified language tag
func FromLanguage(locale string) *Localizer {
	loc, ok := locales[locale]
	if !ok {
		return locales["en-US"]
	}
	return loc
}

func FromLanguageName(name string) (string, error) {
	for l, loc := range locales {
		if loc.name == name {
			return l, nil
		}
	}
	return "", errors.New("language not found")
}

func GetLanguageChoices() tgbotapi.ReplyKeyboardMarkup {
	var keyboard [][]tgbotapi.KeyboardButton
	for _, loc := range locales {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(loc.name),
		))
	}

	return tgbotapi.ReplyKeyboardMarkup{
		ResizeKeyboard: true,
		Keyboard:       keyboard,
	}
}

func (l *Localizer) Sprintf(key message.Reference, args ...interface{}) string {
	return l.printer.Sprintf(key, args...)
}

func (l *Localizer) FormatDate(date time.Time) string {
	// TODO: A proper way to format date time.
	return date.Format("January 2, 2006 15:04")
}
