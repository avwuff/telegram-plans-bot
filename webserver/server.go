package webserver

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	ics "github.com/arran4/golang-ical"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var saltValue string
var db dbInterface.DBFeatures

const layoutIsoFull = "2006-01-02 15:04:05"

// StartServer provides a basic web server for handling 'add to calendar'
// and ICS feeds

func StartServer(salt string, useDb dbInterface.DBFeatures) {
	saltValue = salt
	db = useDb

	r := mux.NewRouter()

	// This will serve files under http://localhost:8000/static/<filename>
	r.HandleFunc("/add/{key}.html", addToCalendarHandler)
	r.HandleFunc("/guests/nameguests.html", nameGuestsHandler)
	r.HandleFunc("/feed/nearby/{lat}/{lon}/plans.ics", generateNearbyFeed)
	r.HandleFunc("/feed/{user}/{key}/plans.ics", generateCalFeed)
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./html"))))

	srv := &http.Server{
		Handler: logging(log.Default())(r),
		Addr:    ":8080",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Starting listener on", srv.Addr)

	log.Fatal(srv.ListenAndServe())
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

	tmpl = strings.ReplaceAll(tmpl, "%INSTRUCTION%", loc.Sprintf("Add '%v' to your Calendar", event.Name()))
	tmpl = strings.ReplaceAll(tmpl, "%DATESTART%", event.DateTime().Format(layoutIsoFull))
	tmpl = strings.ReplaceAll(tmpl, "%DATEEND%", event.DateTime().Add(time.Hour).Format(layoutIsoFull)) // Just make it one hour long always
	tmpl = strings.ReplaceAll(tmpl, "%TIMEZONE%", event.TimeZone())
	tmpl = strings.ReplaceAll(tmpl, "%EVENTNAME%", event.Name())
	tmpl = strings.ReplaceAll(tmpl, "%NOTES%", event.Notes())
	tmpl = strings.ReplaceAll(tmpl, "%HOST%", event.OwnerName())
	tmpl = strings.ReplaceAll(tmpl, "%LOCATION%", event.Location())

	w.Write([]byte(tmpl))
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
		event.SetOrganizer(helpers.StripHtmlRegex(ev.OwnerName()))
	}
	w.Header().Add("Content-Type", "text/calendar")
	w.Write([]byte(cal.Serialize()))
}

func icsTz(start ics.ComponentProperty, dateTime time.Time) ics.ComponentProperty {
	// DTEND;TZID=Asia/Shanghai:20170324T213000
	return ics.ComponentProperty(fmt.Sprintf("%v;TZID=%v", start, dateTime.Location().String()))
}

var IcalTimestampFormatTz = "20060102T150405"

func SetTimeZone(calendar *ics.Calendar, location string) {
	// download current time zone info from tzurl
	// TODO: replace with a better source
	url := fmt.Sprintf("https://www.tzurl.org/zoneinfo-outlook/%s", location)
	res, err := http.Get(url)
	if err != nil {
		fmt.Printf("Timezone %v not found", location)
		return
	}
	tzCal, err := ics.ParseCalendar(res.Body)
	if err != nil {
		fmt.Printf("Error parsing time zone %v: %v", location, err)
		return
	}
	timeZones := tzCal.Timezones()
	for _, tz := range timeZones {
		calendar.AddVTimezone(tz)
	}
}
