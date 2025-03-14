package common

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func ExtractChunks(content, method string) []string {
	var chunks []string
	// var currentHeader string

	chunks = append(chunks, content)

	if method == "ocr" {

		md := goldmark.DefaultParser()
		reader := text.NewReader([]byte(content))
		root := md.Parse(reader)

		// Iterate over markdown AST nodes
		ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if heading, ok := node.(*ast.Heading); ok && entering {
				// Extract heading text
				var headerText string
				for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
					if textNode, ok := child.(*ast.Text); ok {
						headerText += string(textNode.Value([]byte(content)))
					}
				}
				// Store header as chunk
				chunks = append(chunks, headerText)
				// currentHeader = headerText
			} else if paragraph, ok := node.(*ast.Paragraph); ok && entering {
				// Extract paragraph text
				var paragraphText string
				for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
					if textNode, ok := child.(*ast.Text); ok {
						paragraphText += string(textNode.Value([]byte(content)))
					}
				}
				// Split paragraph into sentences
				sentences := splitSentences(paragraphText)
				for _, sentence := range sentences {
					chunks = append(chunks, sentence)
				}
			}
			return ast.WalkContinue, nil
		})

	} else if method == "vision" {
		// just split by new lines and sentences
		chunks = splitSentences(content)
	}

	return chunks
}

func splitSentences(text string) []string {
	re := regexp.MustCompile(`[。！？!?.]`)
	sentences := re.Split(text, -1)

	var result []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) > 0 {
			result = append(result, s)
		}
	}
	return result
}
