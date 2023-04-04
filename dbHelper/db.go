package dbHelper

import (
	"encoding/json"
	"fmt"
	"furryplansbot.avbrand.com/localizer"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"html"
	"os"
	"regexp"
	"strconv"
)

var db *gorm.DB

// AttendMsgs is for the messages that can result from clicking the attendance buttons
type AttendMsgs int

const (
	ATTEND_ERROR AttendMsgs = iota
	ATTEND_ADDED
	ATTEND_MAYBE
	ATTEND_REMOVED
	ATTEND_FULL
	ATTEND_ACTIVE
)

type CanAttend int

const (
	CANATTEND_NO    CanAttend = 0
	CANATTEND_YES   CanAttend = 1
	CANATTEND_MAYBE CanAttend = 2
)

// In the old version of the furry plans bot, for some reason, this syntax was used for special characters:
// Hello /$\uabcd World
// This regex helps clean that up.
var oldSyntax = regexp.MustCompile(`/\$\\u`)

func InitDB(dsnFile string) error {

	dsn, err := os.ReadFile(dsnFile)
	if err != nil {
		return err
	}
	//dsn := "telegram:telegram@tcp(10.1.0.60:3306)/telegram?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(string(dsn)), &gorm.Config{})
	if err != nil {
		return err
	}

	// Migrate the schema
	err = db.AutoMigrate(&UserPrefs{})
	if err != nil {
		return fmt.Errorf("db migration error: %v", err)
	}

	return nil
}

// GetPrefs returns the user preferences of the user.
func GetPrefs(userid int64) UserPrefs {
	var userPrefs UserPrefs
	if err := db.Where(&UserPrefs{UserID: userid}).First(&userPrefs).Error; err != nil {
		// DEFAULT user prefs
		return UserPrefs{
			UserID:   userid,
			Language: "en-US",
		}
	}
	return userPrefs
}

func SavePrefs(userid int64, prefs UserPrefs, colName string) {
	if db.Model(&UserPrefs{}).Where(&UserPrefs{UserID: userid}).Select(colName).Updates(&prefs).RowsAffected == 0 {
		db.Create(&prefs)
	}
}

func CreateEvent(event *FurryPlans) (uint, error) {
	err := db.Create(&event).Error
	if err != nil {
		return 0, err
	}
	return event.EventID, nil
}

// GetEvent returns the event, and also an overridden localizer if they changed the language of the event.
func GetEvent(eventId uint, ownerId int64) (*FurryPlans, *localizer.Localizer, error) {
	var event FurryPlans
	if err := db.Where(&FurryPlans{EventID: eventId, OwnerID: fmt.Sprintf("%v", ownerId)}).First(&event).Error; err != nil {
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

	return &event, loc, nil
}

// GetEventByHash searches for this event by the hash.
func GetEventByHash(hash string, saltValue string) (*FurryPlans, *localizer.Localizer, error) {

	// TODO: This whole sharing mechanism needs to be overhauled.
	sql := `SELECT * FROM furryplans WHERE 
            CONCAT('', MD5(CONCAT(eventID, ?))) = ? AND 
            EventDateTime > NOW() - INTERVAL 2 DAY AND 
            AllowShare=1`

	var event FurryPlans
	res := db.Raw(sql, saltValue, hash).Scan(&event)
	if res.Error != nil {
		return nil, nil, res.Error
	}

	// Clean up the old syntax from the previous event bot
	event.cleanOldSyntax()
	loc := localizer.FromLanguage(event.Language)

	// Update the time on the event to match the time zone.
	if event.TimeZone != "" {
		tz := localizer.FromTimeZone(event.TimeZone)
		event.DateTime.Time = event.DateTime.Time.In(tz)
	}
	return &event, loc, nil
}

func SearchEvents(ownerId int64, searchText string) ([]*FurryPlans, error) {
	var events []*FurryPlans
	query := db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Where("EventName LIKE ?", "%"+searchText+"%").Order("EventDateTime DESC")
	// TODO: This query doesn't use time zones, it probably should.
	query = query.Where("EventDateTime > NOW() - INTERVAL 2 DAY")
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		event.cleanOldSyntax()
	}
	return events, nil
}

func GetEvents(ownerId int64, includeOld bool) ([]*FurryPlans, error) {
	var events []*FurryPlans
	query := db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Order("EventDateTime DESC").Limit(100)
	if !includeOld {
		// TODO: This query doesn't use time zones, it probably should.
		query = query.Where("EventDateTime > NOW() - INTERVAL 2 DAY")
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		event.cleanOldSyntax()
	}
	return events, nil
}

