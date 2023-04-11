package main

import (
	"context"
	"furryplansbot.avbrand.com/dbHelper"
	_ "furryplansbot.avbrand.com/internal/translations"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgPlansBot"
	"furryplansbot.avbrand.com/webserver"
	"log"
	"os"
)

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
	err = dbHelper.InitDB("dsn.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected.")

	log.Println("Starting telegram bot...")
	go tgPlansBot.StartTG(ctx, string(saltValue))

	// Wait until the application exits now
	log.Println("Listening for updates.")

	log.Println("Starting web server")
	webserver.StartServer(string(saltValue))

}
