package main

import (
	"context"
	"furryplansbot.avbrand.com/dbHelper"
	_ "furryplansbot.avbrand.com/internal/translations"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgPlansBot"
	"log"
	"time"
)

func main() {
	log.Println("== Furry Plans Bot Startup ==")
	// Initialize the language list -- this must be called here since the translations package is now initialized.
	localizer.InitLang()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Connecting to database...")
	err := dbHelper.InitDB("dsn.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected.")

	log.Println("Starting telegram bot...")
	go tgPlansBot.StartTG(ctx)

	// Wait until the application exits now
	log.Println("Listening for updates.")
	for {
		// TODO replace this
		time.Sleep(time.Second)
	}

}
