package webserver

import (
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_organizerCantAddToTheCalendar(t *testing.T) {
	transport := http.DefaultTransport
	http.DefaultTransport = emptyTimeZones{}
	t.Cleanup(func() { http.DefaultTransport = transport })

	for name, owner := range map[string]string{
		"newline":         "bob\nEND:VEVENT\nBEGIN:VEVENT\nUID:injected\nSUMMARY:Injected",
		"carriage return": "bob\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:injected\r\nSUMMARY:Injected",
		"entity":          "bob&#10;END:VEVENT&#10;BEGIN:VEVENT&#10;UID:injected&#10;SUMMARY:Injected",
	} {
		t.Run(name, func(t *testing.T) {
			ev := dbInterface.NewDBEventMock(t)
			ev.On("ID").Return(uint(1)).Maybe()
			ev.On("DateTime").Return(time.Date(2026, 12, 1, 19, 0, 0, 0, time.UTC)).Maybe()
			ev.On("EndDateTime").Return(time.Time{}).Maybe()
			ev.On("Name").Return("Test Event").Maybe()
			ev.On("Location").Return("My house").Maybe()
			ev.On("Notes").Return("").Maybe()
			ev.On("OwnerName").Return(owner).Maybe()
			ev.On("GetCanAttend").Return(dbInterface.CANATTEND_YES).Maybe()

			rec := httptest.NewRecorder()
			addEvents(rec, []dbInterface.DBEvent{ev}, "Test")

			// Whatever is left of the text has to stay on the organizer's line.
			assert.Equal(t, 1, strings.Count(rec.Body.String(), "\nBEGIN:VEVENT"))
			assert.NotContains(t, rec.Body.String(), "\nUID:injected")
		})
	}
}

// countingTimeZones counts time zone downloads, and whether their responses were closed.
type countingTimeZones struct {
	lock   sync.Mutex
	status int
	err    error
	calls  int
	closed int
}

func (c *countingTimeZones) RoundTrip(*http.Request) (*http.Response, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return &http.Response{StatusCode: c.status, Body: &closeCounter{
		Reader: strings.NewReader("BEGIN:VCALENDAR\r\nBEGIN:VTIMEZONE\r\nTZID:Test/Zone\r\nEND:VTIMEZONE\r\nEND:VCALENDAR\r\n"),
		parent: c,
	}}, nil
}

type closeCounter struct {
	io.Reader
	parent *countingTimeZones
}

func (c *closeCounter) Close() error {
	c.parent.lock.Lock()
	defer c.parent.lock.Unlock()
	c.parent.closed++
	return nil
}

func useTimeZones(t *testing.T, stub http.RoundTripper) {
	transport := http.DefaultTransport
	http.DefaultTransport = stub
	tzCache = map[string]cachedTimeZone{}
	t.Cleanup(func() {
		http.DefaultTransport = transport
		tzCache = map[string]cachedTimeZone{}
	})
}

func Test_timeZoneInfo(t *testing.T) {
	t.Run("downloads have a time limit", func(t *testing.T) {
		assert.NotZero(t, tzClient.Timeout)
	})

	t.Run("downloaded once and closed", func(t *testing.T) {
		stub := &countingTimeZones{status: http.StatusOK}
		useTimeZones(t, stub)

		assert.Len(t, timeZoneInfo("Test/Zone"), 1)
		assert.Len(t, timeZoneInfo("Test/Zone"), 1)

		assert.Equal(t, 1, stub.calls)
		assert.Equal(t, 1, stub.closed)
	})

	t.Run("each zone is separate", func(t *testing.T) {
		stub := &countingTimeZones{status: http.StatusOK}
		useTimeZones(t, stub)

		timeZoneInfo("Test/Zone")
		timeZoneInfo("Test/Other")

		assert.Equal(t, 2, stub.calls)
	})

	t.Run("downloaded again when out of date", func(t *testing.T) {
		stub := &countingTimeZones{status: http.StatusOK}
		useTimeZones(t, stub)

		timeZoneInfo("Test/Zone")
		tzCache["Test/Zone"] = cachedTimeZone{zones: tzCache["Test/Zone"].zones, nextFetch: time.Now().Add(-time.Minute)}
		timeZoneInfo("Test/Zone")

		assert.Equal(t, 2, stub.calls)
	})

	t.Run("failed downloads aren't repeated on every request", func(t *testing.T) {
		stub := &countingTimeZones{err: errors.New("no network")}
		useTimeZones(t, stub)

		assert.Empty(t, timeZoneInfo("Test/Zone"))
		assert.Empty(t, timeZoneInfo("Test/Zone"))

		assert.Equal(t, 1, stub.calls)
		assert.WithinDuration(t, time.Now().Add(tzRetryTime), tzCache["Test/Zone"].nextFetch, time.Minute)
	})

	t.Run("error page is closed and not used", func(t *testing.T) {
		stub := &countingTimeZones{status: http.StatusInternalServerError}
		useTimeZones(t, stub)

		assert.Empty(t, timeZoneInfo("Test/Zone"))

		assert.Equal(t, 1, stub.closed)
	})

	t.Run("keeps the old copy when a download fails", func(t *testing.T) {
		stub := &countingTimeZones{status: http.StatusOK}
		useTimeZones(t, stub)

		timeZoneInfo("Test/Zone")
		tzCache["Test/Zone"] = cachedTimeZone{zones: tzCache["Test/Zone"].zones, nextFetch: time.Now().Add(-time.Minute)}
		stub.err = errors.New("no network")

		assert.Len(t, timeZoneInfo("Test/Zone"), 1)
	})
}

func Test_limitConcurrent(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := limitConcurrent(1, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec
	}

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		get()
	}()
	<-started

	rec := get()
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	close(release)
	<-finished

	// The slot is free again. The handler would panic on a second close, so use a new one that shares nothing.
	free := limitConcurrent(1, func(w http.ResponseWriter, r *http.Request) {})
	rec = httptest.NewRecorder()
	free(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_limitConcurrent_freesItsSlot(t *testing.T) {
	calls := 0
	handler := limitConcurrent(1, func(w http.ResponseWriter, r *http.Request) { calls++ })

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}
	assert.Equal(t, 3, calls)
}

// useAddEvent sets up an event for the 'add to calendar' page and file. It was saved before links were escaped.
func useAddEvent(t *testing.T, key string, end time.Time) {
	localizer.InitLang()
	zone := time.FixedZone("Test", -8*60*60)

	ev := dbInterface.NewDBEventMock(t)
	ev.On("ID").Return(uint(1200)).Maybe()
	ev.On("DateTime").Return(time.Date(2026, 12, 1, 19, 0, 0, 0, zone))
	ev.On("EndDateTime").Return(end)
	ev.On("Name").Return(`Party &amp; <b>games</b>`)
	ev.On("Notes").Return(`<a href="https://a.example/"><script>alert(1)</script>">click</a> &lt;img src=x onerror=alert(2)&gt;`)
	ev.On("OwnerName").Return(`@bob`).Maybe()
	ev.On("Location").Return(`<a href="x" onmouseover="alert(3)">My house</a>`)

	useDb := dbInterface.NewDBFeaturesMock(t)
	useDb.On("GetEventByHash", key, mock.Anything, false).Return(ev, localizer.FromLanguage("en-US"), nil)
	db = useDb
}

// linkOnPage finds the link that starts with this address, and returns its settings.
func linkOnPage(t *testing.T, body string, prefix string) url.Values {
	start := strings.Index(body, `href="`+prefix)
	require.NotEqual(t, -1, start, prefix)
	link, _, found := strings.Cut(body[start+len(`href="`):], `"`)
	require.True(t, found)
	parsed, err := url.Parse(html.UnescapeString(link))
	require.NoError(t, err)
	assert.NotContains(t, parsed.RawQuery, "+")
	return parsed.Query()
}

func Test_addToCalendarPage(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	useAddEvent(t, key, time.Time{})

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/add/"+key+".html", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	t.Run("event text can't add to the page", func(t *testing.T) {
		assert.NotContains(t, body, "alert(1)</script>")
		assert.NotContains(t, body, "<img")
		assert.NotContains(t, body, "onmouseover=")
		assert.Contains(t, body, "Party &amp; games")
	})

	t.Run("nothing is loaded from anywhere else", func(t *testing.T) {
		assert.NotContains(t, body, "<script")
		assert.NotContains(t, body, "addtocalendar.com")
		assert.NotContains(t, body, "googleapis")
		assert.Contains(t, body, `href="`+key+`.ics"`)
	})

	t.Run("google link", func(t *testing.T) {
		link := linkOnPage(t, body, "https://calendar.google.com/")
		assert.Equal(t, "Party & games", link.Get("text"))
		assert.Equal(t, "My house", link.Get("location"))
		assert.Contains(t, link.Get("details"), "<img src=x onerror=alert(2)>")
		// 19:00 at UTC-8, and an hour long.
		assert.Equal(t, "20261202T030000Z/20261202T040000Z", link.Get("dates"))
	})

	for name, prefix := range map[string]string{
		"outlook":   "https://outlook.live.com/",
		"office365": "https://outlook.office.com/",
	} {
		t.Run(name+" link", func(t *testing.T) {
			link := linkOnPage(t, body, prefix)
			assert.Equal(t, "addevent", link.Get("rru"))
			assert.Equal(t, "Party & games", link.Get("subject"))
			assert.Equal(t, "2026-12-02T03:00:00Z", link.Get("startdt"))
			assert.Equal(t, "2026-12-02T04:00:00Z", link.Get("enddt"))
		})
	}

	t.Run("yahoo link", func(t *testing.T) {
		link := linkOnPage(t, body, "https://calendar.yahoo.com/")
		assert.Equal(t, "Party & games", link.Get("title"))
		assert.Equal(t, "20261202T030000Z", link.Get("st"))
		assert.Equal(t, "20261202T040000Z", link.Get("et"))
	})
}

