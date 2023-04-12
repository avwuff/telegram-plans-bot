package helpers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html"
	"regexp"
)

// Tuple provides a simple 2-item type
type Tuple struct {
	DisplayText string
	Key         string
}

// CalenFeedMD5 combines the ID and the sale to make a hash.
// TODO: This should no longer use MD5.
// Keeping this for compatibility.
func CalenFeedMD5(saltValue string, id int64) string {
	str := fmt.Sprintf("%v%v", id, saltValue)
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// StripHtmlRegex uses a regular expresion to remove HTML tags.
func StripHtmlRegex(s string) string {
	r := regexp.MustCompile(`<.*?>`)
	return html.UnescapeString(r.ReplaceAllString(s, ""))
}
