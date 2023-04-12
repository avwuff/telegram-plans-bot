package userManager

import (
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"time"
)

// User Modes handles keeping track of what mode a user is in.
// Can also store some arbitrary data per-user

// For now. we implement this as a simple map, but later we might back it with a database
// and add a mutex.

// UserEphemeral contains information that is ephemeral and not stored in the database.
type UserEphemeral struct {
	UserMode Mode
	Data     map[string]interface{}
}

type UserInfo struct {
	Eph      *UserEphemeral
	Prefs    dbInterface.Prefs
	Locale   *localizer.Localizer
	TimeZone *time.Location
}

var userEph = make(map[int64]*UserEphemeral)
var db dbInterface.DBFeatures

func Init(useDb dbInterface.DBFeatures) {
	db = useDb
}

// Get will either retrieve or create a user data object for the specified user.
func Get(userid int64) *UserInfo {

	usrData := &UserInfo{
		Prefs: db.GetPrefs(userid),
	}
	usrData.Locale = localizer.FromLanguage(usrData.Prefs.Language)
	usrData.TimeZone = localizer.FromTimeZone(usrData.Prefs.TimeZone)

	// Find and attach the ephemeral object
	usrEph, ok := userEph[userid]
	if !ok {
		usrEph = &UserEphemeral{
			Data: make(map[string]interface{}),
		}
		userEph[userid] = usrEph
	}
	usrData.Eph = usrEph

	return usrData
}

func (u *UserInfo) SetData(key string, data interface{}) {
	u.Eph.Data[key] = data
}

func (u *UserInfo) SetMode(mode Mode) {
	u.Eph.UserMode = mode
}

func (u *UserInfo) GetData(key string) interface{} {
	data, ok := u.Eph.Data[key]
	if !ok {
		return nil
	}
	return data
}
