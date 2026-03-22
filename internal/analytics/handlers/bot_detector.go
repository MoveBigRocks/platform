package analyticshandlers

import (
	"strings"
)

// botSubstrings are known bot UA substrings for server-side filtering.
var botSubstrings = []string{
	"bot", "crawler", "spider", "headless", "phantom",
	"scrapy", "wget", "curl", "httpx", "python-requests",
	"go-http-client", "java/", "apache-httpclient",
	"ahrefs", "semrush", "mj12bot", "dotbot", "petalbot",
	"yandexbot", "baiduspider", "sogou", "exabot",
	"facebookexternalhit", "twitterbot", "linkedinbot",
	"slackbot", "telegrambot", "whatsapp", "discordbot",
}

// referrerSpamDomains are known spam referrer domains.
var referrerSpamDomains = []string{
	"semalt.com",
	"buttons-for-website.com",
	"free-social-buttons.com",
	"darodar.com",
	"ilovevitaly.com",
}

// IsBotUA checks if a User-Agent string matches known bot patterns.
func IsBotUA(ua string) bool {
	lower := strings.ToLower(ua)
	for _, sub := range botSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// IsReferrerSpam checks if a referrer URL matches known spam domains.
func IsReferrerSpam(referrer string) bool {
	lower := strings.ToLower(referrer)
	for _, domain := range referrerSpamDomains {
		if strings.Contains(lower, domain) {
			return true
		}
	}
	return false
}