func (event *FurryPlans) cleanOldSyntax() {
	event.Name = cleanOldSyntaxText(event.Name)
	event.OwnerName = cleanOldSyntaxText(event.OwnerName)
	event.Notes = cleanOldSyntaxText(event.Notes)
}

func cleanOldSyntaxText(text string) string {

	if !oldSyntax.MatchString(text) {
		return text
	}
	fixed := oldSyntax.ReplaceAllString(text, "\\u")

	// Use the json unmarshaler to fix it
	var str string
	_ = json.Unmarshal([]byte("\""+fixed+"\""), &str)
	return htmlEntities(str)
}

func htmlEntities(str string) string {
	str = html.EscapeString(str)
	res := ""
	runes := []rune(str)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 128 {
			res += string(r)
		} else {
			res += "&#" + strconv.FormatInt(int64(r), 10) + ";"
		}
	}
	return res
}

func (event *FurryPlans) UpdateEvent(column string) error {
	return db.Where(&FurryPlans{EventID: event.EventID}).Select(column).Updates(event).Error
}

func (event *FurryPlans) GetAttending() ([]*FurryPlansAttend, error) {
	var attend []*FurryPlansAttend
	query := db.Where(&FurryPlansAttend{EventID: event.EventID}).Order("UCASE(UserName) DESC")
	err := query.Find(&attend).Error
	if err != nil {
		return nil, err
	}
	for _, att := range attend {
		att.UserName = cleanOldSyntaxText(att.UserName)
	}
	return attend, nil
}

// Attending marks (or unmarks) a person as attending this event.
func (event *FurryPlans) Attending(userId int64, name string, attendType CanAttend, plusPeople int) AttendMsgs {
	// Does this event have a cap?

	if attendType == CANATTEND_MAYBE || attendType == CANATTEND_NO {
		// No need to check maxes here.
	} else {

		if event.MaxAttendees > 0 {
			// See how many people are currently attending.
			sql := `SELECT CONCAT(COUNT(*) + SUM(PlusMany), '') as AttendCount 
		FROM furryplansattend WHERE EventID=?
		AND CanAttend IN (1, 20, 30) AND userID <> ?`

			// TODO make sure this is working
			var count *int
			res := db.Raw(sql, event.EventID, userId).Scan(&count)
			if res.Error != nil {
				return ATTEND_ERROR
			}

			if count == nil {
				// Nothing is nothing
				numZero := 0
				count = &numZero
			}

			if event.MaxAttendees > 0 && *count+1+plusPeople > event.MaxAttendees {
				return ATTEND_FULL
			}

		}
	}
	event.updateAttendTable(userId, name, attendType, plusPeople)
	if attendType == CANATTEND_YES {
		return ATTEND_ADDED
	}
	if attendType == CANATTEND_MAYBE {
		return ATTEND_MAYBE
	}
	if attendType == CANATTEND_NO {
		return ATTEND_REMOVED
	}
	return ATTEND_ERROR
}

func (event *FurryPlans) updateAttendTable(userId int64, name string, attendVal CanAttend, plusPeople int) {
	// Instead of using REPLACE INTO, we use GORM's UPDATE/CREATE workflow.
	attend := FurryPlansAttend{
		EventID:   event.EventID,
		UserID:    userId,
		UserName:  name,
		CanAttend: int(attendVal),
		PlusMany:  plusPeople,
	}

	if db.Model(&FurryPlansAttend{}).Where(&FurryPlansAttend{EventID: event.EventID, UserID: userId}).Select("*").Updates(&attend).RowsAffected == 0 {
		db.Create(&attend)
	}
}

// SavePosting stores the inline message ID of the posting so the event can be refreshed later
func (event *FurryPlans) SavePosting(MessageID string) {
	posting := &FurryPlansPosted{
		EventID:   event.EventID,
		MessageID: MessageID,
	}
	if db.Model(&FurryPlansPosted{}).Updates(&posting).RowsAffected == 0 {
		db.Create(&posting)
	}
}

func (event *FurryPlans) Postings() ([]FurryPlansPosted, error) {
	var posted []FurryPlansPosted
	query := db.Where(&FurryPlansPosted{EventID: event.EventID})
	err := query.Find(&posted).Error
	if err != nil {
		return nil, err
	}
	return posted, nil
}

func (event *FurryPlans) DeletePosting(MessageID string) error {
	return db.Model(&FurryPlansPosted{}).Unscoped().Delete(&FurryPlansPosted{
		EventID:   event.EventID,
		MessageID: MessageID,
	}).Error
}
