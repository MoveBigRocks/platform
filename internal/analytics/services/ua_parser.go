package analyticsservices

import (
	"github.com/mssola/useragent"
)

// ParseUA extracts browser name, OS name, and device type from a User-Agent string.
// No version numbers — "Chrome" not "Chrome 122.0".
func ParseUA(uaString string) (browser, os, deviceType string) {
	if uaString == "" {
		return "", "", ""
	}

	ua := useragent.New(uaString)

	browserName, _ := ua.Browser()
	browser = browserName

	os = ua.OS()

	if ua.Mobile() {
		deviceType = "Mobile"
	} else if ua.Bot() {
		deviceType = "Bot"
	} else {
		// useragent library doesn't distinguish tablet; default to Desktop
		deviceType = "Desktop"
	}

	// Detect tablets from OS hints
	if deviceType == "Desktop" {
		if os == "iPad" || os == "iPadOS" {
			deviceType = "Tablet"
		}
	}

	return browser, os, deviceType
}
