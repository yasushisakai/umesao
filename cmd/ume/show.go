package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/yasushisakai/umesao/pkg/common"
)

func showCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: ume show [card_id]")
	}

	showFlags := flag.NewFlagSet("show", flag.ExitOnError)
	versionFlag := showFlags.Int("version", -1, "Version number of markdown file (default: latest)")
	versionShortFlag := showFlags.Int("v", -1, "Version number of markdown file (default: latest)")
	langFlag := showFlags.String("lang", "", "Translate markdown to specified language")
	langShortFlag := showFlags.String("l", "", "Translate markdown to specified language")
	showFlags.Parse(args[1:])

	// If short flag is set but long flag is not, use short flag's value
	version := *versionFlag
	if version == -1 && *versionShortFlag != -1 {
		version = *versionShortFlag
	}

	lang := *langFlag
	if lang == "" && *langShortFlag != "" {
		lang = *langShortFlag
	}

	cardID, err := common.ParseCardIDString(showFlags.Arg(0))
	if err != nil {
		return err
	}

	return showImpl(cardID, version, lang)
}

func showImpl(cardID int, version int, lang string) error {
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return err
	}
	defer dbpool.Close()

	// Get card information
	card, err := queries.GetCardImage(context.Background(), int32(cardID))
	if err != nil {
		return fmt.Errorf("card not found: %w", err)
	}

	// Get image URL
	minioClient, err := common.NewMinioClient()
	if err != nil {
		return err
	}

	imageURL := minioClient.GetImageURLForCard(card.Filename)

	var markdownContent string

	// If no version is specified, get the latest version
	if version == -1 {
		latestVersion, err := queries.GetLatestMarkdownVersion(context.Background(), int32(cardID))
		if err != nil {
			return fmt.Errorf("failed to get latest markdown version: %w", err)
		}
		version = int(latestVersion)
	}

	// Create a temporary file to store the markdown content
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("card_%d_*.md", cardID))
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpFileName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFileName)

	// Get markdown content
	err = minioClient.GetMarkdownForCard(int32(cardID), int32(version), tmpFileName)
	if err != nil {
		return fmt.Errorf("failed to get markdown: %w", err)
	}

	// Read markdown content
	markdownBytes, err := os.ReadFile(tmpFileName)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}
	markdownContent = string(markdownBytes)

	// If language is specified, translate the markdown
	if lang != "" {
		openaiClient, err := common.NewOpenAIClient()
		if err != nil {
			return fmt.Errorf("failed to create OpenAI client: %w", err)
		}

		translatedContent, err := openaiClient.TranslateText(markdownContent, lang)
		if err != nil {
			return fmt.Errorf("failed to translate text: %w", err)
		}
		markdownContent = translatedContent
	}

	// Create HTML content
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Card %d - Version %d</title>
    <style>
        body {
			background-color: #000000;
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            display: flex;
        }
        .image-container {
            flex: 1;
            padding-right: 20px;
        }
        .markdown-container {
            flex: 1;
        }
        img {
			filter: invert(1);
            max-width: 100%%;
            max-height: 800px;
            object-fit: contain;
        }
    </style>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/github-markdown-css/github-markdown.min.css">
    <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
</head>
<body>
	<div>
    <div class="image-container">
        <img src="%s" alt="Card Image">
    </div>
    <div class="markdown-container markdown-body" id="markdown-content"></div>
    <script>
        document.getElementById('markdown-content').innerHTML = marked.parse("%s");
    </script>
	</div>
</body>
</html>`, cardID, version, imageURL, template.JSEscapeString(markdownContent))

	// Create a temporary HTML file
	htmlTmpFile, err := os.CreateTemp("", fmt.Sprintf("card_%d_*.html", cardID))
	if err != nil {
		return fmt.Errorf("failed to create temporary HTML file: %w", err)
	}
	htmlTmpFileName := htmlTmpFile.Name()

	// Write HTML content to file
	_, err = htmlTmpFile.WriteString(htmlContent)
	if err != nil {
		htmlTmpFile.Close()
		os.Remove(htmlTmpFileName)
		return fmt.Errorf("failed to write HTML file: %w", err)
	}
	htmlTmpFile.Close()

	// Convert to file URL
	htmlFileURL := fmt.Sprintf("file://%s", filepath.ToSlash(htmlTmpFileName))

	// Open HTML file in browser
	err = common.OpenBrowser(htmlFileURL)
	if err != nil {
		os.Remove(htmlTmpFileName)
		return err
	}

	fmt.Printf("Opened card %d in browser. Press Enter to close...\n", cardID)
	fmt.Scanln() // Wait for user input before removing the file

	// Remove the temporary file after user is done viewing
	return os.Remove(htmlTmpFileName)
}
