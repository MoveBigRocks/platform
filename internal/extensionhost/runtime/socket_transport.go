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
	"time"

	"github.com/gin-gonic/gin"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	publicruntime "github.com/movebigrocks/platform/pkg/extensionsruntime"
)

const unixSocketHTTPTimeout = 15 * time.Second

func (r *Registry) DispatchEndpoint(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint, ctx *gin.Context) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		return r.proxyHTTPToUnixSocket(extension, endpoint, ctx)
	default:
		if ctx == nil {
			return fmt.Errorf("request context is required")
		}
		if !r.Dispatch(endpoint.ServiceTarget, ctx) {
			return fmt.Errorf("service target %s is not registered", endpoint.ServiceTarget)
		}
		return nil
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
				publicruntime.HeaderInternalRequest: []string{"true"},
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
		return r.Probe(endpoint.ServiceTarget, http.MethodGet, endpoint.MountPath, nil)
	}
}

func (r *Registry) ConsumeExtension(extension *platformdomain.InstalledExtension, consumer platformdomain.ExtensionEventConsumer, ctx context.Context, data []byte) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		headers := http.Header{
			publicruntime.HeaderInternalRequest: []string{"true"},
		}
		resp, err := r.doUnixSocketRequest(ctx, extension, http.MethodPost, publicruntime.InternalConsumerPath(consumer.ServiceTarget), data, headers)
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
		return r.Consume(consumer.ServiceTarget, ctx, data)
	}
}

func (r *Registry) RunExtensionJob(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob, ctx context.Context) error {
	if r == nil || extension == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	switch strings.TrimSpace(extension.Manifest.Runtime.Protocol) {
	case platformdomain.ExtensionRuntimeProtocolUnixSocketHTTP:
		headers := http.Header{
			publicruntime.HeaderInternalRequest: []string{"true"},
		}
		resp, err := r.doUnixSocketRequest(ctx, extension, http.MethodPost, publicruntime.InternalJobPath(job.ServiceTarget), nil, headers)
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
		return r.RunJob(job.ServiceTarget, ctx)
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
	applyForwardedHeaders(headers, extension, ctx)

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
	socketPath := publicruntime.SocketPath(r.runtimeDir, extension.Manifest.PackageKey())
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("extension runtime socket unavailable for %s: %w", extension.Manifest.PackageKey(), err)
	}

	timeoutCtx := ctx
	if timeoutCtx == nil {
		timeoutCtx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(timeoutCtx, unixSocketHTTPTimeout)
	defer cancel()

	urlPath := requestURI
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	req, err := http.NewRequestWithContext(timeoutCtx, method, "http://unix"+urlPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = cloneHeaders(headers)
	applyRuntimeIdentityHeaders(req.Header, extension)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return client.Do(req)
}

func applyRuntimeIdentityHeaders(headers http.Header, extension *platformdomain.InstalledExtension) {
	if headers == nil || extension == nil {
		return
	}
	headers.Set(publicruntime.HeaderInternalRequest, "true")
	headers.Set(publicruntime.HeaderExtensionID, strings.TrimSpace(extension.ID))
	headers.Set(publicruntime.HeaderExtensionSlug, strings.TrimSpace(extension.Slug))
	headers.Set(publicruntime.HeaderExtensionPackageKey, strings.TrimSpace(extension.Manifest.PackageKey()))
	if raw, ok := marshalHeaderValue(extension.EffectiveConfig().ToMap()); ok {
		headers.Set(publicruntime.HeaderExtensionConfigJSON, raw)
	}
	if workspaceID := strings.TrimSpace(extension.WorkspaceID); workspaceID != "" {
		headers.Set(publicruntime.HeaderWorkspaceID, workspaceID)
	}
}

func applyForwardedHeaders(headers http.Header, extension *platformdomain.InstalledExtension, ctx *gin.Context) {
	applyRuntimeIdentityHeaders(headers, extension)
	if ctx == nil {
		return
	}
	if workspaceID := strings.TrimSpace(ctx.GetString("workspace_id")); workspaceID != "" {
		headers.Set(publicruntime.HeaderWorkspaceID, workspaceID)
	}
	if userID := strings.TrimSpace(ctx.GetString("user_id")); userID != "" {
		headers.Set(publicruntime.HeaderUserID, userID)
	}
	if name, ok := ctx.Get("name"); ok {
		headers.Set(publicruntime.HeaderUserName, fmt.Sprint(name))
	}
	if email, ok := ctx.Get("email"); ok {
		headers.Set(publicruntime.HeaderUserEmail, fmt.Sprint(email))
	}
	if session, ok := ctx.Get("session"); ok {
		if raw, ok := marshalSessionContext(session); ok {
			headers.Set(publicruntime.HeaderSessionContextJSON, raw)
		}
	}
	if raw, ok := marshalHeaderValue(paramsToMap(ctx.Params)); ok {
		headers.Set(publicruntime.HeaderRouteParamsJSON, raw)
	}
	if raw, ok := marshalContextValue(ctx, "admin_extension_nav"); ok {
		headers.Set(publicruntime.HeaderAdminExtensionNavJSON, raw)
	}
	if raw, ok := marshalContextValue(ctx, "admin_extension_widgets"); ok {
		headers.Set(publicruntime.HeaderAdminWidgetsJSON, raw)
	}
	if show, ok := ctx.Get("admin_feature_analytics"); ok {
		headers.Set(publicruntime.HeaderShowAnalytics, fmt.Sprint(show))
	}
	if show, ok := ctx.Get("admin_feature_error_tracking"); ok {
		headers.Set(publicruntime.HeaderShowErrorTracking, fmt.Sprint(show))
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
