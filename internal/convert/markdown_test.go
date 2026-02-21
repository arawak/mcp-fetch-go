package convert_test

import (
	"os"
	"strings"
	"testing"

	"github.com/arawak/mcp-fetch-go/internal/convert"
)

func TestConvertHTMLToMarkdown_Simple(t *testing.T) {
	html := `<html><body><h1>Hello</h1><p>World</p></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "Hello") {
		t.Errorf("expected heading text in output, got: %q", md)
	}
	if !strings.Contains(md, "World") {
		t.Errorf("expected paragraph text in output, got: %q", md)
	}
	if strings.Contains(md, "<h1>") || strings.Contains(md, "<p>") {
		t.Errorf("raw HTML tags should not appear in output, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_HeadingLevels(t *testing.T) {
	html := `<html><body><h1>One</h1><h2>Two</h2><h3>Three</h3></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "# One") {
		t.Errorf("expected h1 as '# One', got: %q", md)
	}
	if !strings.Contains(md, "## Two") {
		t.Errorf("expected h2 as '## Two', got: %q", md)
	}
	if !strings.Contains(md, "### Three") {
		t.Errorf("expected h3 as '### Three', got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_CodeBlock(t *testing.T) {
	html := `<html><body><pre><code>func main() {}</code></pre></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "func main()") {
		t.Errorf("expected code content in output, got: %q", md)
	}
	if strings.Contains(md, "<pre>") || strings.Contains(md, "<code>") {
		t.Errorf("raw code tags should not appear in output, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_Links(t *testing.T) {
	html := `<html><body><p><a href="https://go.dev">Go</a></p></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "Go") {
		t.Errorf("expected link text in output, got: %q", md)
	}
	if !strings.Contains(md, "https://go.dev") {
		t.Errorf("expected link href in output, got: %q", md)
	}
}

// ---- noise stripping ----

