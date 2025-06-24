package dbHelper

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"gorm.io/gorm"
	"time"
)

// Connector is an implementation of the dbinterface.
type Connector struct {
	db *gorm.DB
}

func (c *Connector) GetPrefs(userid int64) dbInterface.Prefs {
	var userPrefs UserPrefs
	if err := c.db.Where(&UserPrefs{UserID: userid}).First(&userPrefs).Error; err != nil {
		// DEFAULT user prefs
		return dbInterface.Prefs{
			Language: "en-US",
		}
	}

	return dbInterface.Prefs{
		SetupComplete: userPrefs.SetupComplete,
		Language:      userPrefs.Language,
		TimeZone:      userPrefs.TimeZone,
	}
}

func (c *Connector) SavePrefs(userid int64, prefs dbInterface.Prefs, colName string) error {
	p := &UserPrefs{
		UserID:        userid,
		SetupComplete: prefs.SetupComplete,
		Language:      prefs.Language,
		TimeZone:      prefs.TimeZone,
	}

	if c.db.Model(&UserPrefs{}).Where(&UserPrefs{UserID: userid}).Select(colName).Updates(&p).RowsAffected == 0 {
		return c.db.Create(&p).Error
	}
	return nil
}

func (c *Connector) CreateEvent(OwnerID int64, Name string, DateTime time.Time, TimeZone string, OwnerName string, Location string, Language string, Notes string) (uint, error) {
	event := FurryPlans{
		OwnerID:   fmt.Sprintf("%v", OwnerID),
		Name:      Name,
		DateTime:  sql.NullTime{Time: DateTime, Valid: true},
		TimeZone:  TimeZone,
		CreatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		OwnerName: OwnerName,
		Location:  Location,
		Notes:     Notes,
		Language:  Language, // By default, events pick up the language of their creators
		MaxGuests: 2,        // by default, we allow two guests
	}
	err := c.db.Create(&event).Error
	if err != nil {
		return 0, err
	}
	return event.EventID, nil
}

func (c *Connector) GetEvent(eventId uint, ownerId int64) (dbInterface.DBEvent, error) {
	var event FurryPlans

	query := c.db.Where(&FurryPlans{EventID: eventId})

	// Allow querying by owner ID
	if ownerId != -1 {
		query = query.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)})
	}

	if err := query.First(&event).Error; err != nil {
		return nil, fmt.Errorf("event not found")
	}

	// Clean up the old syntax from the previous event bot
	event.cleanOldSyntax()

	// Update the time on the event to match the time zone.
	if event.TimeZone != "" {
		tz := localizer.FromTimeZone(event.TimeZone)
		event.DateTime.Time = event.DateTime.Time.In(tz)
		if event.EndDateTime.Valid {
			event.EndDateTime.Time = event.EndDateTime.Time.In(tz)
		}
	}

	return c.wrap(&event), nil
}

// wrap takes a real database object and wraps it into a DBEvent
func (c *Connector) wrap(event *FurryPlans) dbInterface.DBEvent {
	return &eventConnector{
		db: c.db,
		ev: event,
	}
}

func (c *Connector) wrapWithAttend(event *FurryPlansWithAttend) dbInterface.DBEvent {
	return &eventConnector{
		db:        c.db,
		ev:        &event.FurryPlans,
		canAttend: dbInterface.CanAttend(event.CanAttend),
	}
}

// GetEventByHash searches for this event by the hash.
func (c *Connector) GetEventByHash(hash string, saltValue string, shareMode bool) (dbInterface.DBEvent, *localizer.Localizer, error) {

	// TODO: This whole sharing mechanism needs to be overhauled.
	// Shouldn't rely on a hardcoded salt, nor on MD5 any longer.
	var sql string

	// In share mode we only want to see an event from the last few days.
	if shareMode {
		sql = `SELECT * FROM furryplans WHERE 
            CONCAT('', MD5(CONCAT(eventID, ?))) = ? AND 
            (EventDateTime > NOW() - INTERVAL 2 DAY OR EndDateTime > NOW() - INTERVAL 2 DAY) AND 
            AllowShare=1`

	} else {
		sql = `SELECT * FROM furryplans WHERE 
            CONCAT('', MD5(CONCAT(eventID, ?))) = ?`
	}

	var event FurryPlans
	res := c.db.Raw(sql, saltValue, hash).Scan(&event)
	if res.Error != nil {
		return nil, nil, res.Error
	}

	if event.EventID == 0 {
		return nil, nil, fmt.Errorf("event not found")
	}

	// Clean up the old syntax from the previous event bot
	event.cleanOldSyntax()
	loc := localizer.FromLanguage(event.Language)

	// Update the time on the event to match the time zone.
	if event.TimeZone != "" {
		tz := localizer.FromTimeZone(event.TimeZone)
		event.DateTime.Time = event.DateTime.Time.In(tz)
	}
	return c.wrap(&event), loc, nil
}

