package dbHelper

import (
	"database/sql"
	"gorm.io/gorm"
)

type UserPrefs struct {
	gorm.Model
	UserID   int64  // the ID of the user these prefs are being stored for
	Language string // The language code the user prefers
}

type FurryPlans struct {
	EventID   uint         `gorm:"primarykey;column:eventID"`
	OwnerID   string       `gorm:"column:ownerID"` // Should be an int64
	Name      string       `gorm:"column:EventName"`
	DateTime  sql.NullTime `gorm:"column:EventDateTime"`
	CreatedAt sql.NullTime `gorm:"column:CreatedAt"`
	OwnerName string       `gorm:"column:ownerName"`
	Location  string       `gorm:"column:EventLocation"`
	Notes     string       `gorm:"column:Notes"`

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
