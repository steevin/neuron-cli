package web

import (
	"bytes"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
)

var wikiPattern = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

func renderMarkdown(content, notePath string, root *os.Root) string {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	var output bytes.Buffer
	if err := md.Convert([]byte(content), &output); err != nil {
		return ""
	}
	doc, err := html.Parse(strings.NewReader(output.String()))
	if err != nil {
		return ""
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "code" || n.Data == "pre") {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			var source string
			for _, a := range n.Attr {
				if a.Key == "src" {
					source = a.Val
				}
			}
			u, err := url.Parse(source)
			if err != nil || u.IsAbs() || u.Host != "" { // Do not silently fetch remote images while reading local notes.
				n.Type = html.TextNode
				n.Data = "[Remote image: " + source + "]"
				n.Attr = nil
				return
			}
			source = u.Path
			resolved := ""
			for _, candidate := range []string{path.Join(path.Dir(notePath), source), strings.TrimPrefix(source, "/")} {
				if _, err := cleanPath(candidate, false); err != nil {
					continue
				}
				if info, err := root.Stat(candidate); err == nil && info.Mode().IsRegular() {
					resolved = candidate
					break
				}
			}
			for i := range n.Attr {
				if n.Attr[i].Key == "src" {
					n.Attr[i].Val = "/api/asset?path=" + url.QueryEscape(resolved)
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			for i := range n.Attr {
				if n.Attr[i].Key == "href" {
					href := n.Attr[i].Val
					u, err := url.Parse(href)
					if err == nil && !u.IsAbs() && u.Host == "" && !strings.HasPrefix(href, "#") {
						n.Attr = append(n.Attr, html.Attribute{Key: "data-wiki", Val: strings.TrimSuffix(u.Path, ".md")})
						n.Attr[i].Val = "#note"
					}
				}
			}
			n.Attr = append(n.Attr, html.Attribute{Key: "target", Val: "_blank"}, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
			return
		}
		if n.Type == html.TextNode && n.Parent != nil {
			matches := wikiPattern.FindAllStringSubmatchIndex(n.Data, -1)
			if len(matches) > 0 {
				offset := 0
				for _, m := range matches {
					if m[0] > offset {
						n.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: n.Data[offset:m[0]]}, n)
					}
					target := strings.TrimSpace(n.Data[m[2]:m[3]])
					label := target
					if m[4] >= 0 {
						label = n.Data[m[4]:m[5]]
					}
					a := &html.Node{Type: html.ElementNode, Data: "a", Attr: []html.Attribute{{Key: "href", Val: "#note"}, {Key: "data-wiki", Val: target}}}
					a.AppendChild(&html.Node{Type: html.TextNode, Data: label})
					n.Parent.InsertBefore(a, n)
					offset = m[1]
				}
				n.Data = n.Data[offset:]
			}
			return
		}
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			walk(child)
			child = next
		}
	}
	walk(doc)
	output.Reset()
	if err := html.Render(&output, doc); err != nil {
		return ""
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("data-wiki").OnElements("a")
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	policy.AllowAttrs("disabled", "checked").OnElements("input")
	return policy.Sanitize(output.String())
}
