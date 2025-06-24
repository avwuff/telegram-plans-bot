package dbHelper

import (
	"database/sql"
	"gorm.io/gorm"
)

// UserPrefs stores per-user preferences
type UserPrefs struct {
	gorm.Model
	UserID int64 // the ID of the user these prefs are being stored for

	// General data about this user
	SetupComplete bool // Has the user completed the setup process?

	// User preferences
	Language string // The language code the user prefers
	TimeZone string // The user's default time zone
}

type GlobalSend struct {
	gorm.Model
	UserID   int64 `gorm:"primarykey"`
	SendType int   // 0 - unsent, 1 - sent, 2 - user no longer valid so can't send.
}

type FurryPlans struct {
	EventID     uint         `gorm:"primarykey;column:eventID"`
	OwnerID     string       `gorm:"column:ownerID"` // Should be an int64
	Name        string       `gorm:"column:EventName"`
	DateTime    sql.NullTime `gorm:"column:EventDateTime"`
	EndDateTime sql.NullTime `gorm:"column:EndDateTime"`
	TimeZone    string       `gorm:"column:TimeZone"`
	CreatedAt   sql.NullTime `gorm:"column:CreatedAt"`
	OwnerName   string       `gorm:"column:ownerName"`
	Location    string       `gorm:"column:EventLocation"`
	Notes       string       `gorm:"column:Notes"`
	PictureURL  string       `gorm:"column:PictureURL"`
	Language    string       `gorm:"column:Language"`

	// Should really have been BOOLs, they are ints for compatibility with the old system
	Suitwalk     int `gorm:"column:Suitwalk"`
	MaxAttendees int `gorm:"column:MaxAttendees"`
	DisableMaybe int `gorm:"column:DisableMaybe"`
	AllowShare   int `gorm:"column:AllowShare"`

	// new items are being added as bools!
	Closed    bool    `gorm:"column:Closed"`
	MaxGuests int     `gorm:"column:MaxGuests"`
	HideNames bool    `gorm:"column:HideNames"`
	Public    bool    `gorm:"column:Public"`   // Whether or not the event is open to the general public
	Latitude  float64 `gorm:"column:Latitude"` // The lat & long, generally not shown to the end user, for public feeds
	Longitude float64 `gorm:"column:Longitude"`
}

// FurryPlansWithAttend is the same as FurryPlans, and doesn't actually represent a table in the DB.
// Instead, it is the data that comes back from the CalendarFeed so that we can get the user's attendance status.
type FurryPlansWithAttend struct {
	FurryPlans
	CanAttend int `gorm:"column:CanAttend"`
}

// TableName is used to override Gorm's default table naming
func (*FurryPlans) TableName() string {
	return "furryplans"
}

// FurryPlansAttend keeps track of who has marked themselves as able to attend an event.
type FurryPlansAttend struct {
	EventID   uint   `gorm:"primarykey;column:eventID"`
	UserID    int64  `gorm:"primarykey;column:userID"`
	UserName  string `gorm:"column:UserName"`
	CanAttend int    `gorm:"column:canAttend"`
	PlusMany  int    `gorm:"column:plusMany"`
	Guests    string `gorm:"column:guestList"`
}

func (FurryPlansAttend) TableName() string {
	return "furryplansattend"
}

// FurryPlansPosted tracks where the event has been posted and the message ID there.
// This allows all the places where it has been posted to be updated at once.
type FurryPlansPosted struct {
	EventID   uint   `gorm:"primarykey;column:event_id"`
	MessageID string `gorm:"primarykey;column:message_id"`
	ChatID    int64  `gorm:"primarykey;column:chat_id"`
	LocalID   int    `gorm:"primarykey;column:local_id"`
}

func (FurryPlansPosted) TableName() string {
	return "furryplansposted"
}
