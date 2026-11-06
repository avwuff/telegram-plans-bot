package webserver

import "html/template"

// addToCalendarTemplate is checked when the bot starts. Everything put in it is escaped to suit where it goes.
var addToCalendarTemplate = template.Must(template.New("add").Parse(ADDTOCALENDAR_PAGE))

const ADDTOCALENDAR_PAGE = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Title}}</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <!-- The address of this page shouldn't be passed on to the calendar services that it links to. -->
  <meta name="referrer" content="no-referrer">
  <link rel="stylesheet" href="../normalize.css">
  <link rel="stylesheet" href="../skeleton.css">
  <style>
    /* Long labels have to wrap on a phone, which the normal Skeleton buttons don't do. */
    .button { height: auto; padding: 10px 20px; line-height: 1.4; white-space: normal; }
  </style>
</head>
<body>
  <div class="container">
    <div class="row">
      <div class="one-half column" style="margin-top: 10%">
        <h4>{{.Instruction}}</h4>
        <p>
          <a class="button button-primary u-full-width" href="{{.FileURL}}">{{.FileLabel}}</a>
          <a class="button u-full-width" href="{{.GoogleURL}}">Google Calendar</a>
          <a class="button u-full-width" href="{{.OutlookURL}}">Outlook.com</a>
          <a class="button u-full-width" href="{{.Office365URL}}">Microsoft 365</a>
          <a class="button u-full-width" href="{{.YahooURL}}">Yahoo Calendar</a>
        </p>

        <h4>{{.DidYouKnow}}</h4>
        <p>{{.FeedMessage}}</p>
      </div>
    </div>
  </div>
</body>
</html>
`

const NAME_GUESTS_PAGE = `<html>
	<head>
		<script src="https://telegram.org/js/telegram-web-app.js"></script>
	</head>
	<body>
		Hello World!
		<script>
			// info about the user
			//alert(
		</script>
	</body>
</html>
`
