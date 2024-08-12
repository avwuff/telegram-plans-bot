package dbHelper

import (
	"encoding/json"
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"regexp"
	"strings"
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

	// Remove whitespace from the file
	dsnData := strings.TrimSpace(string(dsn))

	db, err := gorm.Open(mysql.Open(dsnData), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema

	// These tables were present in the old schema also
	err = db.AutoMigrate(&FurryPlans{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	err = db.AutoMigrate(&FurryPlansAttend{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	err = db.AutoMigrate(&FurryPlansWithAttend{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	// New tables
	err = db.AutoMigrate(&UserPrefs{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	err = db.AutoMigrate(&FurryPlansPosted{})
	if err != nil {
		return nil, fmt.Errorf("db migration error: %v", err)
	}

	err = db.AutoMigrate(&GlobalSend{})
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
