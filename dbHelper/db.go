package dbHelper

import (
	"fmt"
	"furryplansbot.avbrand.com/localizer"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
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

	loc := localizer.FromLanguage(event.Language)

	// Update the time on the event to match the time zone.
	if event.TimeZone != "" {
		tz := localizer.FromTimeZone(event.TimeZone)
		event.DateTime.Time = event.DateTime.Time.In(tz)
	}

	return &event, loc, nil
}

func GetEvents(ownerId int64, includeOld bool) ([]FurryPlans, error) {
	var events []FurryPlans
	query := db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Order("EventDateTime DESC").Limit(100)
	if !includeOld {
		// TODO: This query doesn't use time zones, it probably should.
		query = query.Where("EventDateTime > NOW() - INTERVAL 2 DAY")
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (event *FurryPlans) UpdateEvent(column string) error {
	return db.Where(&FurryPlans{EventID: event.EventID}).Select(column).Updates(event).Error
}

func (event *FurryPlans) GetAttending() ([]FurryPlansAttend, error) {
	var attend []FurryPlansAttend
	query := db.Where(&FurryPlansAttend{EventID: event.EventID}).Order("UCASE(UserName) DESC")
	err := query.Find(&attend).Error
	if err != nil {
		return nil, err
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

			res := db.Raw(sql, event.EventID, userId)
			if res.Error != nil {
				return ATTEND_ERROR
			}
			var count *int
			err := res.Row().Scan(&count)
			if err != nil {
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
