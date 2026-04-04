package extensionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimeproto"
	"github.com/movebigrocks/platform/internal/extensionhost/hostapi"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

const unixSocketHTTPTimeout = 15 * time.Second

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (r *cancelOnCloseReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil {
		r.once.Do(r.cancel)
	}
	return n, err
}

func (r *cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.once.Do(r.cancel)
	return err
}

func (r *Registry) DispatchEndpoint(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint, ctx *gin.Context) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		return r.proxyHTTPToUnixSocket(extension, endpoint, ctx)
	default:
		return fmt.Errorf("runtime protocol %s is not supported", strings.TrimSpace(extension.Manifest.Runtime.Protocol))
	}
}

func (r *Registry) ProbeEndpoint(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint) (ProbeResult, error) {
	if r == nil || extension == nil {
		return ProbeResult{}, fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		resp, err := r.doUnixSocketRequest(
			context.Background(),
			extension,
			http.MethodGet,
			endpoint.MountPath,
			nil,
			http.Header{
				runtimeproto.HeaderInternalRequest: []string{"true"},
			},
		)
		if err != nil {
			return ProbeResult{}, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ProbeResult{}, err
		}
		return ProbeResult{StatusCode: resp.StatusCode, Body: body}, nil
	default:
		return ProbeResult{}, fmt.Errorf("runtime protocol %s is not supported", strings.TrimSpace(extension.Manifest.Runtime.Protocol))
	}
}

func (r *Registry) ConsumeExtension(extension *platformdomain.InstalledExtension, consumer platformdomain.ExtensionEventConsumer, ctx context.Context, data []byte) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		headers := http.Header{
			runtimeproto.HeaderInternalRequest: []string{"true"},
		}
		resp, err := r.doUnixSocketRequest(ctx, extension, http.MethodPost, runtimeproto.InternalConsumerPath(consumer.ServiceTarget), data, headers)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("consumer target %s returned HTTP %d: %s", consumer.ServiceTarget, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil
	default:
		return fmt.Errorf("runtime protocol %s is not supported", strings.TrimSpace(extension.Manifest.Runtime.Protocol))
	}
}

func (r *Registry) RunExtensionJob(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob, ctx context.Context) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		headers := http.Header{
			runtimeproto.HeaderInternalRequest: []string{"true"},
		}
		resp, err := r.doUnixSocketRequest(ctx, extension, http.MethodPost, runtimeproto.InternalJobPath(job.ServiceTarget), nil, headers)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("job target %s returned HTTP %d: %s", job.ServiceTarget, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil
	default:
		return fmt.Errorf("runtime protocol %s is not supported", strings.TrimSpace(extension.Manifest.Runtime.Protocol))
	}
}

func (r *Registry) proxyHTTPToUnixSocket(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint, ctx *gin.Context) error {
	if ctx == nil || ctx.Request == nil {
		return fmt.Errorf("request context is required")
	}

	var body []byte
	if ctx.Request.Body != nil {
		payload, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			return fmt.Errorf("read proxied request body: %w", err)
		}
		body = payload
	}

	headers := cloneHeaders(ctx.Request.Header)
	applyForwardedHeaders(headers, extension, ctx, r.hostTokenSecret)

	resp, err := r.doUnixSocketRequest(
		ctx.Request.Context(),
		extension,
		ctx.Request.Method,
		ctx.Request.URL.RequestURI(),
		body,
		headers,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	copyResponseHeaders(ctx.Writer.Header(), resp.Header)
	ctx.Status(resp.StatusCode)
	if _, err := io.Copy(ctx.Writer, resp.Body); err != nil {
		return fmt.Errorf("copy proxied response: %w", err)
	}
	return nil
}