func (c *Connector) CalendarFeed(ownerId int64) ([]dbInterface.DBEvent, error) {

	sql := fmt.Sprintf(`SELECT furryplans.*, furryplansattend.CanAttend FROM furryplansattend 
		LEFT JOIN furryplans USING (eventID) 
		WHERE furryplansattend.userid = ? 
		AND furryplansattend.CanAttend IN (%v, %v, %v, %v) 
		AND (furryplans.EventDateTime > NOW() - INTERVAL 7 DAY  OR furryplans.EndDateTime > NOW() - INTERVAL 2 DAY) 
		ORDER BY EventDateTime `, dbInterface.CANATTEND_YES, dbInterface.CANATTEND_MAYBE, dbInterface.CANATTEND_SUITING, dbInterface.CANATTEND_PHOTOGRAPHER)

	var events []*FurryPlansWithAttend
	res := c.db.Raw(sql, ownerId).Scan(&events)
	if res.Error != nil {
		return nil, res.Error
	}

	// Clean up the old syntax from the previous event bot
	list := make([]dbInterface.DBEvent, len(events))
	for i, event := range events {
		event.cleanOldSyntax()

		// Update the time on the event to match the time zone.
		if event.TimeZone != "" {
			tz := localizer.FromTimeZone(event.TimeZone)
			event.DateTime.Time = event.DateTime.Time.In(tz)
		}
		list[i] = c.wrapWithAttend(event)
	}

	return list, nil
}

func (c *Connector) SearchEvents(ownerId int64, searchText string) ([]dbInterface.DBEvent, error) {
	var events []*FurryPlans
	query := c.db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Where("EventName LIKE ?", "%"+searchText+"%").Order("EventDateTime DESC")
	// TODO: This query doesn't use time zones, it probably should.
	query = query.Where("EventDateTime > NOW() - INTERVAL 2 DAY OR EndDateTime > NOW() - INTERVAL 2 DAY")
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	list := make([]dbInterface.DBEvent, len(events))
	for i, event := range events {
		event.cleanOldSyntax()
		list[i] = c.wrap(event)
	}
	return list, nil
}

