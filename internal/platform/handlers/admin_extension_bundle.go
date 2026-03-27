package platformhandlers

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

const extensionBundleRootSelector = ".extension-bundle-root"

type parsedAdminExtensionDocument struct {
	Title    string
	HeadHTML template.HTML
	BodyHTML template.HTML
}

// RenderAdminBundlePage wraps bundle-backed admin HTML inside the shared admin shell.
func RenderAdminBundlePage(c *gin.Context, resolved *platformservices.ResolvedExtensionAssetRoute) error {
	pageData, err := BuildAdminBundlePageData(c, resolved)
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "extension_bundle.html", pageData)
	return nil
}

// BuildAdminBundlePageData prepares the shared admin template context for a bundle page.
func BuildAdminBundlePageData(c *gin.Context, resolved *platformservices.ResolvedExtensionAssetRoute) (gin.H, error) {
	document, err := parseAdminExtensionDocument(resolved.Asset.Content)
	if err != nil {
		return nil, err
	}

	activePage, pageTitle, pageSubtitle := resolvedAdminBundleMetadata(resolved)
	data := buildAdminTemplateContext(c, activePage, pageTitle, pageSubtitle)
	data["ExtensionHeadContent"] = document.HeadHTML
	data["ExtensionBodyContent"] = document.BodyHTML
	data["ExtensionDocumentTitle"] = fallbackDocumentTitle(document.Title, pageTitle)
	data["ExtensionSlug"] = resolved.Extension.Slug
	data["ExtensionName"] = resolved.Extension.Name
	data["ExtensionMountPath"] = resolved.MountPath
	return data, nil
}

func resolvedAdminBundleMetadata(resolved *platformservices.ResolvedExtensionAssetRoute) (activePage, pageTitle, pageSubtitle string) {
	activePage = strings.TrimSpace(resolved.Extension.Slug)
	pageTitle = strings.TrimSpace(resolved.Extension.Name)
	pageSubtitle = firstSentence(resolved.Extension.Manifest.Description)

	matchedEndpoints := make(map[string]struct{})
	for _, endpoint := range resolved.Extension.Manifest.Endpoints {
		if endpoint.Class != platformdomain.ExtensionEndpointClassAdminPage {
			continue
		}
		if strings.TrimSpace(endpoint.MountPath) == strings.TrimSpace(resolved.MountPath) {
			matchedEndpoints[endpoint.Name] = struct{}{}
		}
	}

	for _, item := range resolved.Extension.Manifest.AdminNavigation {
		if _, ok := matchedEndpoints[item.Endpoint]; !ok {
			continue
		}
		if trimmed := strings.TrimSpace(item.ActivePage); trimmed != "" {
			activePage = trimmed
		}
		break
	}

	if pageTitle == "" {
		pageTitle = "Extension"
	}
	return activePage, pageTitle, pageSubtitle
}

func fallbackDocumentTitle(documentTitle, pageTitle string) string {
	if trimmed := strings.TrimSpace(documentTitle); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(pageTitle); trimmed != "" {
		return trimmed
	}
	return "Extension"
}

func firstSentence(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if index := strings.Index(trimmed, "."); index >= 0 {
		return strings.TrimSpace(trimmed[:index+1])
	}
	return trimmed
}

