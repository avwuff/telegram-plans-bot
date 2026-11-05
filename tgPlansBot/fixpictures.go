package tgPlansBot

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
)

// This file is TEMPORARY.  It moves the old pictures that are stored on disk over to Telegram.
// Once that's done, remove this file, its test, and the call to initFixPicturesCommands.

const (
	// Telegram allows about 20 messages a minute to the same chat.
	fixPicturesDelay = 3100 * time.Millisecond

	// How many times to retry a picture that is being rate limited.
	fixPicturesRetries = 3

	// Stop after this many failures in a row, since it probably means the channel is wrong.
	fixPicturesMaxFailuresInARow = 5

	// How many event IDs to list in the summary message.
	fixPicturesMaxListedIDs = 50
)

var errPictureFileMissing = errors.New("picture file is missing from disk")

var fixPicturesRunning atomic.Bool

// fixPicturesSleep can be replaced by the tests.
var fixPicturesSleep = time.Sleep

func (tgp *TGPlansBot) initFixPicturesCommands() {
	tgp.cmds.Add(tgCommands.Command{Command: "/fixpictures", Handler: tgp.fixPictures, Private: true})
}

// fixPictures uploads the pictures that haven't been moved to Telegram yet, so it is safe to run more than once.
// Uploads have to be sent to a chat, so the command takes the ID of a channel to use: /fixpictures -1001234567890
func (tgp *TGPlansBot) fixPictures(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Only let the authorized user use this function.
	if msg.Chat.ID != limitToUser {
		tgp.quickReply(msg, "Sorry, but this function is not available.")
		return
	}

	channelId, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || channelId == 0 {
		tgp.quickReply(msg, "Please include the ID of the channel to upload to, for example:\n/fixpictures -1001234567890")
		return
	}

	if !fixPicturesRunning.CompareAndSwap(false, true) {
		tgp.quickReply(msg, "Already fixing pictures, please wait for it to finish.")
		return
	}

	events, err := tgp.db.EventsMissingPictureFileID()
	if err != nil {
		fixPicturesRunning.Store(false)
		tgp.quickReply(msg, err.Error())
		return
	}

	tgp.quickReply(msg, fmt.Sprintf("Checking %v pictures. Pictures not on this server are skipped. This will take up to %v.", len(events), (time.Duration(len(events))*fixPicturesDelay).Round(time.Minute)))

	// This can take a while, and we don't want to stop the bot from handling other updates.
	go func() {
		defer fixPicturesRunning.Store(false)
		tgp.quickReply(msg, tgp.uploadPictures(channelId, events))
	}()
}

// uploadPictures uploads each of the pictures, and returns a summary of how it went.
func (tgp *TGPlansBot) uploadPictures(channelId int64, events []dbInterface.DBEvent) string {
	fixed := 0
	failuresInARow := 0
	var missing, failed []string
	needsDelay := false

	for i, event := range events {
		if needsDelay {
			fixPicturesSleep(fixPicturesDelay)
		}

		err := tgp.uploadPicture(channelId, event)

		// A file that isn't on this server is skipped without contacting Telegram, so there's no need to wait after it.
		// It also isn't marked as done, so the server that does have the file can still upload it.
		needsDelay = !errors.Is(err, errPictureFileMissing)

		switch {
		case err == nil:
			fixed++
			failuresInARow = 0
		case errors.Is(err, errPictureFileMissing):
			log.Printf("fixpictures: event %v: %v\n", event.ID(), err)
			missing = append(missing, fmt.Sprint(event.ID()))
		default:
			log.Printf("fixpictures: event %v: %v\n", event.ID(), err)
			failed = append(failed, fmt.Sprint(event.ID()))
			failuresInARow++
		}

		if failuresInARow >= fixPicturesMaxFailuresInARow {
			return fixPicturesSummary(fixed, missing, failed) +
				fmt.Sprintf("\n\nStopped after %v failures in a row, with %v events not tried. Is the bot an admin of the channel? Last error: %v", failuresInARow, len(events)-i-1, err)
		}
	}
	return fixPicturesSummary(fixed, missing, failed)
}

func fixPicturesSummary(fixed int, missing []string, failed []string) string {
	t := fmt.Sprintf("Uploaded %v pictures.", fixed)
	if len(missing) > 0 {
		t += fmt.Sprintf("\n\nFile missing from disk for %v events: %v", len(missing), fixPicturesIDList(missing))
	}
	if len(failed) > 0 {
		t += fmt.Sprintf("\n\nFailed for %v events: %v", len(failed), fixPicturesIDList(failed))
	}
	return t
}

// fixPicturesIDList keeps the list short enough to fit in a message. All the IDs are in the log.
func fixPicturesIDList(ids []string) string {
	if len(ids) <= fixPicturesMaxListedIDs {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%v and %v more (see the log)", strings.Join(ids[:fixPicturesMaxListedIDs], ", "), len(ids)-fixPicturesMaxListedIDs)
}

// uploadPicture uploads the picture for one event from disk, and stores the file ID Telegram gives it.
func (tgp *TGPlansBot) uploadPicture(channelId int64, event dbInterface.DBEvent) error {
	file, err := pictureCachePath(event.PictureURL())
	if err != nil {
		return err
	}
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("%w: %v", errPictureFileMissing, file)
	}

	photo := tgbotapi.NewPhoto(channelId, tgbotapi.FilePath(file))
	photo.DisableNotification = true

	sent, err := tgp.sendPictureWithRetry(photo)
	if err != nil {
		return err
	}

	fileID := largestPhotoFileID(sent.Photo)
	if fileID == "" {
		return errors.New("telegram didn't return a photo")
	}
	return event.SetPictureFileID(fileID)
}

// sendPictureWithRetry sends the picture, waiting and trying again if Telegram asks us to slow down.
func (tgp *TGPlansBot) sendPictureWithRetry(photo tgbotapi.PhotoConfig) (tgbotapi.Message, error) {
	for attempt := 0; ; attempt++ {
		sent, err := tgp.tg.Send(photo)

		var tgErr *tgbotapi.Error
		if err == nil || attempt >= fixPicturesRetries || !errors.As(err, &tgErr) || tgErr.RetryAfter == 0 {
			return sent, err
		}
		fixPicturesSleep(time.Duration(tgErr.RetryAfter) * time.Second)
	}
}

// pictureCachePath finds where on disk the file for an old picture URL is stored.
func pictureCachePath(pictureURL string) (string, error) {
	u, err := url.Parse(pictureURL)
	if err != nil {
		return "", fmt.Errorf("invalid picture URL %v: %w", pictureURL, err)
	}
	// Only the name of the file is used, so this can't point to anywhere outside the cache.
	name := path.Base(u.Path)
	if name == "." || name == "/" || name == ".." {
		return "", fmt.Errorf("invalid picture URL %v", pictureURL)
	}
	return filepath.Join("html", "cache", name), nil
}