func (c *Connector) GetEvents(ownerId int64, includeOld bool) ([]dbInterface.DBEvent, error) {
	var events []*FurryPlans
	query := c.db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Order("EventDateTime DESC").Limit(100)
	if !includeOld {
		// TODO: This query doesn't use time zones, it probably should.
		query = query.Where("EventDateTime > NOW() - INTERVAL 2 DAY OR EndDateTime > NOW() - INTERVAL 2 DAY")
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	list := make([]dbInterface.DBEvent, len(events))
	for i, event := range events {
		event.cleanOldSyntax()
		list[i] = c.wrap(event)
	}
	return list, nil
}

func (c *Connector) NearbyFeed(Latitude float64, Longitude float64, distMiles int) ([]dbInterface.DBEvent, error) {

	sql := `SELECT *  
		FROM furryplans
		WHERE Public=true 
		AND (6371 * acos( 
                cos( radians( Latitude) ) 
              * cos( radians( ? ) ) 
              * cos( radians( ? ) - radians(Longitude) ) 
              + sin( radians( Latitude ) ) 
              * sin( radians( ? ) )
        ) ) < ? 
		AND (EventDateTime > NOW() - INTERVAL 7 DAY  OR EndDateTime > NOW() - INTERVAL 2 DAY) 
		ORDER BY EventDateTime `

	var events []*FurryPlansWithAttend
	res := c.db.Raw(sql, Latitude, Longitude, Latitude, distMiles).Scan(&events)
	if res.Error != nil {
		return nil, res.Error
	}

	// Clean up the old syntax from the previous event bot
	list := make([]dbInterface.DBEvent, len(events))
	for i, event := range events {
		event.cleanOldSyntax()

		// Update the time on the event to match the time zone.
		if event.TimeZone != "" {
			tz := localizer.FromTimeZone(event.TimeZone)
			event.DateTime.Time = event.DateTime.Time.In(tz)
		}
		list[i] = c.wrapWithAttend(event)
	}

	return list, nil
}

// GLOBAL MESSAGING SYSTEM

// GetAllUsers will return a list of ALL users that have EVER created an event with this bot.
func (c *Connector) GetAllUsers() ([]int64, error) {
	query := "select ownerid from telegram.furryplans group by ownerid;"

	var owners []int64
	res := c.db.Raw(query).Scan(&owners)
	if res.Error != nil {
		return nil, res.Error
	}

	return owners, nil
}

func (c *Connector) GlobalShouldSend(chatId int64) bool {
	var send GlobalSend
	query := c.db.Where(&GlobalSend{UserID: chatId})
	if err := query.First(&send).Error; err != nil {
		return true
	}

	// only if they haven't gotten the message before, put it at 0
	return send.SendType == 0
}
func (c *Connector) GlobalMarkBadUser(chatId int64) {
	send := &GlobalSend{
		UserID:   chatId,
		SendType: 2,
	}
	if c.db.Model(&GlobalSend{}).Where(&GlobalSend{UserID: chatId}).Updates(&send).RowsAffected == 0 {
		c.db.Create(&send)
	}
}
func (c *Connector) GlobalSent(chatId int64) {
	send := &GlobalSend{
		UserID:   chatId,
		SendType: 1,
	}
	if c.db.Model(&GlobalSend{}).Where(&GlobalSend{UserID: chatId}).Updates(&send).RowsAffected == 0 {
		c.db.Create(&send)
	}
}

type eventConnector struct {
	db *gorm.DB
	ev *FurryPlans
	// only present when used from the Calendar Feed
	canAttend dbInterface.CanAttend
}

func (e *eventConnector) updateEvent(columns ...string) error {
	q := e.db.Where(&FurryPlans{EventID: e.ev.EventID})

	// Select all the columns we need to update
	q = q.Select(columns)

	return q.Updates(e.ev).Error
}

func (e *eventConnector) GetAttending(userId int64) ([]*dbInterface.Attend, error) {
	var attend []*FurryPlansAttend
	query := e.db.Where(&FurryPlansAttend{EventID: e.ev.EventID})
	if userId != -1 {
		query = query.Where(&FurryPlansAttend{UserID: userId})
	}
	query = query.Order("UCASE(UserName) ASC")

	err := query.Find(&attend).Error
	if err != nil {
		return nil, err
	}
	list := make([]*dbInterface.Attend, len(attend))
	for i, att := range attend {

		var guests []string
		_ = json.Unmarshal([]byte(att.Guests), &guests)

		list[i] = &dbInterface.Attend{
			EventID:   att.EventID,
			UserID:    att.UserID,
			UserName:  cleanOldSyntaxText(att.UserName),
			CanAttend: att.CanAttend,
			PlusMany:  att.PlusMany,
			Guests:    guests,
		}
	}
	return list, nil
}

// AmIAttending returns true or false depending on whether or not this user is attending this event
func (e *eventConnector) AmIAttending(id int64) bool {
	attending, err := e.GetAttending(id)
	if err != nil {
		return false
	}
	for _, attend := range attending {
		if attend.CanAttend > 0 {
			return true
		}
	}
	return false
}

// Attending marks (or unmarks) a person as attending this event.
func (e *eventConnector) Attending(userId int64, name string, attendType dbInterface.CanAttend, plusPeople int, guests []string) dbInterface.AttendMsgs {
	// Does this event have a cap?

	if attendType == dbInterface.CANATTEND_MAYBE || attendType == dbInterface.CANATTEND_NO {
		// No need to check maxes here.
	} else {

		if e.ev.MaxAttendees > 0 {
			// See how many people are currently attending.
			sql := `SELECT CONCAT(COUNT(*) + SUM(PlusMany), '') as AttendCount 
		FROM furryplansattend WHERE EventID=?
		AND CanAttend IN (1, 20, 30) AND userID <> ?`

			var count *int
			res := e.db.Raw(sql, e.ev.EventID, userId).Scan(&count)
			if res.Error != nil {
				return dbInterface.ATTEND_ERROR
			}

			if count == nil {
				// Nothing is nothing
				numZero := 0
				count = &numZero
			}

			if e.ev.MaxAttendees > 0 && *count+1+plusPeople > e.ev.MaxAttendees {
				return dbInterface.ATTEND_FULL
			}

		}
	}
	e.updateAttendTable(userId, name, attendType, plusPeople, guests)
	if attendType == dbInterface.CANATTEND_YES || attendType == dbInterface.CANATTEND_SUITING || attendType == dbInterface.CANATTEND_PHOTOGRAPHER {
		return dbInterface.ATTEND_ADDED
	}
	if attendType == dbInterface.CANATTEND_MAYBE {
		return dbInterface.ATTEND_MAYBE
	}
	if attendType == dbInterface.CANATTEND_NO {
		return dbInterface.ATTEND_REMOVED
	}
	return dbInterface.ATTEND_ERROR
}

func (e *eventConnector) updateAttendTable(userId int64, name string, attendVal dbInterface.CanAttend, plusPeople int, guests []string) {
	// Instead of using REPLACE INTO, we use GORM's UPDATE/CREATE workflow.

	guestsJson, _ := json.Marshal(&guests)

	attend := FurryPlansAttend{
		EventID:   e.ev.EventID,
		UserID:    userId,
		UserName:  name,
		CanAttend: int(attendVal),
		PlusMany:  plusPeople,
		Guests:    string(guestsJson),
	}

	if e.db.Model(&FurryPlansAttend{}).Where(&FurryPlansAttend{EventID: e.ev.EventID, UserID: userId}).Select("*").Updates(&attend).RowsAffected == 0 {
		e.db.Create(&attend)
	}
}

// SavePosting stores the inline message ID of the posting so the event can be refreshed later
func (e *eventConnector) SavePosting(MessageID string) {
	posting := &FurryPlansPosted{
		EventID:   e.ev.EventID,
		MessageID: MessageID,
	}
	if e.db.Model(&FurryPlansPosted{}).Updates(&posting).RowsAffected == 0 {
		e.db.Create(&posting)
	}
}
func (e *eventConnector) SavePostingRegular(chatId int64, messageId int) {
	posting := &FurryPlansPosted{
		EventID: e.ev.EventID,
		ChatID:  chatId,
		LocalID: messageId,
	}
	if e.db.Model(&FurryPlansPosted{}).Updates(&posting).RowsAffected == 0 {
		e.db.Create(&posting)
	}
}

func (e *eventConnector) Postings() ([]dbInterface.Posting, error) {
	var posted []FurryPlansPosted
	query := e.db.Where(&FurryPlansPosted{EventID: e.ev.EventID})
	err := query.Find(&posted).Error
	if err != nil {
		return nil, err
	}

	list := make([]dbInterface.Posting, len(posted))
	for i, post := range posted {
		list[i].InlineMessageID = post.MessageID
		list[i].ChatID = post.ChatID
		list[i].MessageId = post.LocalID
	}

	return list, nil
}

func (e *eventConnector) DeletePosting(MessageID string) error {
	return e.db.Model(&FurryPlansPosted{}).Unscoped().Delete(&FurryPlansPosted{
		EventID:   e.ev.EventID,
		MessageID: MessageID,
	}).Error
}
func (e *eventConnector) DeletePostingRegular(chatId int64, messageId int) error {
	return e.db.Model(&FurryPlansPosted{}).Unscoped().Delete(&FurryPlansPosted{
		EventID: e.ev.EventID,
		ChatID:  chatId,
		LocalID: messageId,
	}).Error
}

func (e *eventConnector) Name() string {
	return e.ev.Name
}

func (e *eventConnector) SetName(t string) error {
	e.ev.Name = t
	return e.updateEvent("EventName")
}

func (e *eventConnector) ID() uint {
	return e.ev.EventID
}

func (e *eventConnector) GetCanAttend() dbInterface.CanAttend {
	return e.canAttend
}

func (e *eventConnector) DateTime() time.Time {
	return e.ev.DateTime.Time
}

func (e *eventConnector) SetDateTime(d time.Time) error {
	e.ev.DateTime = sql.NullTime{
		Time:  d,
		Valid: true,
	}
	return e.updateEvent("EventDateTime")
}
func (e *eventConnector) EndDateTime() time.Time {
	return e.ev.EndDateTime.Time
}

func (e *eventConnector) SetEndDateTime(d time.Time) error {
	e.ev.EndDateTime = sql.NullTime{
		Time:  d,
		Valid: true,
	}
	return e.updateEvent("EndDateTime")
}

func (e *eventConnector) TimeZone() string {
	return e.ev.TimeZone
}

func (e *eventConnector) SetTimeZone(t string) error {
	e.ev.TimeZone = t
	return e.updateEvent("TimeZone")
}

func (e *eventConnector) OwnerName() string {
	return e.ev.OwnerName
}

func (e *eventConnector) SetOwnerName(t string) error {
	e.ev.OwnerName = t
	return e.updateEvent("ownerName")
}

func (e *eventConnector) Location() string {
	return e.ev.Location
}

func (e *eventConnector) SetLocation(t string) error {
	e.ev.Location = t
	return e.updateEvent("Location")
}

func (e *eventConnector) Notes() string {
	return e.ev.Notes
}

func (e *eventConnector) SetNotes(t string) error {
	e.ev.Notes = t
	return e.updateEvent("Notes")
}

func (e *eventConnector) PictureURL() string {
	return e.ev.PictureURL
}

func (e *eventConnector) SetPictureURL(t string) error {
	e.ev.PictureURL = t
	return e.updateEvent("PictureURL")
}

func (e *eventConnector) Public() (bool, float64, float64) {
	return e.ev.Public, e.ev.Latitude, e.ev.Longitude
}

func (e *eventConnector) SetPublic(v bool, lat float64, lon float64) error {
	e.ev.Public = v
	e.ev.Latitude = lat
	e.ev.Longitude = lon
	return e.updateEvent("Public", "Latitude", "Longitude")
}

func (e *eventConnector) SetPublicOnly(v bool) error {
	e.ev.Public = v
	return e.updateEvent("Public")
}

func (e *eventConnector) Language() string {
	return e.ev.Language
}

func (e *eventConnector) SetLanguage(t string) error {
	e.ev.Language = t
	return e.updateEvent("Language")
}

func (e *eventConnector) Suitwalk() bool {
	return e.ev.Suitwalk == 1
}

func (e *eventConnector) SetSuitwalk(v bool) error {
	if v {
		e.ev.Suitwalk = 1
		return e.updateEvent("Suitwalk")
	}
	e.ev.Suitwalk = 0
	return e.updateEvent("Suitwalk")
}

func (e *eventConnector) HideNames() bool {
	return e.ev.HideNames
}

func (e *eventConnector) SetHideNames(v bool) error {
	e.ev.HideNames = v
	return e.updateEvent("HideNames")
}

func (e *eventConnector) Closed() bool {
	return e.ev.Closed
}

func (e *eventConnector) SetClosed(v bool) error {
	e.ev.Closed = v
	return e.updateEvent("Closed")
}

func (e *eventConnector) MaxGuests() int {
	return e.ev.MaxGuests
}

func (e *eventConnector) SetMaxGuests(v int) error {
	e.ev.MaxGuests = v
	return e.updateEvent("MaxGuests")
}

func (e *eventConnector) MaxAttendees() int {
	return e.ev.MaxAttendees
}

func (e *eventConnector) SetMaxAttendees(i int) error {
	e.ev.MaxAttendees = i
	return e.updateEvent("MaxAttendees")
}

func (e *eventConnector) DisableMaybe() bool {
	return e.ev.DisableMaybe == 1
}

func (e *eventConnector) SetDisableMaybe(v bool) error {
	if v {
		e.ev.DisableMaybe = 1
		return e.updateEvent("DisableMaybe")
	}
	e.ev.DisableMaybe = 0
	return e.updateEvent("DisableMaybe")

}

func (e *eventConnector) SharingAllowed() bool {
	return e.ev.AllowShare == 1
}

func (e *eventConnector) SetSharingAllowed(v bool) error {
	if v {
		e.ev.AllowShare = 1
		return e.updateEvent("AllowShare")
	}
	e.ev.AllowShare = 0
	return e.updateEvent("AllowShare")

}
