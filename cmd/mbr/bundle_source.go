package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	envMarketplaceURL   = "MBR_MARKETPLACE_URL"
	envRegistryToken    = "MBR_REGISTRY_TOKEN"
	envRegistryUsername = "MBR_REGISTRY_USERNAME"
	envRegistryPassword = "MBR_REGISTRY_PASSWORD"
)

type bundleSourceKind string

const (
	bundleSourceKindLocal       bundleSourceKind = "local"
	bundleSourceKindHTTP        bundleSourceKind = "http"
	bundleSourceKindOCI         bundleSourceKind = "oci"
	bundleSourceKindMarketplace bundleSourceKind = "marketplace"
)

type bundleSourcePayload struct {
	Kind   bundleSourceKind
	Bundle bundleFile
	Bytes  []byte
}

type marketplaceResolution struct {
	BundleURL    string            `json:"bundleURL"`
	OCIReference string            `json:"ociReference"`
	Headers      map[string]string `json:"headers"`
}

type ociReference struct {
	Insecure   bool
	Registry   string
	Repository string
	Reference  string
}

type ociManifest struct {
	MediaType string          `json:"mediaType"`
	Layers    []ociDescriptor `json:"layers"`
	Blobs     []ociDescriptor `json:"blobs"`
	Manifests []ociDescriptor `json:"manifests"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

func readBundleSource(ctx context.Context, source, licenseToken string) (bundleFile, error) {
	payload, err := readBundleSourcePayload(ctx, source, licenseToken)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

func readBundleSourcePayload(ctx context.Context, source, licenseToken string) (bundleSourcePayload, error) {
	return readBundleSourcePayloadDepth(ctx, strings.TrimSpace(source), strings.TrimSpace(licenseToken), 0)
}

func readBundleSourcePayloadDepth(ctx context.Context, source, licenseToken string, depth int) (bundleSourcePayload, error) {
	if source == "" {
		return bundleSourcePayload{}, fmt.Errorf("bundle source is required")
	}
	if depth > 4 {
		return bundleSourcePayload{}, fmt.Errorf("bundle source resolution exceeded maximum depth")
	}
	if localBundleSourceExists(source) {
		return readBundleFilePayload(source)
	}
	if remoteURL, ok := bundleSourceURL(source); ok {
		return readBundleURLPayloadWithHeaders(ctx, remoteURL, nil, bundleSourceKindHTTP)
	}

	if ref, ok, err := parseOCIReference(source); ok {
		if err != nil {
			return bundleSourcePayload{}, err
		}
		return readBundleOCIPayload(ctx, ref)
	}

	if looksLikeMarketplaceAlias(source) {
		resolution, err := resolveMarketplaceAlias(ctx, source, licenseToken)
		if err != nil {
			return bundleSourcePayload{}, err
		}
		switch {
		case strings.TrimSpace(resolution.OCIReference) != "":
			return readBundleSourcePayloadDepth(ctx, resolution.OCIReference, licenseToken, depth+1)
		case strings.TrimSpace(resolution.BundleURL) != "":
			return readBundleURLPayloadWithHeaders(ctx, resolution.BundleURL, resolution.Headers, bundleSourceKindMarketplace)
		default:
			return bundleSourcePayload{}, fmt.Errorf("marketplace alias %q did not resolve to a bundle source", source)
		}
	}

	return readBundleFilePayload(source)
}

func localBundleSourceExists(source string) bool {
	cleanPath := filepath.Clean(source)
	if cleanPath == "." && strings.TrimSpace(source) == "" {
		return false
	}
	_, err := os.Stat(cleanPath)
	return err == nil
}

func resolveMarketplaceAlias(ctx context.Context, alias, licenseToken string) (marketplaceResolution, error) {
	baseURL := strings.TrimSpace(os.Getenv(envMarketplaceURL))
	if baseURL == "" {
		return marketplaceResolution{}, fmt.Errorf("marketplace alias resolution requires %s", envMarketplaceURL)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return marketplaceResolution{}, fmt.Errorf("invalid marketplace URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/resolve"
	query := u.Query()
	query.Set("ref", alias)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return marketplaceResolution{}, fmt.Errorf("build marketplace request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if licenseToken != "" {
		req.Header.Set("Authorization", "Bearer "+licenseToken)
	}

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return marketplaceResolution{}, fmt.Errorf("resolve marketplace alias: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return marketplaceResolution{}, fmt.Errorf("read marketplace response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return marketplaceResolution{}, fmt.Errorf("resolve marketplace alias: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var resolution marketplaceResolution
	if err := json.Unmarshal(body, &resolution); err != nil {
		return marketplaceResolution{}, fmt.Errorf("decode marketplace response: %w", err)
	}
	return resolution, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("build bundle request: %w", err)
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("download bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return bundleSourcePayload{}, fmt.Errorf("download bundle: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read bundle response: %w", err)
	}
	return decodeBundlePayload(data, kind)
}

//nolint:unused // pending extension install CLI
func readBundleOCI(ctx context.Context, ref ociReference) (bundleFile, error) {
	payload, err := readBundleOCIPayload(ctx, ref)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

//nolint:unused // pending extension install CLI
func readBundleOCIPayload(ctx context.Context, ref ociReference) (bundleSourcePayload, error) {
	client := newHTTPClient()

	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", ref.scheme(), ref.Registry, ref.Repository, ref.Reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("build OCI manifest request: %w", err)
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.artifact.manifest.v1+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}, ", "))
	resp, err := doRegistryRequest(ctx, client, req, ref.Registry)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("fetch OCI manifest: %w", err)
	}
	defer resp.Body.Close()

	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read OCI manifest: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return bundleSourcePayload{}, fmt.Errorf("fetch OCI manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(manifestBytes)))
	}

	var manifest ociManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return bundleSourcePayload{}, fmt.Errorf("decode OCI manifest: %w", err)
	}
	if len(manifest.Manifests) > 0 {
		return bundleSourcePayload{}, fmt.Errorf("OCI indexes are not supported for extension bundles")
	}

	descriptor, ok := selectBundleDescriptor(manifest)
	if !ok {
		return bundleSourcePayload{}, fmt.Errorf("OCI manifest does not include a bundle blob")
	}

	blobURL := fmt.Sprintf("%s://%s/v2/%s/blobs/%s", ref.scheme(), ref.Registry, ref.Repository, descriptor.Digest)
	blobReq, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("build OCI blob request: %w", err)
	}
	blobResp, err := doRegistryRequest(ctx, client, blobReq, ref.Registry)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("fetch OCI blob: %w", err)
	}
	defer blobResp.Body.Close()

	blobBytes, err := io.ReadAll(blobResp.Body)
	if err != nil {
		return bundleSourcePayload{}, fmt.Errorf("read OCI blob: %w", err)
	}
	if blobResp.StatusCode < 200 || blobResp.StatusCode >= 300 {
		return bundleSourcePayload{}, fmt.Errorf("fetch OCI blob: status %d: %s", blobResp.StatusCode, strings.TrimSpace(string(blobBytes)))
	}

	return decodeBundlePayload(blobBytes, bundleSourceKindOCI)
}

//nolint:unused // pending extension install CLI
func decodeBundleBytes(data []byte) (bundleFile, error) {
	payload, err := decodeBundlePayload(data, bundleSourceKindLocal)
	if err != nil {
		return bundleFile{}, err
	}
	return payload.Bundle, nil
}

func decodeBundlePayload(data []byte, kind bundleSourceKind) (bundleSourcePayload, error) {
	var bundle bundleFile
	if err := json.Unmarshal(data, &bundle); err != nil {
		return bundleSourcePayload{}, fmt.Errorf("decode bundle file: %w", err)
	}
	if len(bundle.Manifest) == 0 {
		return bundleSourcePayload{}, fmt.Errorf("bundle file missing manifest")
	}
	return bundleSourcePayload{
		Kind:   kind,
		Bundle: bundle,
		Bytes:  append([]byte(nil), data...),
	}, nil
}

func selectBundleDescriptor(manifest ociManifest) (ociDescriptor, bool) {
	if len(manifest.Blobs) > 0 {
		return manifest.Blobs[0], true
	}
	if len(manifest.Layers) > 0 {
		return manifest.Layers[0], true
	}
	return ociDescriptor{}, false
}

func looksLikeMarketplaceAlias(source string) bool {
	if source == "" {
		return false
	}
	if _, ok := bundleSourceURL(source); ok {
		return false
	}
	if _, ok, _ := parseOCIReference(source); ok {
		return false
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") || strings.Contains(source, string(filepath.Separator)) && strings.Contains(source, ".json") {
		return false
	}
	slash := strings.Index(source, "/")
	at := strings.LastIndex(source, "@")
	if slash <= 0 || at <= slash+1 || at == len(source)-1 {
		return false
	}
	namespace := source[:slash]
	if strings.Contains(namespace, ".") || strings.Contains(namespace, ":") {
		return false
	}
	return true
}

func parseOCIReference(raw string) (ociReference, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ociReference{}, false, nil
	}

	switch {
	case strings.HasPrefix(raw, "oci://"):
		return parseOCIHostReference(strings.TrimPrefix(raw, "oci://"), false)
	case strings.HasPrefix(raw, "oci+http://"):
		u, err := url.Parse(raw)
		if err != nil {
			return ociReference{}, true, fmt.Errorf("invalid OCI reference: %w", err)
		}
		return parseOCIURLReference(u, true)
	case strings.HasPrefix(raw, "oci+https://"):
		u, err := url.Parse(raw)
		if err != nil {
			return ociReference{}, true, fmt.Errorf("invalid OCI reference: %w", err)
		}
		return parseOCIURLReference(u, false)
	default:
		firstSlash := strings.Index(raw, "/")
		if firstSlash <= 0 {
			return ociReference{}, false, nil
		}
		registryHost := raw[:firstSlash]
		if !strings.Contains(registryHost, ".") && !strings.Contains(registryHost, ":") && registryHost != "localhost" {
			return ociReference{}, false, nil
		}
		return parseOCIHostReference(raw, false)
	}
}

func parseOCIURLReference(u *url.URL, insecure bool) (ociReference, bool, error) {
	if u == nil || u.Host == "" {
		return ociReference{}, true, fmt.Errorf("invalid OCI reference")
	}
	reference, err := splitOCIRepositoryReference(strings.TrimPrefix(u.Path, "/"))
	if err != nil {
		return ociReference{}, true, err
	}
	return ociReference{
		Insecure:   insecure,
		Registry:   u.Host,
		Repository: reference.Repository,
		Reference:  reference.Reference,
	}, true, nil
}

func parseOCIHostReference(raw string, insecure bool) (ociReference, bool, error) {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return ociReference{}, true, fmt.Errorf("invalid OCI reference")
	}
	reference, err := splitOCIRepositoryReference(parts[1])
	if err != nil {
		return ociReference{}, true, err
	}
	return ociReference{
		Insecure:   insecure,
		Registry:   parts[0],
		Repository: reference.Repository,
		Reference:  reference.Reference,
	}, true, nil
}

type ociRepositoryReference struct {
	Repository string
	Reference  string
}

func splitOCIRepositoryReference(raw string) (ociRepositoryReference, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	if raw == "" {
		return ociRepositoryReference{}, fmt.Errorf("invalid OCI reference")
	}
	if at := strings.LastIndex(raw, "@"); at > 0 {
		return ociRepositoryReference{
			Repository: raw[:at],
			Reference:  raw[at+1:],
		}, nil
	}
	lastSlash := strings.LastIndex(raw, "/")
	lastColon := strings.LastIndex(raw, ":")
	if lastColon > lastSlash {
		return ociRepositoryReference{
			Repository: raw[:lastColon],
			Reference:  raw[lastColon+1:],
		}, nil
	}
	return ociRepositoryReference{
		Repository: raw,
		Reference:  "latest",
	}, nil
}

func (r ociReference) scheme() string {
	if r.Insecure {
		return "http"
	}
	return "https"
}

func doRegistryRequest(ctx context.Context, client *http.Client, req *http.Request, registryHost string) (*http.Response, error) {
	applyRegistryCredentials(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	challenge := strings.TrimSpace(resp.Header.Get("Www-Authenticate"))
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return resp, nil
	}
	_ = resp.Body.Close()

	token, err := fetchRegistryBearerToken(ctx, client, challenge, registryHost)
	if err != nil {
		return nil, err
	}

	retry := req.Clone(ctx)
	applyRegistryCredentials(retry)
	retry.Header.Set("Authorization", "Bearer "+token)
	return client.Do(retry)
}

func applyRegistryCredentials(req *http.Request) {
	token := strings.TrimSpace(os.Getenv(envRegistryToken))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	username := strings.TrimSpace(os.Getenv(envRegistryUsername))
	password := strings.TrimSpace(os.Getenv(envRegistryPassword))
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
}

func fetchRegistryBearerToken(ctx context.Context, client *http.Client, challenge, registryHost string) (string, error) {
	params := parseWWWAuthenticate(challenge)
	realm := strings.TrimSpace(params["realm"])
	if realm == "" {
		return "", fmt.Errorf("registry %s requested bearer auth without a realm", registryHost)
	}

	tokenURL, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid bearer auth realm: %w", err)
	}
	query := tokenURL.Query()
	if service := strings.TrimSpace(params["service"]); service != "" {
		query.Set("service", service)
	}
	if scope := strings.TrimSpace(params["scope"]); scope != "" {
		query.Set("scope", scope)
	}
	tokenURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build registry token request: %w", err)
	}
	username := strings.TrimSpace(os.Getenv(envRegistryUsername))
	password := strings.TrimSpace(os.Getenv(envRegistryPassword))
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch registry bearer token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read registry token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch registry bearer token: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode registry token response: %w", err)
	}
	if strings.TrimSpace(payload.Token) != "" {
		return payload.Token, nil
	}
	if strings.TrimSpace(payload.AccessToken) != "" {
		return payload.AccessToken, nil
	}
	return "", fmt.Errorf("registry token response did not include a token")
}

func parseWWWAuthenticate(value string) map[string]string {
	out := map[string]string{}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return out
	}
	for _, item := range strings.Split(parts[1], ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key, rawValue, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(rawValue), `"`)
	}
	return out
}
