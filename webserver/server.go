package webserver

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
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

const layoutIsoFull = "2006-01-02 15:04:05"

// StartServer provides a basic web server for handling 'add to calendar'
// and ICS feeds
func StartServer(salt string) {
	saltValue = salt

	r := mux.NewRouter()

	// This will serve files under http://localhost:8000/static/<filename>
	r.HandleFunc("/add/{key}.html", addToCalendarHandler)
	r.HandleFunc("/feed/{user}/{key}/plans.ics", generateCalFeed)
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./html"))))

	srv := &http.Server{
		Handler: r,

		Addr: "127.0.0.1:8000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func errorMessage(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusInternalServerError)
	// TODO: nicer message
	w.Write([]byte(msg))
}

func addToCalendarHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// Find this event.
	event, loc, err := dbHelper.GetEventByHash(vars["key"], saltValue)
	if err != nil {
		errorMessage(w, "Event not found")
		return
	}

	// Make the template page.
	tmpl := ADDTOCALENDAR_PAGE
	tmpl = strings.ReplaceAll(tmpl, "%TITLE%", loc.Sprintf("Add to Calendar"))
	tmpl = strings.ReplaceAll(tmpl, "%DIDYOUKNOW%", loc.Sprintf("Did you know?"))
	tmpl = strings.ReplaceAll(tmpl, "%FEEDMSG%", loc.Sprintf(`The Furry Plans Bot provides an iCal feed of all events that you've marked as 'Yes' or 'Maybe'.  
You can add this feed to Google Calendar or other calendars, and events will appear automatically! 
To get the feed URL, chat with @furryplansbot and send the command <b>/feed</b>.`))

	tmpl = strings.ReplaceAll(tmpl, "%INSTRUCTION%", loc.Sprintf("Add '%v' to your Calendar", event.Name))
	tmpl = strings.ReplaceAll(tmpl, "%DATESTART%", event.DateTime.Time.Format(layoutIsoFull))
	tmpl = strings.ReplaceAll(tmpl, "%DATEEND%", event.DateTime.Time.Add(time.Hour).Format(layoutIsoFull)) // Just make it one hour long always
	tmpl = strings.ReplaceAll(tmpl, "%TIMEZONE%", event.TimeZone)
	tmpl = strings.ReplaceAll(tmpl, "%EVENTNAME%", event.Name)
	tmpl = strings.ReplaceAll(tmpl, "%NOTES%", event.Notes)
	tmpl = strings.ReplaceAll(tmpl, "%HOST%", event.OwnerName)
	tmpl = strings.ReplaceAll(tmpl, "%LOCATION%", event.Location)

	w.Write([]byte(tmpl))
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
	events, err := dbHelper.CalendarFeed(int64(owner))

	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodRequest)
	cal.SetName("Furry Plans Attending Events")
	cal.SetDescription("Furry Plans Calendar")
	for _, ev := range events {
		event := cal.AddEvent(fmt.Sprintf("furryplans%v-plans@telegram.com", ev.EventID))
		event.SetCreatedTime(time.Now())
		event.SetDtStampTime(time.Now())
		event.SetModifiedAt(time.Now())
		event.SetStartAt(ev.DateTime.Time)
		event.SetEndAt(ev.DateTime.Time.Add(time.Hour))
		event.SetSummary(helpers.StripHtmlRegex(ev.Name))
		event.SetLocation(helpers.StripHtmlRegex(ev.Location))
		event.SetDescription(helpers.StripHtmlRegex(ev.Notes))
		event.SetOrganizer(helpers.StripHtmlRegex(ev.OwnerName))
	}
	//w.Header().Add("Content-Type", "text/calendar")
	w.Write([]byte(cal.Serialize()))
}
