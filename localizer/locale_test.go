package localizer

import (
	"testing"
	"time"

	_ "furryplansbot.avbrand.com/internal/translations"
	"github.com/stretchr/testify/assert"
)

func TestFormatShortDate(t *testing.T) {
	InitLang()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	start := time.Date(2026, 11, 5, 14, 27, 0, 0, time.UTC)

	tests := []struct {
		name string
		lang string
		date time.Time
		want string
	}{
		{name: "this year", lang: "en-US", date: start, want: "Thu, Nov 5, 2:27 PM"},
		{name: "another year", lang: "en-US", date: start.AddDate(1, 0, 0), want: "Fri, Nov 5, 2027, 2:27 PM"},
		{name: "european layout", lang: "de-DE", date: start, want: "Do, 5. Nov, 14:27"},
		{name: "european layout, another year", lang: "de-DE", date: start.AddDate(1, 0, 0), want: "Fr, 5. Nov 2027, 14:27"},
		{name: "french layout", lang: "fr-CA", date: start, want: "jeu. 5 nov., 14 h 27"},
		{name: "translated weekday can match an English month", lang: "fr-CA", date: time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC), want: "mar. 3 mars, 09 h 00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FromLanguage(tt.lang).FormatShortDateForLocale(tt.date, now))
		})
	}

	t.Run("the year is checked in the time zone of the event", func(t *testing.T) {
		zone := time.FixedZone("behind", -8*60*60)
		date := time.Date(2026, 12, 31, 20, 0, 0, 0, zone)
		newYear := time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC) // Still 2026 for the event.

		assert.Equal(t, "Thu, Dec 31, 8:00 PM", FromLanguage("en-US").FormatShortDateForLocale(date, newYear))
	})
}

func TestFormatShortDateRange(t *testing.T) {
	InitLang()
	loc := FromLanguage("en-US")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	start := time.Date(2026, 11, 5, 19, 0, 0, 0, time.UTC)

	assert.Equal(t, "Thu, Nov 5, 7:00 PM", loc.FormatShortDateRangeForLocale(start, time.Time{}, now))
	assert.Equal(t, "Thu, Nov 5, 7:00 PM", loc.FormatShortDateRangeForLocale(start, start, now))
	assert.Equal(t, "Thu, Nov 5, 7:00 PM – 10:00 PM", loc.FormatShortDateRangeForLocale(start, start.Add(3*time.Hour), now))
	assert.Equal(t, "Thu, Nov 5, 7:00 PM – Fri, Nov 6, 1:00 AM", loc.FormatShortDateRangeForLocale(start, start.Add(6*time.Hour), now))
}
