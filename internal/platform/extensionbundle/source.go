package extensionbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

const (
	EnvMarketplaceURL   = "MBR_MARKETPLACE_URL"
	EnvRegistryToken    = "MBR_REGISTRY_TOKEN"
	EnvRegistryUsername = "MBR_REGISTRY_USERNAME"
	EnvRegistryPassword = "MBR_REGISTRY_PASSWORD"
)

type SourceKind string

const (
	SourceKindLocal       SourceKind = "local"
	SourceKindHTTP        SourceKind = "http"
	SourceKindOCI         SourceKind = "oci"
	SourceKindMarketplace SourceKind = "marketplace"
)

type File struct {
	Manifest   map[string]any `json:"manifest"`
	Assets     []Asset        `json:"assets"`
	Migrations []Migration    `json:"migrations,omitempty"`
}

type Asset struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	ContentType    string `json:"contentType,omitempty"`
	IsCustomizable bool   `json:"isCustomizable,omitempty"`
}

type Migration struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SourcePayload struct {
	Kind   SourceKind
	Bundle File
	Bytes  []byte
}

type MarketplaceResolution struct {
	BundleURL    string            `json:"bundleURL"`
	OCIReference string            `json:"ociReference"`
	Headers      map[string]string `json:"headers"`
}

type OCIReference struct {
	Insecure   bool
	Registry   string
	Repository string
	Reference  string
}

type ResolverConfig struct {
	MarketplaceURL   string
	RegistryToken    string
	RegistryUsername string
	RegistryPassword string
	HTTPClient       func() *http.Client
}

func DefaultResolverConfigFromEnv() ResolverConfig {
	return ResolverConfig{
		MarketplaceURL:   strings.TrimSpace(os.Getenv(EnvMarketplaceURL)),
		RegistryToken:    strings.TrimSpace(os.Getenv(EnvRegistryToken)),
		RegistryUsername: strings.TrimSpace(os.Getenv(EnvRegistryUsername)),
		RegistryPassword: strings.TrimSpace(os.Getenv(EnvRegistryPassword)),
		HTTPClient: func() *http.Client {
			return &http.Client{}
		},
	}
}

func (c ResolverConfig) withDefaults() ResolverConfig {
	if c.HTTPClient == nil {
		c.HTTPClient = func() *http.Client {
			return &http.Client{}
		}
	}
	return c
}

func DecodeManifest(raw map[string]any) (platformdomain.ExtensionManifest, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return platformdomain.ExtensionManifest{}, err
	}
	var manifest platformdomain.ExtensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return platformdomain.ExtensionManifest{}, err
	}
	manifest.Normalize()
	return manifest, nil
}

func ReadSource(ctx context.Context, source, licenseToken string, cfg ResolverConfig) (SourcePayload, error) {
	return readSourceDepth(ctx, strings.TrimSpace(source), strings.TrimSpace(licenseToken), cfg.withDefaults(), 0)
}

func readSourceDepth(ctx context.Context, source, licenseToken string, cfg ResolverConfig, depth int) (SourcePayload, error) {
	if source == "" {
		return SourcePayload{}, fmt.Errorf("bundle source is required")
	}
	if depth > 4 {
		return SourcePayload{}, fmt.Errorf("bundle source resolution exceeded maximum depth")
	}
	if LocalSourceExists(source) {
		return ReadFilePayload(source)
	}
	if remoteURL, ok := SourceURL(source); ok {
		return ReadURLPayloadWithHeaders(ctx, remoteURL, nil, SourceKindHTTP, cfg)
	}
	if ref, ok, err := ParseOCIReference(source); ok {
		if err != nil {
			return SourcePayload{}, err
		}
		return ReadOCIPayload(ctx, ref, cfg)
	}
	if LooksLikeMarketplaceAlias(source) {
		resolution, err := ResolveMarketplaceAlias(ctx, source, licenseToken, cfg)
		if err != nil {
			return SourcePayload{}, err
		}
		switch {
		case strings.TrimSpace(resolution.OCIReference) != "":
			return readSourceDepth(ctx, resolution.OCIReference, licenseToken, cfg, depth+1)
		case strings.TrimSpace(resolution.BundleURL) != "":
			return ReadURLPayloadWithHeaders(ctx, resolution.BundleURL, resolution.Headers, SourceKindMarketplace, cfg)
		default:
			return SourcePayload{}, fmt.Errorf("marketplace alias %q did not resolve to a bundle source", source)
		}
	}
	return ReadFilePayload(source)
}