func Test_addToCalendarPage_usesTheEndTime(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	useAddEvent(t, key, time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC))

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/add/"+key+".html", nil))

	link := linkOnPage(t, rec.Body.String(), "https://calendar.google.com/")
	assert.Equal(t, "20261202T030000Z/20261203T120000Z", link.Get("dates"))
}

func Test_shortenForLink(t *testing.T) {
	assert.Equal(t, "short", shortenForLink("short"))

	exact := strings.Repeat("é", maxLinkNotes)
	assert.Equal(t, exact, shortenForLink(exact))

	// Cut by characters, so that it doesn't end in the middle of one.
	assert.Equal(t, exact+"…", shortenForLink(exact+"more"))
}

func Test_eventFile(t *testing.T) {
	// Time zones are downloaded for the feeds, but the file shouldn't need them.
	stub := &countingTimeZones{status: http.StatusOK}
	useTimeZones(t, stub)

	const key = "0123456789abcdef0123456789abcdef"
	useAddEvent(t, key, time.Time{})

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/add/"+key+".ics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Equal(t, "text/calendar; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, 1, strings.Count(body, "BEGIN:VEVENT"))
	assert.Contains(t, body, "METHOD:PUBLISH")
	assert.Contains(t, body, "DTSTART:20261202T030000Z")
	assert.Contains(t, body, "DTEND:20261202T040000Z")
	assert.Contains(t, body, "SUMMARY:Party & games")
	assert.Contains(t, body, "LOCATION:My house")
	assert.Zero(t, stub.calls)
}

func Test_eventFile_notFound(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	useDb := dbInterface.NewDBFeaturesMock(t)
	useDb.On("GetEventByHash", key, mock.Anything, false).Return(nil, nil, errors.New("event not found"))
	db = useDb

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/add/"+key+".ics", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func Test_addToCalendarPage_badKeysDontReachTheDatabase(t *testing.T) {
	db = dbInterface.NewDBFeaturesMock(t)

	for _, key := range []string{"short", "0123456789abcdef0123456789abcdeg", "0123456789abcdef0123456789abcdef0"} {
		for _, ext := range []string{".html", ".ics"} {
			rec := httptest.NewRecorder()
			newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/add/"+key+ext, nil))
			assert.Equal(t, http.StatusNotFound, rec.Code, key+ext)
		}
	}
}