func parseAdminExtensionDocument(content []byte) (parsedAdminExtensionDocument, error) {
	trimmedContent := bytes.TrimSpace(content)
	if len(trimmedContent) == 0 {
		return parsedAdminExtensionDocument{}, nil
	}

	root, _ := html.Parse(bytes.NewReader(trimmedContent))
	if root == nil {
		return parsedAdminExtensionDocument{
			BodyHTML: template.HTML(trimmedContent),
		}, nil
	}

	head := findFirstElement(root, "head")
	body := findFirstElement(root, "body")

	document := parsedAdminExtensionDocument{
		Title: extractTitle(head),
	}

	if head != nil {
		var buf bytes.Buffer
		for child := head.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode && strings.TrimSpace(child.Data) == "" {
				continue
			}
			if child.Type == html.ElementNode && child.Data == "title" {
				continue
			}
			clone := cloneHTMLNode(child)
			scopeExtensionStyles(clone)
			_ = html.Render(&buf, clone)
		}
		document.HeadHTML = template.HTML(strings.TrimSpace(buf.String()))
	}

	if body != nil {
		var buf bytes.Buffer
		for child := body.FirstChild; child != nil; child = child.NextSibling {
			clone := cloneHTMLNode(child)
			scopeExtensionStyles(clone)
			_ = html.Render(&buf, clone)
		}
		document.BodyHTML = template.HTML(strings.TrimSpace(buf.String()))
	} else {
		document.BodyHTML = template.HTML(trimmedContent)
	}

	return document, nil
}

func findFirstElement(node *html.Node, tag string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && node.Data == tag {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func extractTitle(head *html.Node) string {
	if head == nil {
		return ""
	}
	titleNode := findFirstElement(head, "title")
	if titleNode == nil {
		return ""
	}
	var buf strings.Builder
	for child := titleNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			buf.WriteString(child.Data)
		}
	}
	return strings.TrimSpace(buf.String())
}

func cloneHTMLNode(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}
	clone := &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
		Attr:      append([]html.Attribute(nil), node.Attr...),
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		clonedChild := cloneHTMLNode(child)
		if clonedChild != nil {
			clone.AppendChild(clonedChild)
		}
	}
	return clone
}

func scopeExtensionStyles(node *html.Node) {
	if node == nil {
		return
	}
	if node.Type == html.ElementNode && node.Data == "style" {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				child.Data = prefixCSSSelectors(child.Data, extensionBundleRootSelector)
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		scopeExtensionStyles(child)
	}
}

func prefixCSSSelectors(css, scope string) string {
	var out strings.Builder
	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open == -1 {
			out.WriteString(css)
			break
		}

		selectors := css[:open]
		body, rest, ok := consumeCSSBlock(css[open+1:])
		if !ok {
			out.WriteString(css)
			break
		}

		trimmedSelectors := strings.TrimSpace(selectors)
		switch {
		case trimmedSelectors == "":
			out.WriteString(selectors)
			out.WriteByte('{')
			out.WriteString(body)
			out.WriteByte('}')
		case strings.HasPrefix(trimmedSelectors, "@media"),
			strings.HasPrefix(trimmedSelectors, "@supports"),
			strings.HasPrefix(trimmedSelectors, "@container"),
			strings.HasPrefix(trimmedSelectors, "@layer"),
			strings.HasPrefix(trimmedSelectors, "@document"):
			out.WriteString(selectors)
			out.WriteByte('{')
			out.WriteString(prefixCSSSelectors(body, scope))
			out.WriteByte('}')
		case strings.HasPrefix(trimmedSelectors, "@"):
			out.WriteString(selectors)
			out.WriteByte('{')
			out.WriteString(body)
			out.WriteByte('}')
		default:
			out.WriteString(prefixSelectorList(selectors, scope))
			out.WriteByte('{')
			out.WriteString(body)
			out.WriteByte('}')
		}

		css = rest
	}
	return out.String()
}

func consumeCSSBlock(input string) (body string, rest string, ok bool) {
	depth := 1
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return input[:i], input[i+1:], true
			}
		}
	}
	return "", "", false
}

func prefixSelectorList(selectors, scope string) string {
	parts := strings.Split(selectors, ",")
	for index, selector := range parts {
		trimmed := strings.TrimSpace(selector)
		if trimmed == "" {
			continue
		}
		rewritten := strings.NewReplacer(
			":root", scope,
			"html", scope,
			"body", scope,
		).Replace(trimmed)
		if !strings.Contains(rewritten, scope) {
			rewritten = scope + " " + rewritten
		}
		parts[index] = strings.Replace(selector, trimmed, rewritten, 1)
	}
	return strings.Join(parts, ",")
}
