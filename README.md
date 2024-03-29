# Telegram Event Plan & RSVP Bot

A bot to help plan, organize, disseminate, and coordinate events. Makes life easy for event organizers.

![Example event](imgs/sample1.jpg?raw=true "Example event UI")

## Features 

- Create a new event in less than a minute
- RSVP feature allows guests to indicate if they can attend
- _+1_, _+2_, _Maybe_, and _Can't Make It_ helps count guests
- Attendees don't need to interact with the bot directly
- The bot does not need to be added to a chat to work
- Can be used in group chats or 1-on-1 chats
- Changes to attendees or event info updates everywhere, all at once
- Events can be directly added to your calendar
- An ICAL feed can be imported into your calendar so any event you RSVP to automatically appears there
- Events can optionally be shared to other chats by guests
- Easy-to-use UI for choosing the date and time

### New in version 2.0
- Formatting and emoji in all fields
- Multiple language support
- Time zones
- Faster
- Rewritten in Golang
- Open source!
- Multi-day and multi-hour events
- Include a picture with your event

### The event edit user interface
![Edit UI](imgs/editui.jpg?raw=true "The Edit UI")

# Getting Started

If you want to try the bot, it's available for free right now at 
www.t.me/furryplansbot

Just send the bot a `/start` command and follow the instructions.

# Development

As of version 2.0, this bot is now fully open source!  
You can clone the code and run your own copy, or you can submit pull requests 
and bug reports to this version to bring them to the `@furryplansbot`.

## Changelog
See the [Changelog](changelog.md) for the most recent changes!

## Roadmap

As time allows, I'd like to bring a lot more features to this bot.  
Here's what is on my roadmap so far:
- **Migrate to newer DB:** Right now the bot still runs on my old DB. 
- **More languages and time zones** 
- **Reoccurring events** Support for events that occur more than once
- **Remove hardcoded URLs** Right now, plansbot.avbrand.com is hardcoded in a few places.
- **Attendee management:** Allow the organize to see and remove attendees
- **Better notes editor:** What if adding/editing notes worked in a popup?
- **and more!**

## Environment

To build this bot, you will need to install Go.  Follow the instructions on Go's webpage to get started.
Building the bot is as easy as typing:
```shell
> go build .
```

## Starting your own copy of the bot

A few files need to be set up for this bot to work.  These will be moved into a config file in the future.

#### token.txt 
Place your Telegram Bot token here, as the only content of the file.  
You can get this token from @BotFather on Telegram.  

#### dsn.txt 
Place your database connection string here.  Example:
```
<user>>:<pass>@tcp(<host>:<port>)/<database>?parseTime=true
```

#### salt.txt
For backwards-compatibility reasons, the bot still uses MD5 for hashing.  
One day this will be removed.
In this file, place any value that will be used to salt the MD5 hashes.

## Updating the language files
The Furry Plans Bot uses `gotext` to help provide translations.
1. Run `go generate internal\translations\translations.go`. This will run `gotext` to create the translation files.
2. Look in the `locales` directory.  You will see a new file named `out.gotext.json` for each language.
3. Edit this file and fill in the missing translations
4. Delete the `messages.gotext.json` file in each language folder
5. Rename the `out.gotext.json` file to `messages.gotext.json`
6. Run the `go generate` from step 1 again to import your changes into the code. Delete the `out` files generated this time. 