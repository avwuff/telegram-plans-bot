package translations

// I used this article to figure out how to do this:
// https://www.alexedwards.net/blog/i18n-managing-translations

// Last argument is the list of packages to create translations for
//go:generate gotext -srclang=en-US update -out=catalog.go -lang=en-US,de-DE,fr-CH furryplansbot.avbrand.com/tgPlansBot
