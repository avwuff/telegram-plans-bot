package main

import (
	"context"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/dbInterface"
	_ "furryplansbot.avbrand.com/internal/translations"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgPlansBot"
	"furryplansbot.avbrand.com/userManager"
	"furryplansbot.avbrand.com/webserver"
	"log"
	"os"
)

var dbMain dbInterface.DBFeatures

func main() {
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
