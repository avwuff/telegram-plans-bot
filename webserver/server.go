package webserver

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	ics "github.com/arran4/golang-ical"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var saltValue string
var db dbInterface.DBFeatures

// Calendar links use UTC, so that the event is at the right moment no matter what time zone the person is in.
const (
	layoutCalendarLink = "20060102T150405Z"
	layoutOutlookLink  = "2006-01-02T15:04:05Z"
)

// maxLinkNotes is how much of an event's notes go in a calendar link, since very long links get rejected.
// The calendar file always has all of them.
const maxLinkNotes = 1000

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
	"/":              "index.html",
	"/robots.txt":    "robots.txt",
	"/normalize.css": "normalize.css",
	"/skeleton.css":  "skeleton.css",
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
	r.HandleFunc("/add/{key:[0-9a-fA-F]{32}}.ics", limitConcurrent(maxAddPageRequests, eventFileHandler))
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

	name := helpers.StripHtmlRegex(event.Name())
	location := helpers.StripHtmlRegex(event.Location())
	notes := shortenForLink(helpers.StripHtmlRegex(event.Notes()))
	start := event.DateTime().UTC()
	end := eventEnd(event).UTC()

	page := addToCalendarData{
		Title:       loc.Sprintf("Add to Calendar"),
		Instruction: loc.Sprintf("Add '%v' to your Calendar", name),
		DidYouKnow:  loc.Sprintf("Did you know?"),
		FeedMessage: template.HTML(loc.Sprintf(`The Furry Plans Bot provides an iCal feed of all events that you've marked as 'Yes' or 'Maybe'.
You can add this feed to Google Calendar or other calendars, and events will appear automatically!
To get the feed URL, chat with @furryplansbot and send the command <b>/feed</b>.`)),
		FileLabel: loc.Sprintf("Apple Calendar, Outlook app or other (.ics file)"),
		FileURL:   vars["key"] + ".ics",
		GoogleURL: calendarLink("https://calendar.google.com/calendar/render", url.Values{
			"action":   {"TEMPLATE"},
			"text":     {name},
			"dates":    {start.Format(layoutCalendarLink) + "/" + end.Format(layoutCalendarLink)},
			"details":  {notes},
			"location": {location},
		}),
		OutlookURL:   outlookLink("https://outlook.live.com", name, location, notes, start, end),
		Office365URL: outlookLink("https://outlook.office.com", name, location, notes, start, end),
		YahooURL: calendarLink("https://calendar.yahoo.com/", url.Values{
			"v":      {"60"},
			"title":  {name},
			"st":     {start.Format(layoutCalendarLink)},
			"et":     {end.Format(layoutCalendarLink)},
			"desc":   {notes},
			"in_loc": {location},
		}),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := addToCalendarTemplate.Execute(w, page); err != nil {
		log.Println("Unable to show the add to calendar page:", err)
	}
}

// addToCalendarData is what goes on the 'add to calendar' page. FeedMessage comes from the translations
// and not from any user, which is why it's allowed to have HTML in it.
type addToCalendarData struct {
	Title, Instruction, DidYouKnow string
	FeedMessage                    template.HTML
	FileLabel, FileURL             string
	GoogleURL, OutlookURL          string
	Office365URL, YahooURL         string
}

// calendarLink makes a link to a calendar service. Spaces are written as %20, because
// Outlook can show the + form as a literal plus sign.
func calendarLink(base string, params url.Values) string {
	return base + "?" + strings.ReplaceAll(params.Encode(), "+", "%20")
}

// outlookLink makes a link for the personal and work versions of Outlook, which only differ by their address.
func outlookLink(site, name, location, notes string, start, end time.Time) string {
	return calendarLink(site+"/calendar/0/deeplink/compose", url.Values{
		"path":     {"/calendar/action/compose"},
		"rru":      {"addevent"},
		"subject":  {name},
		"startdt":  {start.Format(layoutOutlookLink)},
		"enddt":    {end.Format(layoutOutlookLink)},
		"body":     {notes},
		"location": {location},
	})
}

// shortenForLink cuts text down to maxLinkNotes characters.
func shortenForLink(s string) string {
	chars := []rune(s)
	if len(chars) <= maxLinkNotes {
		return s
	}
	return strings.TrimSpace(string(chars[:maxLinkNotes])) + "…"
}

// eventFileHandler provides a single event as a calendar file. The times are in UTC, because a time zone would
// need its VTIMEZONE information to be in the file, and for one event nothing is lost by leaving it out.
func eventFileHandler(w http.ResponseWriter, r *http.Request) {
	event, _, err := db.GetEventByHash(mux.Vars(r)["key"], saltValue, false)
	if err != nil {
		errorMessage(w, "Event not found")
		return
	}

	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)

	calEvent := newCalendarEvent(cal, event)
	calEvent.SetStartAt(event.DateTime())
	calEvent.SetEndAt(eventEnd(event))

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="event.ics"`)
	w.Write([]byte(cal.Serialize()))
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
		event := newCalendarEvent(cal, ev)
		//event.SetStartAt(ev.DateTime())

		event.SetProperty(icsTz(ics.ComponentPropertyDtStart, ev.DateTime()), ev.DateTime().Format(IcalTimestampFormatTz))

		endsAt := eventEnd(ev)
		//event.SetEndAt(endsAt)
		event.SetProperty(icsTz(ics.ComponentPropertyDtEnd, endsAt), endsAt.Format(IcalTimestampFormatTz))

		if ev.GetCanAttend() == dbInterface.CANATTEND_MAYBE {
			event.SetStatus(ics.ObjectStatusTentative)
		}
	}
	w.Header().Add("Content-Type", "text/calendar; charset=utf-8")
	w.Write([]byte(cal.Serialize()))
}

// newCalendarEvent adds an event to the calendar with everything except its start and end times.
func newCalendarEvent(cal *ics.Calendar, ev dbInterface.DBEvent) *ics.VEvent {
	event := cal.AddEvent(fmt.Sprintf("furryplans%v-plans@telegram.com", ev.ID()))
	event.SetCreatedTime(time.Now())
	event.SetDtStampTime(time.Now())
	event.SetModifiedAt(time.Now())
	event.SetSummary(helpers.StripHtmlRegex(ev.Name()))
	event.SetLocation(helpers.StripHtmlRegex(ev.Location()))
	event.SetDescription(helpers.StripHtmlRegex(ev.Notes()))
	event.SetOrganizer(icsSingleLine(helpers.StripHtmlRegex(ev.OwnerName())))
	return event
}

// eventEnd is when an event finishes. Events without an end time are treated as being an hour long.
func eventEnd(ev dbInterface.DBEvent) time.Time {
	if !ev.EndDateTime().IsZero() && ev.EndDateTime() != ev.DateTime() {
		return ev.EndDateTime()
	}
	return ev.DateTime().Add(time.Hour)
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
