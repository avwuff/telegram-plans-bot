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

var locales map[string]*Localizer
var timezones map[string]*time.Location

func InitLang() {
	locales = map[string]*Localizer{
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

	// Get the list from here
	// https://github.com/Lewington-pitsos/golang-time-locations
	list := []string{
		"America/Toronto",
		"America/Chicago",
		"America/Los_Angeles",
		"Europe/London",
	}

	// create the time zones
	timezones = make(map[string]*time.Location)
	for _, tz := range list {
		var err error
		timezones[tz], err = time.LoadLocation(tz)
		if err != nil {
			panic(err)
		}
	}
}

// FromTimeZone will return a time zone based on the specified input
func FromTimeZone(timeZone string) *time.Location {
	tz, ok := timezones[timeZone]
	if !ok {
		return timezones["America/Los_Angeles"]
	}
	return tz
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

func GetLanguageChoicesMap() map[string]string {
	out := make(map[string]string)
	for key, loc := range locales {
		out[key] = loc.name
	}
	return out
}

func GetTimeZoneChoicesMap() map[string]*time.Location {
	out := make(map[string]*time.Location)
	for key, loc := range timezones {
		out[key] = loc
	}
	return out
}

func (l *Localizer) Sprintf(key message.Reference, args ...interface{}) string {
	return l.printer.Sprintf(key, args...)
}

func (l *Localizer) FormatDate(date time.Time) string {
	// TODO: A proper way to format date time.
	return date.Format("January 2, 2006 15:04")
}
