package main

import (
	"context"
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

type httpClientFactoryContextKey struct{}

func contextWithHTTPClientFactory(ctx context.Context, factory func() *http.Client) context.Context {
	if factory == nil {
		return ctx
	}
	return context.WithValue(ctx, httpClientFactoryContextKey{}, factory)
}

func httpClientFactoryFromContext(ctx context.Context) func() *http.Client {
	if ctx != nil {
		if factory, ok := ctx.Value(httpClientFactoryContextKey{}).(func() *http.Client); ok && factory != nil {
			return factory
		}
	}
	return newHTTPClient
}

func httpClientFromContext(ctx context.Context) *http.Client {
	return httpClientFactoryFromContext(ctx)()
}
