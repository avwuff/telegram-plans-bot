package dbInterface

import (
	"furryplansbot.avbrand.com/localizer"
	"time"
)

// Generate mocks of these interfaces for testing
//go:generate go run github.com/vektra/mockery/v2 --name=DBFeatures --structname DBFeaturesMock --filename dbfeatures_mock.go --inpackage
//go:generate go run github.com/vektra/mockery/v2 --name=DBEvent --structname DBEventMock --filename dbevent_mock.go --inpackage

// DBFeatures defines the features we expect to see from the database in a nice, tidy interface.
type DBFeatures interface {
	GetPrefs(userid int64) Prefs
	SavePrefs(userid int64, prefs Prefs, colName string) error
	CreateEvent(OwnerID int64, Name string, DateTime time.Time, TimeZone string, OwnerName string, Location string, Language string, Notes string) (uint, error)
	GetEvent(eventId uint, ownerId int64) (DBEvent, error)
	GetEventByHash(hash string, saltValue string, shareMode bool) (DBEvent, *localizer.Localizer, error)
	CalendarFeed(ownerId int64) ([]DBEvent, error)
	SearchEvents(ownerId int64, searchText string) ([]DBEvent, error)
	GetEvents(ownerId int64, includeOld bool) ([]DBEvent, error)
	GetAllUsers() ([]int64, error)
	GlobalShouldSend(id int64) bool
	GlobalMarkBadUser(id int64)
	GlobalSent(id int64)
	NearbyFeed(latitude float64, longitude float64, distKM int) ([]DBEvent, error)
}

// DBEvent provides access to features on the event.
type DBEvent interface {
	GetAttending(userId int64) ([]*Attend, error)
	Attending(userId int64, name string, attendType CanAttend, plusPeople int, guests []string) AttendMsgs
	SavePosting(MessageID string)
	SavePostingRegular(chatId int64, messageId int)
	Postings() ([]Posting, error)
	DeletePosting(MessageID string) error
	DeletePostingRegular(chatId int64, messageId int) error

	// Setters and Getters
	Name() string
	SetName(t string) error
	ID() uint
	DateTime() time.Time
	SetDateTime(d time.Time) error
	EndDateTime() time.Time
	SetEndDateTime(d time.Time) error
	TimeZone() string
	SetTimeZone(t string) error
	OwnerName() string
	SetOwnerName(t string) error
	Location() string
	SetLocation(t string) error
	Notes() string
	SetNotes(t string) error
	Language() string
	SetLanguage(t string) error
	Suitwalk() bool
	SetSuitwalk(v bool) error
	HideNames() bool
	SetHideNames(v bool) error
	MaxGuests() int
	SetMaxGuests(v int) error
	Closed() bool
	SetClosed(v bool) error
	MaxAttendees() int
	SetMaxAttendees(i int) error
	DisableMaybe() bool
	SetDisableMaybe(v bool) error
	SharingAllowed() bool
	SetSharingAllowed(v bool) error
	GetCanAttend() CanAttend
	AmIAttending(id int64) bool
	PictureURL() string
	SetPictureURL(t string) error
	Public() (bool, float64, float64)
	SetPublic(v bool, lat float64, lon float64) error
	SetPublicOnly(v bool) error
}

type Attend struct {
	EventID   uint
	UserID    int64
	UserName  string
	CanAttend int
	PlusMany  int
	Guests    []string
}

type CanAttend int

const (
	CANATTEND_NO    CanAttend = 0
	CANATTEND_YES   CanAttend = 1
	CANATTEND_MAYBE CanAttend = 2

	CANATTEND_SUITING      CanAttend = 20
	CANATTEND_PHOTOGRAPHER CanAttend = 30
	CANATTEND_SPOTTING     CanAttend = 0
)

// Posting is used to list out all the postings
type Posting struct {
	InlineMessageID string
	ChatID          int64
	MessageId       int
}

// AttendMsgs is for the messages that can result from clicking the attendance buttons
type AttendMsgs int

const (
	ATTEND_ERROR AttendMsgs = iota
	ATTEND_ADDED
	ATTEND_MAYBE
	ATTEND_REMOVED
	ATTEND_FULL
	ATTEND_ACTIVE
	ATTEND_CLOSED
)

type Prefs struct {
	// General data about this user
	SetupComplete bool // Has the user completed the setup process?

	// User preferences
	Language string // The language code the user prefers
	TimeZone string // The user's default time zone
}
