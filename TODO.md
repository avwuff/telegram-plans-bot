# TODO

## Generate calendar feed time zones locally
`SetTimeZone` in `webserver/server.go` downloads each zone's `VTIMEZONE` from tzurl.org on every feed request.
Generate them from Go's time zone data instead, so the feeds don't depend on that site.

**Approach:** for each zone in a feed, use `ZoneBounds()` to walk the offset changes, from the period in effect at the
earliest event until after the latest event ends. Write one `STANDARD` or `DAYLIGHT` block per change, using `IsDST()` to choose.

**Watch out for:**
- `ZoneBounds()` reports a boundary at January 1 even when the offset doesn't change, so skip those.
- Zones with no changes (`Asia/Tokyo`, `UTC`) have zero-value bounds and need a single fixed block.
- `DTSTART` is the local time before the change, so a change at 09:00 UTC from -0700 is `020000`.
- The vendored `golang-ical` has `AddStandard()` but no `AddDaylight()`.

**Test in Outlook, Google and Apple Calendar.** The current code uses tzurl's Outlook-specific files, and older versions of
Outlook may prefer repeating rules, which Go doesn't expose.

**Fallback:** write all times in UTC with `SetStartAt` and `SetEndAt`. Events would still appear at the correct moment, but
would lose their own time zone, which was added deliberately in `c336642`.
