package localizer

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"strings"
	"time"
)

const (
	usDateFormat = "January 2, 2006 3:04 PM"
	euDateFormat = "2 January 2006 15:04"
)

type Localizer struct {
	name       string
	iso639code string
	dateFormat string // the date format that is most common in this culture
	printer    *message.Printer
}

var locales map[string]*Localizer
var timezones map[string]*time.Location

func InitLang() {
	locales = map[string]*Localizer{
		"de-DE": { // Germany
			name:       "Deutsch",
			iso639code: "de",
			dateFormat: euDateFormat,
			printer:    message.NewPrinter(language.MustParse("de-DE")),
		},
		"fr-CH": { // Switzerland (French speaking)
			name:       "Francais",
			iso639code: "fr",
			dateFormat: euDateFormat,
			printer:    message.NewPrinter(language.MustParse("fr-CH")),
		},
		"en-US": { // United States
			name:       "English",
			iso639code: "en",
			dateFormat: usDateFormat,
			printer:    message.NewPrinter(language.MustParse("en-US")),
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

func GetLanguageChoicesISO639() map[string]string {
	out := make(map[string]string)
	for key, loc := range locales {
		out[key] = loc.iso639code
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
	// Looks like the go x/text package doesn't support date formatting
	// So we do it ourselves.
	formatted := date.Format(l.dateFormat)

	// Now replace the month name with the localized name.
	switch date.Month() {
	case time.January:
		formatted = strings.ReplaceAll(formatted, time.January.String(), l.Sprintf("January"))
	case time.February:
		formatted = strings.ReplaceAll(formatted, time.February.String(), l.Sprintf("February"))
	case time.March:
		formatted = strings.ReplaceAll(formatted, time.March.String(), l.Sprintf("March"))
	case time.April:
		formatted = strings.ReplaceAll(formatted, time.April.String(), l.Sprintf("April"))
	case time.May:
		formatted = strings.ReplaceAll(formatted, time.May.String(), l.Sprintf("May"))
	case time.June:
		formatted = strings.ReplaceAll(formatted, time.June.String(), l.Sprintf("June"))
	case time.July:
		formatted = strings.ReplaceAll(formatted, time.July.String(), l.Sprintf("July"))
	case time.August:
		formatted = strings.ReplaceAll(formatted, time.August.String(), l.Sprintf("August"))
	case time.September:
		formatted = strings.ReplaceAll(formatted, time.September.String(), l.Sprintf("September"))
	case time.October:
		formatted = strings.ReplaceAll(formatted, time.October.String(), l.Sprintf("October"))
	case time.November:
		formatted = strings.ReplaceAll(formatted, time.November.String(), l.Sprintf("November"))
	case time.December:
		formatted = strings.ReplaceAll(formatted, time.December.String(), l.Sprintf("December"))
	}

	return formatted
}