func LocalSourceExists(source string) bool {
	cleanPath := filepath.Clean(source)
	if cleanPath == "." && strings.TrimSpace(source) == "" {
		return false
	}
	_, err := os.Stat(cleanPath)
	return err == nil
}

func ResolveMarketplaceAlias(ctx context.Context, alias, licenseToken string, cfg ResolverConfig) (MarketplaceResolution, error) {
	baseURL := strings.TrimSpace(cfg.MarketplaceURL)
	if baseURL == "" {
		return MarketplaceResolution{}, fmt.Errorf("marketplace alias resolution requires %s", EnvMarketplaceURL)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return MarketplaceResolution{}, fmt.Errorf("invalid marketplace URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/resolve"
	query := u.Query()
	query.Set("ref", alias)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return MarketplaceResolution{}, fmt.Errorf("build marketplace request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if licenseToken != "" {
		req.Header.Set("Authorization", "Bearer "+licenseToken)
	}

	resp, err := cfg.HTTPClient().Do(req)
	if err != nil {
		return MarketplaceResolution{}, fmt.Errorf("resolve marketplace alias: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MarketplaceResolution{}, fmt.Errorf("read marketplace response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MarketplaceResolution{}, fmt.Errorf("resolve marketplace alias: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var resolution MarketplaceResolution
	if err := json.Unmarshal(body, &resolution); err != nil {
		return MarketplaceResolution{}, fmt.Errorf("decode marketplace response: %w", err)
	}
	return resolution, nil
}

func ReadFile(path string) (File, error) {
	payload, err := ReadFilePayload(path)
	if err != nil {
		return File{}, err
	}
	return payload.Bundle, nil
}

func ReadFilePayload(path string) (SourcePayload, error) {
	if remoteURL, ok := SourceURL(path); ok {
		return ReadURLPayloadWithHeaders(context.Background(), remoteURL, nil, SourceKindHTTP, DefaultResolverConfigFromEnv())
	}

	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read bundle file: %w", err)
	}
	if info.IsDir() {
		return ReadDirectoryPayload(cleanPath)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read bundle file: %w", err)
	}
	return DecodePayload(data, SourceKindLocal)
}

func ReadURLWithHeaders(ctx context.Context, rawURL string, headers map[string]string, cfg ResolverConfig) (File, error) {
	payload, err := ReadURLPayloadWithHeaders(ctx, rawURL, headers, SourceKindHTTP, cfg)
	if err != nil {
		return File{}, err
	}
	return payload.Bundle, nil
}

func ReadURLPayloadWithHeaders(ctx context.Context, rawURL string, headers map[string]string, kind SourceKind, cfg ResolverConfig) (SourcePayload, error) {
	cfg = cfg.withDefaults()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("build bundle request: %w", err)
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := cfg.HTTPClient().Do(req)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("download bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SourcePayload{}, fmt.Errorf("download bundle: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read bundle response: %w", err)
	}
	return DecodePayload(data, kind)
}

func ReadDirectory(root string) (File, error) {
	payload, err := ReadDirectoryPayload(root)
	if err != nil {
		return File{}, err
	}
	return payload.Bundle, nil
}

func ReadDirectoryPayload(root string) (SourcePayload, error) {
	manifestPath := filepath.Join(root, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read bundle manifest: %w", err)
	}
	manifest, err := ParseJSONObject(manifestBytes)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("decode bundle manifest: %w", err)
	}

	assetsRoot := filepath.Join(root, "assets")
	assets := []Asset{}
	if info, err := os.Stat(assetsRoot); err == nil && info.IsDir() {
		if err := filepath.WalkDir(assetsRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(assetsRoot, path)
			if err != nil {
				return err
			}
			assets = append(assets, Asset{
				Path:        filepath.ToSlash(relative),
				Content:     string(content),
				ContentType: DetectAssetContentType(path, content),
			})
			return nil
		}); err != nil {
			return SourcePayload{}, fmt.Errorf("walk bundle assets: %w", err)
		}
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	migrationsRoot := filepath.Join(root, "migrations")
	migrations := []Migration{}
	if info, err := os.Stat(migrationsRoot); err == nil && info.IsDir() {
		if err := filepath.WalkDir(migrationsRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if strings.ToLower(filepath.Ext(path)) != ".sql" {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(migrationsRoot, path)
			if err != nil {
				return err
			}
			migrations = append(migrations, Migration{
				Path:    filepath.ToSlash(relative),
				Content: string(content),
			})
			return nil
		}); err != nil {
			return SourcePayload{}, fmt.Errorf("walk bundle migrations: %w", err)
		}
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Path < migrations[j].Path
	})

	bundle := File{
		Manifest:   manifest,
		Assets:     assets,
		Migrations: migrations,
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("encode bundle directory payload: %w", err)
	}
	return SourcePayload{
		Kind:   SourceKindLocal,
		Bundle: bundle,
		Bytes:  encoded,
	}, nil
}

func ReadOCI(ctx context.Context, ref OCIReference, cfg ResolverConfig) (File, error) {
	payload, err := ReadOCIPayload(ctx, ref, cfg)
	if err != nil {
		return File{}, err
	}
	return payload.Bundle, nil
}

func ReadOCIPayload(ctx context.Context, ref OCIReference, cfg ResolverConfig) (SourcePayload, error) {
	cfg = cfg.withDefaults()
	client := cfg.HTTPClient()

	manifestURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", ref.scheme(), ref.Registry, ref.Repository, ref.Reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("build OCI manifest request: %w", err)
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.artifact.manifest.v1+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}, ", "))
	resp, err := doRegistryRequest(ctx, client, req, ref.Registry, cfg)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("fetch OCI manifest: %w", err)
	}
	defer resp.Body.Close()

	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read OCI manifest: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SourcePayload{}, fmt.Errorf("fetch OCI manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(manifestBytes)))
	}

	var manifest ociManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return SourcePayload{}, fmt.Errorf("decode OCI manifest: %w", err)
	}
	if len(manifest.Manifests) > 0 {
		return SourcePayload{}, fmt.Errorf("OCI indexes are not supported for extension bundles")
	}

	descriptor, ok := selectBundleDescriptor(manifest)
	if !ok {
		return SourcePayload{}, fmt.Errorf("OCI manifest does not include a bundle blob")
	}

	blobURL := fmt.Sprintf("%s://%s/v2/%s/blobs/%s", ref.scheme(), ref.Registry, ref.Repository, descriptor.Digest)
	blobReq, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("build OCI blob request: %w", err)
	}
	blobResp, err := doRegistryRequest(ctx, client, blobReq, ref.Registry, cfg)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("fetch OCI blob: %w", err)
	}
	defer blobResp.Body.Close()

	blobBytes, err := io.ReadAll(blobResp.Body)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("read OCI blob: %w", err)
	}
	if blobResp.StatusCode < 200 || blobResp.StatusCode >= 300 {
		return SourcePayload{}, fmt.Errorf("fetch OCI blob: status %d: %s", blobResp.StatusCode, strings.TrimSpace(string(blobBytes)))
	}
	return DecodePayload(blobBytes, SourceKindOCI)
}

