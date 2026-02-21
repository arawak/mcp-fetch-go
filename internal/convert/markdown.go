package convert

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"golang.org/x/net/html"
)

// noiseElements are element names whose entire subtree is removed before
// conversion, regardless of where they appear in the document.
var noiseElements = map[string]bool{
	"nav":      true,
	"header":   true,
	"footer":   true,
	"aside":    true,
	"form":     true,
	"svg":      true,
	"script":   true,
	"style":    true,
	"iframe":   true,
	"noscript": true,
	"button":   true,
	"img":      true,
}

// noiseCustomElements are specific custom element (web component) names whose
// entire subtree should be removed. These contain no readable content.
var noiseCustomElements = map[string]bool{
	// Reddit: loading/shimmer placeholders and pure UI chrome
	"faceplate-loader":                  true,
	"faceplate-shimmer":                 true,
	"faceplate-tracker":                 true,
	"faceplate-screen-reader-content":   true,
	"faceplate-perfmark":                true,
	"faceplate-perfmetric-collector":    true,
	"faceplate-server-session":          true,
	"faceplate-dropdown-menu":           true,
	"shreddit-subreddit-header":         true,
	"shreddit-status-icons":             true,
	"shreddit-post-overflow-menu":       true,
	"shreddit-title":                    true,
	"shreddit-loading":                  true,
	"shreddit-media-lightbox":           true,
	"shreddit-page-meta":                true,
	"shreddit-app-attrs":                true,
	"shreddit-async-loader":             true,
	"shreddit-good-visit-tracker":       true,
	"shreddit-good-visit-tracker-attrs": true,
	"shreddit-recent-communities-data":  true,
	"shreddit-screenview-data":          true,
	"shreddit-activated-feature-meta":   true,
	"shreddit-distinguished-post-tags":  true,
	// Generic icon component libraries
	"icon-caret-down": true,
	"icon-caret-up":   true,
	"icon-upvote":     true,
	"icon-downvote":   true,
	"icon-share":      true,
	"icon-more":       true,
	// GitHub tooltip/popover elements (sr-only text that leaks into output)
	"tool-tip":       true,
	"action-menu":    true,
	"copy-button":    true,
	"clipboard-copy": true,
}

// noiseRoles are ARIA role values that identify non-content regions.
var noiseRoles = map[string]bool{
	"navigation":    true,
	"banner":        true,
	"contentinfo":   true,
	"complementary": true,
	"search":        true,
	"button":        true,
	"menuitem":      true,
	"menubar":       true,
	"toolbar":       true,
	"dialog":        true,
	"alertdialog":   true,
}

// noiseClassTokens are substrings that, when found in a node's class attribute,
// mark the node as non-content. Checked case-insensitively.
var noiseClassTokens = []string{
	"sidebar",
	"breadcrumb",
	"cookie",
	"advertisement",
	"ad-",
	" ad ",
	"menu",
	"navbar",
	"toc",
	"table-of-contents",
}

// multiBlank collapses three or more consecutive blank lines into two.
var multiBlank = regexp.MustCompile(`\n{3,}`)

// mdxTag matches standalone PascalCase MDX component tag lines (opening, closing, or self-closing).
var mdxTag = regexp.MustCompile(`(?m)^\s*</?[A-Z]\w[^>]*>\s*$`)

// mdxSelfClosing matches standalone self-closing PascalCase MDX component lines.
var mdxSelfClosing = regexp.MustCompile(`(?m)^\s*<[A-Z]\w[^/]*/>\s*$`)

// mdxComment matches JSX-style block comments {/* ... */}.
var mdxComment = regexp.MustCompile(`(?s)\{/\*.*?\*/\}`)

// attrVal returns the value of the named attribute for node n, or "".
func attrVal(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// isCustomElement reports whether the tag name is a web component (contains a hyphen).
func isCustomElement(tag string) bool {
	return strings.ContainsRune(tag, '-')
}

// isNoise reports whether an element node should be stripped entirely.
func isNoise(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if noiseElements[n.Data] {
		return true
	}
	if noiseCustomElements[n.Data] {
		return true
	}
	if strings.ToLower(strings.TrimSpace(attrVal(n, "aria-hidden"))) == "true" {
		return true
	}
	role := strings.ToLower(strings.TrimSpace(attrVal(n, "role")))
	if noiseRoles[role] {
		return true
	}
	// Strip <a> tags that are JS-only actions or auth-gate prompts.
	if n.Data == "a" {
		href := strings.TrimSpace(attrVal(n, "href"))
		if href == "#" || href == "" ||
			strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "javascript ") ||
			strings.HasPrefix(href, "/login?") ||
			strings.HasPrefix(href, "/signin?") ||
			strings.HasPrefix(href, "/auth/") {
			return true
		}
	}
	class := strings.ToLower(attrVal(n, "class"))
	for _, token := range noiseClassTokens {
		if strings.Contains(class, token) {
			return true
		}
	}
	return false
}

