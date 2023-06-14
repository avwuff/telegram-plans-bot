package dbHelper

import (
	"encoding/json"
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"regexp"
)

// In the old version of the furry plans bot, for some reason, this syntax was used for special characters:
// Hello /$\uabcd World
// This regex helps clean that up.
var oldSyntax = regexp.MustCompile(`/\$\\u`)

func InitDB(dsnFile string) (*Connector, error) {

	dsn, err := os.ReadFile(dsnFile)
	if err != nil {
		return nil, err
	}
	//dsn := "telegram:telegram@tcp(10.1.0.60:3306)/telegram?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(string(dsn)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema
	err = db.AutoMigrate(&UserPrefs{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	err = db.AutoMigrate(&FurryPlansPosted{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	return &Connector{db: db}, nil
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
	return helpers.HtmlEntities(str)
}
