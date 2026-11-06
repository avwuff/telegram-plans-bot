package webserver

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	ics "github.com/arran4/golang-ical"
	"github.com/gorilla/mux"
	"html"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var saltValue string
var db dbInterface.DBFeatures

const layoutIsoFull = "2006-01-02 15:04:05"

// feedRefreshInterval is how often calendar apps should check a feed for changes, as an iCalendar duration.
const feedRefreshInterval = "PT1H"

// How many requests can run at once on the pages that do the most work. Each route has its own limit, so that
// a flood of one can't block the others. These don't rely on the client's address, which could be a shared proxy.
const (
	// Finding an event by its hash has to check every event in the database.
	maxAddPageRequests = 4
	maxNearbyRequests  = 8
	// Calendar services can fetch a lot of feeds at the same time.
	maxUserFeedRequests = 32
)

const (
	tzFetchTimeout = 5 * time.Second
	tzMaxSize      = 1 << 20
	// Time zones rarely change, so a downloaded one can be used for a day.
	tzCacheTime = 24 * time.Hour
	// After a failed download, wait a bit before trying again so that feeds aren't slowed down by every attempt.
	tzRetryTime = 5 * time.Minute
)

// cachedTimeZone is the downloaded information about a time zone.
type cachedTimeZone struct {
	zones     []*ics.VTimezone
	nextFetch time.Time
}

var (
	tzClient    = &http.Client{Timeout: tzFetchTimeout}
	tzCacheLock sync.Mutex
	tzCache     = map[string]cachedTimeZone{}
)

// staticFiles are the only files served from the html folder. The stylesheets are used by the 'add to calendar' page.
var staticFiles = map[string]string{
	"/":                   "index.html",
	"/robots.txt":         "robots.txt",
	"/normalize.css":      "normalize.css",
	"/skeleton.css":       "skeleton.css",
	"/atc-style-blue.css": "atc-style-blue.css",
}

// StartServer provides a basic web server for handling 'add to calendar'
// and ICS feeds

func StartServer(salt string, useDb dbInterface.DBFeatures) {
	saltValue = salt
	db = useDb

	srv := &http.Server{
		Handler: logging(log.Default())(newRouter()),
		Addr:    ":8080",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Starting listener on", srv.Addr)

	log.Fatal(srv.ListenAndServe())
}

// newRouter sets up the pages and feeds of the web server.
func newRouter() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/add/{key:[0-9a-fA-F]{32}}.html", limitConcurrent(maxAddPageRequests, addToCalendarHandler))
	r.HandleFunc("/guests/nameguests.html", nameGuestsHandler)
	r.HandleFunc("/feed/nearby/{lat}/{lon}/plans.ics", limitConcurrent(maxNearbyRequests, generateNearbyFeed))
	r.HandleFunc("/feed/{user}/{key}/plans.ics", limitConcurrent(maxUserFeedRequests, generateCalFeed))

	for urlPath, file := range staticFiles {
		r.HandleFunc(urlPath, func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, filepath.Join("html", file))
		})
	}
	return r
}

// limitConcurrent turns away requests when too many are already running, and asks them to try again later.
func limitConcurrent(limit int, next http.HandlerFunc) http.HandlerFunc {
	slots := make(chan struct{}, limit)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next(w, r)
		default:
			w.Header().Set("Retry-After", "30")
			http.Error(w, "Server is busy, try again soon", http.StatusServiceUnavailable)
		}
	}
}