// unwrapCustomElements replaces unknown custom elements with their children,
// making them transparent to the Markdown converter. Known noise custom
// elements are left for stripNoise to remove entirely.
func unwrapCustomElements(n *html.Node) {
	var toUnwrap []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode &&
			isCustomElement(node.Data) &&
			!noiseCustomElements[node.Data] &&
			!noiseElements[node.Data] {
			toUnwrap = append(toUnwrap, node)
			// Still recurse into children in case they have custom elements.
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	// Process in reverse so children are unwrapped before parents.
	for i := len(toUnwrap) - 1; i >= 0; i-- {
		node := toUnwrap[i]
		parent := node.Parent
		if parent == nil {
			continue
		}
		// Move all children of the custom element to before it in the parent.
		for node.FirstChild != nil {
			child := node.FirstChild
			node.RemoveChild(child)
			parent.InsertBefore(child, node)
		}
		parent.RemoveChild(node)
	}
}

// stripNoise removes noise elements from the parsed HTML tree in place.
func stripNoise(n *html.Node) {
	var toRemove []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if isNoise(node) {
			toRemove = append(toRemove, node)
			return // don't recurse into nodes we're removing
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	for _, node := range toRemove {
		node.Parent.RemoveChild(node)
	}
}

// findElement does a DFS and returns the first element with the given tag name,
// or nil if not found.
func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

// stripMDXArtifacts removes MDX/React component syntax from markdown output.
// This strips PascalCase tags, self-closing tags, and JSX comments.
func stripMDXArtifacts(s string) string {
	// Remove JSX block comments first (they may span multiple lines).
	s = mdxComment.ReplaceAllString(s, "")
	// Remove standalone PascalCase tag lines.
	s = mdxTag.ReplaceAllString(s, "")
	// Remove self-closing PascalCase tag lines.
	s = mdxSelfClosing.ReplaceAllString(s, "")
	// Collapse any new blank lines created by the removals.
	s = multiBlank.ReplaceAllString(s, "\n\n")
	return s
}

// stripCopyButtonsFromCodeBlocks removes "copy" button text artifacts from
// inside fenced code blocks. It only strips lines that are exactly "copy",
// "Copy", or "COPY" (case-insensitive) when they appear within fenced blocks.
func stripCopyButtonsFromCodeBlocks(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for fenced code block markers (``` or ~~~).
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			result = append(result, line)
			continue
		}

		// Inside code blocks, strip lines that are only "copy" variants.
		if inCodeBlock {
			lower := strings.ToLower(trimmed)
			if lower == "copy" {
				// Skip this line (it's a copy button artifact).
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// wrapNode wraps a single node in a minimal html>body shell so the converter
// receives a well-formed document.
func wrapNode(node *html.Node) (*html.Node, error) {
	// Render just this subtree to a string, then re-parse as a full document.
	var buf bytes.Buffer
	if err := html.Render(&buf, node); err != nil {
		return nil, err
	}
	doc, err := html.Parse(strings.NewReader("<html><body>" + buf.String() + "</body></html>"))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// ConvertHTMLToMarkdown converts an HTML string to clean Markdown.
//
// Strategy:
//  1. Parse the full document.
//  2. If a <main> or <article> element exists, use only that subtree — it is
//     the most reliable signal of primary page content.
//  3. Otherwise fall back to the full document, stripping known noise elements.
//  4. Convert to Markdown.
//  5. Collapse runs of 3+ blank lines down to 2.
func ConvertHTMLToMarkdown(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var workDoc *html.Node

	// Prefer <main>, then <article>.
	if main := findElement(doc, "main"); main != nil {
		unwrapCustomElements(main)
		stripNoise(main)
		workDoc, err = wrapNode(main)
		if err != nil {
			return "", err
		}
	} else if article := findElement(doc, "article"); article != nil {
		unwrapCustomElements(article)
		stripNoise(article)
		workDoc, err = wrapNode(article)
		if err != nil {
			return "", err
		}
	} else {
		// Full-document fallback: strip noise in place.
		unwrapCustomElements(doc)
		stripNoise(doc)
		workDoc = doc
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, workDoc); err != nil {
		return "", err
	}

	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	markdown, err := conv.ConvertString(buf.String())
	if err != nil {
		return "", err
	}

	// Collapse 3+ consecutive blank lines → 2.
	markdown = multiBlank.ReplaceAllString(markdown, "\n\n")

	// Strip MDX artifacts from the markdown output.
	markdown = stripMDXArtifacts(markdown)

	// Strip copy button artifacts from code blocks.
	markdown = stripCopyButtonsFromCodeBlocks(markdown)

	return strings.TrimSpace(markdown), nil
}
