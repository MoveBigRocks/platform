package main

import (
	"net/http"
	"time"

	"github.com/movebigrocks/platform/internal/cliapi"
)

// These function variables form the CLI runtime seam.
// Tests override them to swap in fake clients, HTTP transports, and browser flows
// without having to stand up the whole system.
var (
	loadCLIConfig = cliapi.LoadConfig
	newCLIClient  = cliapi.NewClient
	newHTTPClient = func() *http.Client {
		return &http.Client{Timeout: 15 * time.Second}
	}
	openBrowserURL = openBrowser
)