func DecodeBytes(data []byte) (File, error) {
	payload, err := DecodePayload(data, SourceKindLocal)
	if err != nil {
		return File{}, err
	}
	return payload.Bundle, nil
}

func DecodePayload(data []byte, kind SourceKind) (SourcePayload, error) {
	var bundle File
	if err := json.Unmarshal(data, &bundle); err != nil {
		return SourcePayload{}, fmt.Errorf("decode bundle file: %w", err)
	}
	if len(bundle.Manifest) == 0 {
		return SourcePayload{}, fmt.Errorf("bundle file missing manifest")
	}
	return SourcePayload{
		Kind:   kind,
		Bundle: bundle,
		Bytes:  append([]byte(nil), data...),
	}, nil
}

func SourceURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	return u.String(), true
}

func LooksLikeMarketplaceAlias(source string) bool {
	if source == "" {
		return false
	}
	if _, ok := SourceURL(source); ok {
		return false
	}
	if _, ok, _ := ParseOCIReference(source); ok {
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

func ParseOCIReference(raw string) (OCIReference, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return OCIReference{}, false, nil
	}

	switch {
	case strings.HasPrefix(raw, "oci://"):
		return parseOCIHostReference(strings.TrimPrefix(raw, "oci://"), false)
	case strings.HasPrefix(raw, "oci+http://"):
		u, err := url.Parse(raw)
		if err != nil {
			return OCIReference{}, true, fmt.Errorf("invalid OCI reference: %w", err)
		}
		return parseOCIURLReference(u, true)
	case strings.HasPrefix(raw, "oci+https://"):
		u, err := url.Parse(raw)
		if err != nil {
			return OCIReference{}, true, fmt.Errorf("invalid OCI reference: %w", err)
		}
		return parseOCIURLReference(u, false)
	default:
		firstSlash := strings.Index(raw, "/")
		if firstSlash <= 0 {
			return OCIReference{}, false, nil
		}
		registryHost := raw[:firstSlash]
		if !strings.Contains(registryHost, ".") && !strings.Contains(registryHost, ":") && registryHost != "localhost" {
			return OCIReference{}, false, nil
		}
		return parseOCIHostReference(raw, false)
	}
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

func selectBundleDescriptor(manifest ociManifest) (ociDescriptor, bool) {
	if len(manifest.Blobs) > 0 {
		return manifest.Blobs[0], true
	}
	if len(manifest.Layers) > 0 {
		return manifest.Layers[0], true
	}
	return ociDescriptor{}, false
}

func parseOCIURLReference(u *url.URL, insecure bool) (OCIReference, bool, error) {
	if u == nil || u.Host == "" {
		return OCIReference{}, true, fmt.Errorf("invalid OCI reference")
	}
	reference, err := splitOCIRepositoryReference(strings.TrimPrefix(u.Path, "/"))
	if err != nil {
		return OCIReference{}, true, err
	}
	return OCIReference{
		Insecure:   insecure,
		Registry:   u.Host,
		Repository: reference.Repository,
		Reference:  reference.Reference,
	}, true, nil
}

func parseOCIHostReference(raw string, insecure bool) (OCIReference, bool, error) {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return OCIReference{}, true, fmt.Errorf("invalid OCI reference")
	}
	reference, err := splitOCIRepositoryReference(parts[1])
	if err != nil {
		return OCIReference{}, true, err
	}
	return OCIReference{
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

func (r OCIReference) scheme() string {
	if r.Insecure {
		return "http"
	}
	return "https"
}

func doRegistryRequest(ctx context.Context, client *http.Client, req *http.Request, registryHost string, cfg ResolverConfig) (*http.Response, error) {
	applyRegistryCredentials(req, cfg)

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

	token, err := fetchRegistryBearerToken(ctx, client, challenge, registryHost, cfg)
	if err != nil {
		return nil, err
	}

	retry := req.Clone(ctx)
	applyRegistryCredentials(retry, cfg)
	retry.Header.Set("Authorization", "Bearer "+token)
	return client.Do(retry)
}

func applyRegistryCredentials(req *http.Request, cfg ResolverConfig) {
	token := strings.TrimSpace(cfg.RegistryToken)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	username := strings.TrimSpace(cfg.RegistryUsername)
	password := strings.TrimSpace(cfg.RegistryPassword)
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
}

func fetchRegistryBearerToken(ctx context.Context, client *http.Client, challenge, registryHost string, cfg ResolverConfig) (string, error) {
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
	username := strings.TrimSpace(cfg.RegistryUsername)
	password := strings.TrimSpace(cfg.RegistryPassword)
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
	token := strings.TrimSpace(payload.Token)
	if token == "" {
		token = strings.TrimSpace(payload.AccessToken)
	}
	if token == "" {
		return "", fmt.Errorf("registry token response missing token")
	}
	return token, nil
}

func parseWWWAuthenticate(header string) map[string]string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}
	if space := strings.IndexByte(header, ' '); space >= 0 {
		header = header[space+1:]
	}
	result := make(map[string]string)
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return result
}

func DetectAssetContentType(path string, content []byte) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}
	return http.DetectContentType(content)
}

func ParseJSONObject(data []byte) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode json object: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("config must be a JSON object")
	}
	return value, nil
}
