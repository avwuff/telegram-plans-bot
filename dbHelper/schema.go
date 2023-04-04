package dbHelper

import (
	"database/sql"
	"gorm.io/gorm"
)

type UserPrefs struct {
	gorm.Model
	UserID int64 // the ID of the user these prefs are being stored for

	// General data about this user
	SetupComplete bool // Has the user completed the setup process?

	// User preferences
	Language string // The language code the user prefers
	TimeZone string // The user's default time zone
}

type FurryPlans struct {
	EventID          uint         `gorm:"primarykey;column:eventID"`
	OwnerID          string       `gorm:"column:ownerID"` // Should be an int64
	Name             string       `gorm:"column:EventName"`
	DateTime         sql.NullTime `gorm:"column:EventDateTime"`
	TimeZone         string       `gorm:"column:TimeZone"`
	CreatedAt        sql.NullTime `gorm:"column:CreatedAt"`
	OwnerName        string       `gorm:"column:ownerName"`
	Location         string       `gorm:"column:EventLocation"`
	Notes            string       `gorm:"column:Notes"`
	LanguageOverride string       `gorm:"column:Language"`

	// Should really have been BOOLs
	Suitwalk     int `gorm:"column:Suitwalk"`
	MaxAttendees int `gorm:"column:MaxAttendees"`
	DisableMaybe int `gorm:"column:DisableMaybe"`
	AllowShare   int `gorm:"column:AllowShare"`
}

// TableName is used to override Gorm's default table naming
func (FurryPlans) TableName() string {
	return "furryplans"
}

type FurryPlansAttend struct {
	EventID   uint   `gorm:"primarykey;column:eventID"`
	UserID    int64  `gorm:"primarykey;column:userID"`
	UserName  string `gorm:"column:UserName"`
	CanAttend int    `gorm:"column:canAttend"`
	PlusMany  int    `gorm:"column:plusMany"`
}

// TableName is used to override Gorm's default table naming
func (FurryPlansAttend) TableName() string {
	return "furryplansattend"
}