func errorMessage(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusInternalServerError)
	// TODO: nicer message
	w.Write([]byte(msg))
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {

				logger.Println(r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func addToCalendarHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// Find this event.
	event, loc, err := db.GetEventByHash(vars["key"], saltValue, false)
	if err != nil {
		errorMessage(w, "Event not found")
		return
	}

	// TODO: Use a file template instead of this
	// TODO: Copy the JS file from addtocalendar.com
	// Make the template page.
	tmpl := ADDTOCALENDAR_PAGE
	tmpl = strings.ReplaceAll(tmpl, "%TITLE%", loc.Sprintf("Add to Calendar"))
	tmpl = strings.ReplaceAll(tmpl, "%DIDYOUKNOW%", loc.Sprintf("Did you know?"))
	tmpl = strings.ReplaceAll(tmpl, "%FEEDMSG%", loc.Sprintf(`The Furry Plans Bot provides an iCal feed of all events that you've marked as 'Yes' or 'Maybe'.  
You can add this feed to Google Calendar or other calendars, and events will appear automatically! 
To get the feed URL, chat with @furryplansbot and send the command <b>/feed</b>.`))

	tmpl = strings.ReplaceAll(tmpl, "%INSTRUCTION%", loc.Sprintf("Add '%v' to your Calendar", pageText(event.Name())))
	tmpl = strings.ReplaceAll(tmpl, "%DATESTART%", event.DateTime().Format(layoutIsoFull))
	tmpl = strings.ReplaceAll(tmpl, "%DATEEND%", event.DateTime().Add(time.Hour).Format(layoutIsoFull)) // Just make it one hour long always
	tmpl = strings.ReplaceAll(tmpl, "%TIMEZONE%", html.EscapeString(event.TimeZone()))
	tmpl = strings.ReplaceAll(tmpl, "%EVENTNAME%", pageText(event.Name()))
	tmpl = strings.ReplaceAll(tmpl, "%NOTES%", pageText(event.Notes()))
	tmpl = strings.ReplaceAll(tmpl, "%HOST%", pageText(event.OwnerName()))
	tmpl = strings.ReplaceAll(tmpl, "%LOCATION%", pageText(event.Location()))

	w.Write([]byte(tmpl))
}

// pageText makes text from an event safe to put on a web page. Events are stored as Telegram HTML, which
// came from the user and might have been saved before links were escaped, so it is turned into plain text.
func pageText(s string) string {
	return html.EscapeString(helpers.StripHtmlRegex(s))
}

func nameGuestsHandler(w http.ResponseWriter, r *http.Request) {
	//vars := mux.Vars(r)
	// TODO: Use a file template instead of this
	// Make the template page.
	tmpl := NAME_GUESTS_PAGE

	w.Write([]byte(tmpl))
}

// Create a feed of nearby events
func generateNearbyFeed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lat := vars["lat"]
	lon := vars["lon"]
	latitude, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		errorMessage(w, "invalid coords")
		return
	}
	longitude, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		errorMessage(w, "invalid coords")
		return
	}

	// Get the list of events this owner has decided to go to.
	events, err := db.NearbyFeed(latitude, longitude, 800)
	addEvents(w, events, "Furry Plans Nearby Events")
}

// Create an ICS calendar feed for this user.
func generateCalFeed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ownerId := vars["user"]
	owner, err := strconv.Atoi(ownerId)
	if err != nil {
		errorMessage(w, "invalid owner")
		return
	}
	ownerHash := vars["key"]

	if helpers.CalenFeedMD5(saltValue, int64(owner)) != ownerHash {
		errorMessage(w, "Owner not found")
		return
	}

	// Get the list of events this owner has decided to go to.
	events, err := db.CalendarFeed(int64(owner))

	addEvents(w, events, "Furry Plans Attending Events")
}