func (r *Registry) doUnixSocketRequest(
	ctx context.Context,
	extension *platformdomain.InstalledExtension,
	method string,
	requestURI string,
	body []byte,
	headers http.Header,
) (*http.Response, error) {
	if extension == nil {
		return nil, fmt.Errorf("extension is required")
	}
	socketPath := runtimeproto.SocketPath(r.runtimeDir, extension.Manifest.PackageKey())
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("extension runtime socket unavailable for %s: %w", extension.Manifest.PackageKey(), err)
	}

	timeoutCtx := ctx
	if timeoutCtx == nil {
		timeoutCtx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(timeoutCtx, unixSocketHTTPTimeout)

	urlPath := requestURI
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	req, err := http.NewRequestWithContext(timeoutCtx, method, "http://unix"+urlPath, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header = cloneHeaders(headers)
	applyRuntimeIdentityHeaders(req.Header, extension)
	applyRuntimeHostAuthHeader(req.Header, extension, r.hostTokenSecret)
	applyRuntimeBaseURLHeaders(req.Header, r.publicBaseURL, r.adminBaseURL, r.apiBaseURL)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnCloseReadCloser{
		ReadCloser: resp.Body,
		cancel:     cancel,
	}
	return resp, nil
}

func applyRuntimeIdentityHeaders(headers http.Header, extension *platformdomain.InstalledExtension) {
	if headers == nil || extension == nil {
		return
	}
	headers.Set(runtimeproto.HeaderInternalRequest, "true")
	headers.Set(runtimeproto.HeaderExtensionID, strings.TrimSpace(extension.ID))
	headers.Set(runtimeproto.HeaderExtensionSlug, strings.TrimSpace(extension.Slug))
	headers.Set(runtimeproto.HeaderExtensionPackageKey, strings.TrimSpace(extension.Manifest.PackageKey()))
	if raw, ok := marshalHeaderValue(extension.EffectiveConfig().ToMap()); ok {
		headers.Set(runtimeproto.HeaderExtensionConfigJSON, raw)
	}
	if workspaceID := strings.TrimSpace(extension.WorkspaceID); workspaceID != "" {
		headers.Set(runtimeproto.HeaderWorkspaceID, workspaceID)
	}
}

func applyRuntimeHostAuthHeader(headers http.Header, extension *platformdomain.InstalledExtension, secret string) {
	if headers == nil || extension == nil || strings.TrimSpace(secret) == "" {
		return
	}
	token, err := hostapi.IssueToken(secret, extension, hostapi.DefaultTokenTTL)
	if err != nil {
		return
	}
	headers.Set(runtimeproto.HeaderHostToken, token)
}

func applyRuntimeBaseURLHeaders(headers http.Header, publicBaseURL, adminBaseURL, apiBaseURL string) {
	if headers == nil {
		return
	}
	if value := strings.TrimSpace(publicBaseURL); value != "" {
		headers.Set(runtimeproto.HeaderPublicBaseURL, value)
	}
	if value := strings.TrimSpace(adminBaseURL); value != "" {
		headers.Set(runtimeproto.HeaderAdminBaseURL, value)
	}
	if value := strings.TrimSpace(apiBaseURL); value != "" {
		headers.Set(runtimeproto.HeaderAPIBaseURL, value)
	}
}

func applyForwardedHeaders(headers http.Header, extension *platformdomain.InstalledExtension, ctx *gin.Context, hostTokenSecret string) {
	applyRuntimeIdentityHeaders(headers, extension)
	applyRuntimeHostAuthHeader(headers, extension, hostTokenSecret)
	if ctx == nil {
		return
	}
	if ctx.Request != nil {
		if host := strings.TrimSpace(ctx.Request.Host); host != "" {
			headers.Set("X-Forwarded-Host", host)
		}
		proto := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto"))
		if proto == "" {
			if ctx.Request.TLS != nil {
				proto = "https"
			} else {
				proto = "http"
			}
		}
		headers.Set("X-Forwarded-Proto", proto)
	}
	if workspaceID := strings.TrimSpace(ctx.GetString("workspace_id")); workspaceID != "" {
		headers.Set(runtimeproto.HeaderWorkspaceID, workspaceID)
	}
	if userID := strings.TrimSpace(ctx.GetString("user_id")); userID != "" {
		headers.Set(runtimeproto.HeaderUserID, userID)
	}
	if name, ok := ctx.Get("name"); ok {
		headers.Set(runtimeproto.HeaderUserName, fmt.Sprint(name))
	}
	if email, ok := ctx.Get("email"); ok {
		headers.Set(runtimeproto.HeaderUserEmail, fmt.Sprint(email))
	}
	if session, ok := ctx.Get("session"); ok {
		if raw, ok := marshalSessionContext(session); ok {
			headers.Set(runtimeproto.HeaderSessionContextJSON, raw)
		}
	}
	if raw, ok := marshalHeaderValue(paramsToMap(ctx.Params)); ok {
		headers.Set(runtimeproto.HeaderRouteParamsJSON, raw)
	}
	if raw, ok := marshalContextValue(ctx, "admin_extension_nav"); ok {
		headers.Set(runtimeproto.HeaderAdminExtensionNavJSON, raw)
	}
	if raw, ok := marshalContextValue(ctx, "admin_extension_widgets"); ok {
		headers.Set(runtimeproto.HeaderAdminWidgetsJSON, raw)
	}
	if show, ok := ctx.Get("admin_feature_analytics"); ok {
		headers.Set(runtimeproto.HeaderShowAnalytics, fmt.Sprint(show))
	}
	if show, ok := ctx.Get("admin_feature_error_tracking"); ok {
		headers.Set(runtimeproto.HeaderShowErrorTracking, fmt.Sprint(show))
	}
}

func marshalContextValue(ctx *gin.Context, key string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Get(key)
	if !ok {
		return "", false
	}
	return marshalHeaderValue(value)
}

func marshalHeaderValue(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func marshalSessionContext(value interface{}) (string, bool) {
	session, ok := value.(*platformdomain.Session)
	if !ok || session == nil {
		return "", false
	}
	return marshalHeaderValue(session.CurrentContext)
}

func paramsToMap(params gin.Params) map[string]string {
	if len(params) == 0 {
		return nil
	}
	values := make(map[string]string, len(params))
	for _, param := range params {
		values[param.Key] = param.Value
	}
	return values
}

func cloneHeaders(headers http.Header) http.Header {
	if headers == nil {
		return http.Header{}
	}
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func copyResponseHeaders(dst, src http.Header) {
	for key := range dst {
		dst.Del(key)
	}
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
}
