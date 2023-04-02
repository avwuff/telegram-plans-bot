package userManager

import (
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
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
	Eph    *UserEphemeral
	Prefs  dbHelper.UserPrefs
	Locale *localizer.Localizer
}

var userEph = make(map[int64]*UserEphemeral)

// Get will either retrieve or create a user data object for the specified user.
func Get(userid int64) *UserInfo {

	usrData := &UserInfo{
		Prefs: dbHelper.GetPrefs(userid),
	}
	usrData.Locale = localizer.FromLanguage(usrData.Prefs.Language)

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