func addEvents(w http.ResponseWriter, events []dbInterface.DBEvent, name string) {

	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)
	cal.SetName(name)
	cal.SetDescription("Furry Plans Calendar")

	// Ask calendar apps to check for changes every hour. Not all of them follow this.
	cal.SetRefreshInterval(feedRefreshInterval)
	cal.SetXPublishedTTL(feedRefreshInterval)

	// make a list of the time zones in use in the event
	tzList := map[string]bool{}
	for _, ev := range events {
		tzList[ev.DateTime().Location().String()] = true
	}

	// Grab time zone info for each time zone in the list and add it to the calendar
	for tz := range tzList {
		SetTimeZone(cal, tz)
	}

	for _, ev := range events {
		event := cal.AddEvent(fmt.Sprintf("furryplans%v-plans@telegram.com", ev.ID()))
		event.SetCreatedTime(time.Now())
		event.SetDtStampTime(time.Now())
		event.SetModifiedAt(time.Now())
		//event.SetStartAt(ev.DateTime())

		event.SetProperty(icsTz(ics.ComponentPropertyDtStart, ev.DateTime()), ev.DateTime().Format(IcalTimestampFormatTz))

		// Proper support for multi-day events and durations
		endsAt := ev.DateTime().Add(time.Hour)
		if !ev.EndDateTime().IsZero() && ev.EndDateTime() != ev.DateTime() {
			endsAt = ev.EndDateTime()
		}

		//event.SetEndAt(endsAt)
		event.SetProperty(icsTz(ics.ComponentPropertyDtEnd, endsAt), endsAt.Format(IcalTimestampFormatTz))
		event.SetSummary(helpers.StripHtmlRegex(ev.Name()))
		event.SetLocation(helpers.StripHtmlRegex(ev.Location()))
		event.SetDescription(helpers.StripHtmlRegex(ev.Notes()))
		event.SetOrganizer(icsSingleLine(helpers.StripHtmlRegex(ev.OwnerName())))

		if ev.GetCanAttend() == dbInterface.CANATTEND_MAYBE {
			event.SetStatus(ics.ObjectStatusTentative)
		}
	}
	w.Header().Add("Content-Type", "text/calendar; charset=utf-8")
	w.Write([]byte(cal.Serialize()))
}

// icsSingleLine removes line breaks. Unlike the other event fields, the calendar library doesn't escape the
// organizer, so a line break in the host's name would let them add anything to the calendar.
func icsSingleLine(s string) string {
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
}

func icsTz(start ics.ComponentProperty, dateTime time.Time) ics.ComponentProperty {
	// DTEND;TZID=Asia/Shanghai:20170324T213000
	return ics.ComponentProperty(fmt.Sprintf("%v;TZID=%v", start, dateTime.Location().String()))
}

var IcalTimestampFormatTz = "20060102T150405"

func SetTimeZone(calendar *ics.Calendar, location string) {
	for _, tz := range timeZoneInfo(location) {
		calendar.AddVTimezone(tz)
	}
}

// timeZoneInfo gets the information about a time zone, only downloading it when the saved copy is out of date.
// If a download fails, the out of date copy is still better than nothing.
func timeZoneInfo(location string) []*ics.VTimezone {
	tzCacheLock.Lock()
	cached, found := tzCache[location]
	tzCacheLock.Unlock()

	if found && time.Now().Before(cached.nextFetch) {
		return cached.zones
	}

	zones, err := fetchTimeZone(location)
	if err != nil {
		log.Printf("Unable to get time zone %v: %v\n", location, err)
		cached.nextFetch = time.Now().Add(tzRetryTime)
	} else {
		cached = cachedTimeZone{zones: zones, nextFetch: time.Now().Add(tzCacheTime)}
	}

	tzCacheLock.Lock()
	tzCache[location] = cached
	tzCacheLock.Unlock()

	return cached.zones
}

// fetchTimeZone downloads current time zone info from tzurl
// TODO: replace with a better source
func fetchTimeZone(location string) ([]*ics.VTimezone, error) {
	res, err := tzClient.Get(fmt.Sprintf("https://www.tzurl.org/zoneinfo-outlook/%s", location))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %v", res.StatusCode)
	}

	tzCal, err := ics.ParseCalendar(io.LimitReader(res.Body, tzMaxSize))
	if err != nil {
		return nil, err
	}
	return tzCal.Timezones(), nil
}
