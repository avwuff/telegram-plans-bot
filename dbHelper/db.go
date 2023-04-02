package dbHelper

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
)

var db *gorm.DB

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

func SavePrefs(userid int64, prefs UserPrefs) {
	if db.Model(&UserPrefs{}).Where(&UserPrefs{UserID: userid}).Updates(&prefs).RowsAffected == 0 {
		db.Create(&prefs)
	}
}

func CreateEvent(event *FurryPlans) (uint, error) {
	err := db.Create(&event).Error
	if err != nil {
		return 0, err
	}

	//last_id := db.Exec("SELECT LAST_INSERT_ID() as id")

	return event.EventID, nil
}

func GetEvent(eventId uint, ownerId int64) (*FurryPlans, error) {
	var event FurryPlans
	if err := db.Where(&FurryPlans{EventID: eventId, OwnerID: fmt.Sprintf("%v", ownerId)}).First(&event).Error; err != nil {
		return nil, fmt.Errorf("event not found")
	}
	return &event, nil
}

func UpdateEvent(eventId uint, event *FurryPlans) error {
	return db.Where(&FurryPlans{EventID: eventId}).Save(event).Error
}

func GetEvents(ownerId int64, includeOld bool) ([]FurryPlans, error) {
	var events []FurryPlans
	query := db.Where(&FurryPlans{OwnerID: fmt.Sprintf("%v", ownerId)}).Order("EventDateTime DESC").Limit(100)
	if !includeOld {
		// TODO: TIMEZONES
		query = query.Where("EventDateTime > NOW() - INTERVAL 1 DAY")
	}
	err := query.Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}
