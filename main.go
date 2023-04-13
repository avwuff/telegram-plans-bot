package main

import (
	"context"
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/dbInterface"
	_ "furryplansbot.avbrand.com/internal/translations"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgPlansBot"
	"furryplansbot.avbrand.com/userManager"
	"furryplansbot.avbrand.com/webserver"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var dbMain dbInterface.DBFeatures
var logFileName string
var logFile *os.File

func main() {

	// Rotate the log output every day.
	setLog()
	go checkLog()

	log.Println("== Furry Plans Bot Startup ==")
	// Initialize the language list -- this must be called here since the translations package is now initialized.
	localizer.InitLang()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	saltValue, err := os.ReadFile("salt.txt")
	if err != nil {
		panic(err)
	}

	log.Println("Connecting to database...")
	db, err := dbHelper.InitDB("dsn.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected.")
	dbMain = db
	userManager.Init(db)

	log.Println("Starting telegram bot...")
	go tgPlansBot.StartTG(ctx, string(saltValue), dbMain)

	// Wait until the application exits now
	log.Println("Listening for updates.")

	log.Println("Starting web server")
	webserver.StartServer(string(saltValue), db)

}

func setLog() {
	// SET UP LOGGING
	// get directory of app
	if logFile != nil {
		logFile.Close()
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	logFileName = fmt.Sprintf("%s.log", time.Now().Format(time.DateOnly))

	_ = os.MkdirAll(filepath.Join(dir, "logs"), 750)
	logFile, err = os.OpenFile(filepath.Join(dir, "logs", logFileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func checkLog() {
	for {
		// How much time is it until midnight?
		tomorrow := time.Now().Add(time.Hour * 24)
		midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
		d := midnight.Sub(time.Now())
		time.Sleep(d + time.Second)

		// should now be midnight
		// rotate the log.
		setLog()
	}
}