func TestConvertHTMLToMarkdown_StripsNav(t *testing.T) {
	html := `<html><body>
<nav><a href="/">Home</a><a href="/about">About</a></nav>
<main><h1>Content</h1><p>Real content here.</p></main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Home") || strings.Contains(md, "About") {
		t.Errorf("nav links should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content") || !strings.Contains(md, "Real content here") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsFooter(t *testing.T) {
	html := `<html><body>
<main><p>Article text.</p></main>
<footer><p>Copyright 2025</p></footer>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Copyright") {
		t.Errorf("footer content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Article text") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsScript(t *testing.T) {
	html := `<html><body>
<script>alert('xss')</script>
<p>Page content.</p>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "alert") {
		t.Errorf("script content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Page content") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsStyle(t *testing.T) {
	html := `<html><head><style>body { color: red; }</style></head>
<body><p>Styled content.</p></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "color: red") {
		t.Errorf("style content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Styled content") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsAside(t *testing.T) {
	html := `<html><body>
<main><p>Article text.</p></main>
<aside><p>Related: some sidebar content</p></aside>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "sidebar content") {
		t.Errorf("aside content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Article text") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsRoleNavigation(t *testing.T) {
	html := `<html><body>
<div role="navigation"><a href="/">Nav link</a></div>
<main><p>The real content.</p></main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Nav link") {
		t.Errorf("role=navigation content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "real content") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsClassSidebar(t *testing.T) {
	html := `<html><body>
<div class="sidebar-left"><p>Sidebar stuff</p></div>
<main><p>Main content.</p></main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Sidebar stuff") {
		t.Errorf("sidebar class content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Main content") {
		t.Errorf("main content should be preserved, got: %q", md)
	}
}

// ---- main/article extraction ----

func TestConvertHTMLToMarkdown_PrefersMain(t *testing.T) {
	html := `<html><body>
<nav><a href="/">Home</a></nav>
<header><p>Site header noise</p></header>
<main>
  <h1>Primary Heading</h1>
  <p>Primary content paragraph.</p>
</main>
<footer><p>Footer noise</p></footer>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "Primary Heading") {
		t.Errorf("main content heading should be present, got: %q", md)
	}
	if !strings.Contains(md, "Primary content paragraph") {
		t.Errorf("main content paragraph should be present, got: %q", md)
	}
	for _, noise := range []string{"Site header noise", "Footer noise", "Home"} {
		if strings.Contains(md, noise) {
			t.Errorf("noise text %q should have been excluded via <main> extraction, got: %q", noise, md)
		}
	}
}

func TestConvertHTMLToMarkdown_PrefersArticle(t *testing.T) {
	html := `<html><body>
<nav><a href="/">Home</a></nav>
<article>
  <h2>Article Title</h2>
  <p>Article body text.</p>
</article>
<aside><p>Related links</p></aside>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "Article Title") {
		t.Errorf("article heading should be present, got: %q", md)
	}
	if !strings.Contains(md, "Article body text") {
		t.Errorf("article body should be present, got: %q", md)
	}
	for _, noise := range []string{"Related links", "Home"} {
		if strings.Contains(md, noise) {
			t.Errorf("noise text %q should be excluded via <article> extraction, got: %q", noise, md)
		}
	}
}

func TestConvertHTMLToMarkdown_FallsBackWithoutMain(t *testing.T) {
	// No <main> or <article>: full doc fallback with noise stripping.
	html := `<html><body>
<nav><a href="/">Nav noise</a></nav>
<div>
  <h1>The Content</h1>
  <p>Real paragraph.</p>
</div>
<footer><p>Footer noise</p></footer>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "The Content") {
		t.Errorf("content heading should be present in fallback, got: %q", md)
	}
	if !strings.Contains(md, "Real paragraph") {
		t.Errorf("content paragraph should be present in fallback, got: %q", md)
	}
	for _, noise := range []string{"Nav noise", "Footer noise"} {
		if strings.Contains(md, noise) {
			t.Errorf("noise %q should be stripped in fallback, got: %q", noise, md)
		}
	}
}

// ---- whitespace collapsing ----

func TestConvertHTMLToMarkdown_CollapseWhitespace(t *testing.T) {
	// Construct HTML that would normally produce multiple blank lines.
	html := `<html><body>
<main>
<h1>Title</h1>
<p>Para one.</p>
<p>Para two.</p>
<p>Para three.</p>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "\n\n\n") {
		t.Errorf("output should not contain 3+ consecutive newlines, got: %q", md)
	}
}

// ---- effective_go integration ----

func TestConvertHTMLToMarkdown_EffectiveGo(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/effective_go.html")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	md, err := convert.ConvertHTMLToMarkdown(string(data))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// Should contain real document headings.
	for _, heading := range []string{"Introduction", "Formatting", "Commentary", "Names"} {
		if !strings.Contains(md, heading) {
			t.Errorf("expected heading %q in output", heading)
		}
	}

	// Should not contain raw HTML tags.
	for _, tag := range []string{"<nav", "<footer", "<script", "<style"} {
		if strings.Contains(md, tag) {
			t.Errorf("raw tag %q should not appear in output", tag)
		}
	}

	// Nav link text that should be stripped.
	for _, navText := range []string{"Why Go", "Packages"} {
		if strings.Contains(md, navText) {
			t.Errorf("nav text %q should have been stripped from output", navText)
		}
	}

	// No triple blank lines.
	if strings.Contains(md, "\n\n\n") {
		t.Error("output should not contain 3+ consecutive newlines")
	}

	// Output should be non-trivially long (real content preserved).
	if len(md) < 10000 {
		t.Errorf("output suspiciously short (%d bytes); content may have been lost", len(md))
	}

	t.Logf("effective_go: input=%d bytes, output=%d bytes", len(data), len(md))
}

// ---- button and img stripping ----

func TestConvertHTMLToMarkdown_StripsButton(t *testing.T) {
	html := `<html><body>
<main>
  <p>Article content.</p>
  <button>Read more</button>
  <button>Share</button>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Read more") || strings.Contains(md, "Share") {
		t.Errorf("button text should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Article content") {
		t.Errorf("article content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsImg(t *testing.T) {
	html := `<html><body>
<main>
  <img src="icon.png" alt="subreddit icon">
  <p>Real content.</p>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Image markdown should not appear.
	if strings.Contains(md, "![") {
		t.Errorf("image markdown should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Real content") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsAriaHidden(t *testing.T) {
	html := `<html><body>
<main>
  <p>Post content.</p>
  <span aria-hidden="true">•</span>
  <span aria-hidden="true">decorative separator</span>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "•") || strings.Contains(md, "decorative separator") {
		t.Errorf("aria-hidden content should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Post content") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

// ---- custom element handling ----

func TestConvertHTMLToMarkdown_UnwrapsCustomElements(t *testing.T) {
	// Unknown custom elements should become transparent — their children survive.
	html := `<html><body>
<main>
  <my-component>
    <h1>Heading inside component</h1>
    <p>Paragraph inside component.</p>
  </my-component>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(md, "Heading inside component") {
		t.Errorf("content inside unknown custom element should be preserved, got: %q", md)
	}
	if !strings.Contains(md, "Paragraph inside component") {
		t.Errorf("paragraph inside unknown custom element should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsKnownNoiseCustomElements(t *testing.T) {
	html := `<html><body>
<main>
  <faceplate-loader>loading...</faceplate-loader>
  <faceplate-screen-reader-content>Go to subreddit</faceplate-screen-reader-content>
  <faceplate-shimmer>placeholder</faceplate-shimmer>
  <p>The actual post content.</p>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, noise := range []string{"loading...", "Go to subreddit", "placeholder"} {
		if strings.Contains(md, noise) {
			t.Errorf("noise custom element text %q should be stripped, got: %q", noise, md)
		}
	}
	if !strings.Contains(md, "actual post content") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

// ---- Reddit real-world integration ----

func TestConvertHTMLToMarkdown_Reddit(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/reddit_sample.html")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	md, err := convert.ConvertHTMLToMarkdown(string(data))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// Post title and body must be present.
	if !strings.Contains(md, "Sometimes opencode just stops") {
		t.Errorf("expected post title in output, got: %q", md[:min(500, len(md))])
	}
	if !strings.Contains(md, "first couple of rounds") {
		t.Errorf("expected post body text in output, got: %q", md[:min(500, len(md))])
	}

	// UI noise must be gone.
	for _, noise := range []string{
		"Go to opencodeCLI",
		"Read more",
		"Share",
		"r/opencodeCLI icon",
	} {
		if strings.Contains(md, noise) {
			t.Errorf("UI noise %q should have been stripped, got in output", noise)
		}
	}

	// No raw HTML tags.
	if strings.Contains(md, "<shreddit") || strings.Contains(md, "<faceplate") {
		t.Errorf("custom element tags should not appear in output")
	}

	t.Logf("reddit: input=%d bytes, output=%d bytes", len(data), len(md))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---- role=button and JS anchor stripping ----

func TestConvertHTMLToMarkdown_StripsRoleButton(t *testing.T) {
	html := `<html><body>
<main>
  <p>Content.</p>
  <div role="button">Open menu</div>
  <span role="button">Skip to content</span>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "Open menu") || strings.Contains(md, "Skip to content") {
		t.Errorf("role=button elements should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsJSAnchors(t *testing.T) {
	html := `<html><body>
<main>
  <p>Real content.</p>
  <a href="#">Share</a>
  <a href="javascript:void(0)">Copy link</a>
  <a href="">Open menu</a>
  <a href="https://go.dev">Real link</a>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, noise := range []string{"Share", "Copy link", "Open menu"} {
		if strings.Contains(md, noise) {
			t.Errorf("JS anchor text %q should be stripped, got: %q", noise, md)
		}
	}
	// Real navigation links must survive.
	if !strings.Contains(md, "Real link") || !strings.Contains(md, "go.dev") {
		t.Errorf("real anchor links should be preserved, got: %q", md)
	}
	if !strings.Contains(md, "Real content") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsLoginAnchors(t *testing.T) {
	// GitHub auth-gate links like Star and Notifications point to /login?return_to=...
	html := `<html><body>
<main>
  <a href="/login?return_to=%2Forg%2Frepo">Star</a>
  <a href="/login?return_to=%2Forg%2Frepo">Notifications</a>
  <tool-tip>You must be signed in to change notification settings</tool-tip>
  <p>The official Go SDK.</p>
  <a href="/org/repo">Real repo link</a>
</main>
</body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, noise := range []string{"Star", "Notifications", "You must be signed in"} {
		if strings.Contains(md, noise) {
			t.Errorf("auth noise %q should be stripped, got: %q", noise, md)
		}
	}
	if !strings.Contains(md, "official Go SDK") {
		t.Errorf("content should be preserved, got: %q", md)
	}
	if !strings.Contains(md, "Real repo link") {
		t.Errorf("real repo link should survive, got: %q", md)
	}
}

// ---- GitHub real-world integration ----

func TestConvertHTMLToMarkdown_GitHub(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/github_repo.html")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	md, err := convert.ConvertHTMLToMarkdown(string(data))
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// Repo identity and description must be present.
	if !strings.Contains(md, "go-sdk") {
		t.Errorf("expected repo name in output, got: %q", md[:min(400, len(md))])
	}
	if !strings.Contains(md, "Model Context Protocol") {
		t.Errorf("expected repo description in output, got: %q", md[:min(400, len(md))])
	}

	// Auth-gate noise must be gone.
	for _, noise := range []string{"Star", "Notifications", "You must be signed in"} {
		if strings.Contains(md, noise) {
			t.Errorf("auth noise %q should have been stripped", noise)
		}
	}

	// No raw HTML.
	if strings.Contains(md, "<tool-tip") || strings.Contains(md, "<svg") {
		t.Errorf("raw custom element tags should not appear in output")
	}

	t.Logf("github: input=%d bytes, output=%d bytes", len(data), len(md))
}

// ---- MDX artifact stripping ----

func TestConvertHTMLToMarkdown_StripsOpeningMDXTag(t *testing.T) {
	html := `<html><body><main><h1>Title</h1>
<Intro>
<p>Content here.</p>
</Intro>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "<Intro>") {
		t.Errorf("opening MDX tag should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content here") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsClosingMDXTag(t *testing.T) {
	html := `<html><body><main>
</InlineToc>
<p>Content here.</p>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "</InlineToc>") {
		t.Errorf("closing MDX tag should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content here") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsSelfClosingMDXTag(t *testing.T) {
	html := `<html><body><main>
<InlineToc />
<p>Content here.</p>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "<InlineToc />") {
		t.Errorf("self-closing MDX tag should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content here") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsJSXComment(t *testing.T) {
	html := `<html><body><main>
{/* This is a comment */}
<p>Content here.</p>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "{/*") {
		t.Errorf("JSX comment should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content here") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsMultilineJSXComment(t *testing.T) {
	html := `<html><body><main>
{/*
This is a
multiline comment
*/}
<p>Content here.</p>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "{/*") {
		t.Errorf("JSX comment should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "Content here") {
		t.Errorf("content should be preserved, got: %q", md)
	}
}

// ---- copy button artifact stripping ----

func TestConvertHTMLToMarkdown_StripsCopyButtonInsideCodeBlock(t *testing.T) {
	html := `<html><body><main>
<pre><code>func main() {}
copy</code></pre>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After conversion, this should become a fenced code block.
	if strings.Contains(md, "\ncopy\n") {
		t.Errorf("copy button text should be stripped, got: %q", md)
	}
	if !strings.Contains(md, "func main()") {
		t.Errorf("code content should be preserved, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_StripsCopyButtonCaseVariants(t *testing.T) {
	html := `<html><body><main>
<pre><code>code here
Copy</code></pre>
<pre><code>code here
COPY</code></pre>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(md, "\nCopy\n") || strings.Contains(md, "\nCOPY\n") {
		t.Errorf("Copy/COPY button text should be stripped, got: %q", md)
	}
}

func TestConvertHTMLToMarkdown_PreservesCopyInProseText(t *testing.T) {
	html := `<html><body><main>
<p>Please copy this file to your workspace.</p>
<pre><code>cp file.txt dest/</code></pre>
</main></body></html>`
	md, err := convert.ConvertHTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "copy" in prose should be preserved
	if !strings.Contains(md, "copy this file") {
		t.Errorf("'copy' in prose should be preserved, got: %q", md)
	}
	if !strings.Contains(md, "cp file.txt") {
		t.Errorf("code content should be preserved, got: %q", md)
	}
}
