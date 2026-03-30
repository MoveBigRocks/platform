package main

import (
	"context"
	"net/http"

	"github.com/movebigrocks/platform/internal/platform/extensionbundle"
)

const (
	envMarketplaceURL   = extensionbundle.EnvMarketplaceURL
	envRegistryToken    = extensionbundle.EnvRegistryToken
	envRegistryUsername = extensionbundle.EnvRegistryUsername
	envRegistryPassword = extensionbundle.EnvRegistryPassword
)

type bundleSourceKind = extensionbundle.SourceKind

const (
	bundleSourceKindLocal       = extensionbundle.SourceKindLocal
	bundleSourceKindHTTP        = extensionbundle.SourceKindHTTP
	bundleSourceKindOCI         = extensionbundle.SourceKindOCI
	bundleSourceKindMarketplace = extensionbundle.SourceKindMarketplace
)

type bundleSourcePayload = extensionbundle.SourcePayload

func readBundleSource(ctx context.Context, source, licenseToken string) (bundleFile, error) {
	payload, err := readBundleSourcePayload(ctx, source, licenseToken)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

func readBundleSourcePayload(ctx context.Context, source, licenseToken string) (bundleSourcePayload, error) {
	return extensionbundle.ReadSource(ctx, source, licenseToken, bundleResolverConfig(ctx))
}

//nolint:unused // pending extension install CLI
func readBundleURLWithHeaders(ctx context.Context, rawURL string, headers map[string]string) (bundleFile, error) {
	payload, err := readBundleURLPayloadWithHeaders(ctx, rawURL, headers, bundleSourceKindHTTP)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

func readBundleURLPayloadWithHeaders(ctx context.Context, rawURL string, headers map[string]string, kind bundleSourceKind) (bundleSourcePayload, error) {
	return extensionbundle.ReadURLPayloadWithHeaders(ctx, rawURL, headers, kind, bundleResolverConfig(ctx))
}

func bundleResolverConfig(ctx context.Context) extensionbundle.ResolverConfig {
	cfg := extensionbundle.DefaultResolverConfigFromEnv()
	cfg.HTTPClient = func() *http.Client {
		return httpClientFromContext(ctx)
	}
	return cfg
}
