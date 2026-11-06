package webserver

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// getFromServer requests a path from the router, in a folder with the static files plus files that shouldn't be served.
func getFromServer(t *testing.T, path string) *httptest.ResponseRecorder {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "html", "cache"), 0777))
	for _, file := range staticFiles {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "html", file), []byte("static "+file), 0666))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "html", "cache", "picture.jpg"), []byte("picture"), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "html", "other.txt"), []byte("other"), 0666))
	t.Chdir(dir)

	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func Test_staticFiles(t *testing.T) {
	for urlPath, file := range staticFiles {
		t.Run("serves "+urlPath, func(t *testing.T) {
			rec := getFromServer(t, urlPath)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "static "+file, rec.Body.String())
		})
	}
}

func Test_otherFilesAreNotServed(t *testing.T) {
	for _, urlPath := range []string{
		"/cache/picture.jpg",
		"/cache/",
		"/cache",
		"/other.txt",
		"/html/cache/picture.jpg",
	} {
		t.Run(urlPath, func(t *testing.T) {
			rec := getFromServer(t, urlPath)

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.NotContains(t, rec.Body.String(), "picture")
		})
	}
}

// emptyTimeZones answers the time zone lookups, so that the tests don't use the network.
type emptyTimeZones struct{}

func (emptyTimeZones) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")),
	}, nil
}

func Test_eventStatus(t *testing.T) {
	transport := http.DefaultTransport
	http.DefaultTransport = emptyTimeZones{}
	t.Cleanup(func() { http.DefaultTransport = transport })

	feedFor := func(t *testing.T, canAttend dbInterface.CanAttend) string {
		ev := dbInterface.NewDBEventMock(t)
		ev.On("ID").Return(uint(1200)).Maybe()
		ev.On("DateTime").Return(time.Date(2026, 12, 1, 19, 0, 0, 0, time.UTC)).Maybe()
		ev.On("EndDateTime").Return(time.Time{}).Maybe()
		ev.On("Name").Return("Test Event").Maybe()
		ev.On("Location").Return("My house").Maybe()
		ev.On("Notes").Return("").Maybe()
		ev.On("OwnerName").Return("bob").Maybe()
		ev.On("GetCanAttend").Return(canAttend).Maybe()

		rec := httptest.NewRecorder()
		addEvents(rec, []dbInterface.DBEvent{ev}, "Test")
		return rec.Body.String()
	}

	t.Run("maybe is tentative", func(t *testing.T) {
		assert.Contains(t, feedFor(t, dbInterface.CANATTEND_MAYBE), "STATUS:TENTATIVE")
	})

	// CANATTEND_NO is also what the events in the nearby feed have, since it isn't about one person.
	for _, canAttend := range []dbInterface.CanAttend{
		dbInterface.CANATTEND_YES, dbInterface.CANATTEND_SUITING, dbInterface.CANATTEND_PHOTOGRAPHER, dbInterface.CANATTEND_NO,
	} {
		t.Run(fmt.Sprintf("no status for %v", canAttend), func(t *testing.T) {
			assert.NotContains(t, feedFor(t, canAttend), "STATUS:")
		})
	}
}

func Test_calendarFeeds(t *testing.T) {
	useDb := dbInterface.NewDBFeaturesMock(t)
	useDb.On("CalendarFeed", int64(1234)).Return([]dbInterface.DBEvent{}, nil).Maybe()
	useDb.On("NearbyFeed", mock.Anything, mock.Anything, mock.Anything).Return([]dbInterface.DBEvent{}, nil).Maybe()
	db = useDb
	saltValue = "salt"

	t.Run("user feed", func(t *testing.T) {
		rec := getFromServer(t, "/feed/1234/"+helpers.CalenFeedMD5("salt", 1234)+"/plans.ics")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/calendar; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Body.String(), "BEGIN:VCALENDAR")
		assert.Contains(t, rec.Body.String(), "REFRESH-INTERVAL;VALUE=DURATION:PT1H")
		assert.Contains(t, rec.Body.String(), "X-PUBLISHED-TTL:PT1H")
	})

	t.Run("user feed needs the right key", func(t *testing.T) {
		rec := getFromServer(t, "/feed/1234/wrong/plans.ics")

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("nearby feed", func(t *testing.T) {
		rec := getFromServer(t, "/feed/nearby/45.5/-122.6/plans.ics")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "BEGIN:VCALENDAR")
	})
}
