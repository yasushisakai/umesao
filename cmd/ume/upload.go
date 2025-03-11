package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Import png decoder
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nfnt/resize"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"

	_ "github.com/joho/godotenv/autoload"
)

// OpenAIRequest represents a request to the OpenAI API for vision
type OpenAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

// Message represents a message in the OpenAI request
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Content represents content in a message
type Content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in content
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

// OpenAIResponse represents a response from the OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// uploadImpl implements the upload command functionality
// func uploadImpl(filePath string, method string, language string) error {
func uploadImpl(filePath, method, language string) error {
	// Check if the file exists and is readable
	_, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error accessing file: %v", err)
	}

	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Create a new card
	cardID, err := queries.CreateCard(context.Background())
	if err != nil {
		return fmt.Errorf("error creating card: %v", err)
	}

	fmt.Printf("Created new card with ID: %d\n", cardID)

	// Initialize Minio client from common package
	minioClient, err := common.NewMinioClient()
	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}

	// Upload the image file for the card
	imageName, err := minioClient.UploadImageForCard(cardID, filePath)
	if err != nil {
		return fmt.Errorf("error uploading image file: %v", err)
	}

	fmt.Printf("Successfully uploaded image %s\n", imageName)

	// Associate the image with the card in the database
	err = queries.CreateImage(context.Background(), database.CreateImageParams{
		CardID:   cardID,
		Filename: imageName,
		Method:   method,
	})

	if err != nil {
		return fmt.Errorf("error associating image with card: %v", err)
	}

	fmt.Printf("Successfully associated image %s with card %d in the database\n", imageName, cardID)

	// Get OpenAI API key
	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	if err != nil {
		return fmt.Errorf("error getting OpenAI API key: %v", err)
	}

	// Extract text from the image based on the method
	var content string
	switch method {
	case "ocr":
		content, err = processWithOCR(filePath, language)
	case "mistral":
		content, err = processWithMistral(filePath, openaiKey)
	default:
		content, err = processWithVision(filePath, openaiKey)
	}

	if err != nil {
		return err
	}

	fmt.Println("Successfully converted result to markdown")

	// Extract chunks from markdown
	chunks := common.ExtractChunks(content, method)
	fmt.Printf("Extracted %d chunks from content\n", len(chunks))

	// Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	if err != nil {
		return fmt.Errorf("error generating embeddings: %v", err)
	}

	fmt.Printf("Generated %d embeddings\n", len(embeddings))

	// Calculate hash of markdown content
	hashString := common.CalculateFileHash([]byte(content))

	// Set the markdown version for new cards
	markdownVersion := 1

	// Upload the markdown file using the common function
	err = minioClient.UploadMarkdownForCard(cardID, int32(markdownVersion), []byte(content))
	if err != nil {
		return fmt.Errorf("error uploading markdown file: %v", err)
	}

	fmt.Printf("Successfully uploaded markdown file for card %d, version %d\n", cardID, markdownVersion)

	// Store the markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: cardID,
		Ver:    int32(markdownVersion),
		Hash:   hashString,
	})

	if err != nil {
		return fmt.Errorf("error storing markdown hash in database: %v", err)
	}

	fmt.Printf("Successfully stored markdown hash in database for card %d, version %d\n", cardID, markdownVersion)

	// Store embeddings in the database
	for i, embedding := range embeddings {
		if strings.TrimSpace(chunks[i]) == "" {
			continue
		}

		pgvEmbed := pgvector.NewVector(common.ConvertFloat64ToFloat32(embedding))
		err = queries.CreateEmbeddings(context.Background(), database.CreateEmbeddingsParams{
			CardID:    cardID,
			Ver:       int32(markdownVersion),
			Idx:       int32(i),
			Model:     "text-embedding-3-small",
			Text:      chunks[i],
			Embedding: pgvEmbed,
		})

		if err != nil {
			return fmt.Errorf("error storing embedding %d in database: %v", i, err)
		}
	}

	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, markdownVersion)
	fmt.Println("Upload process completed successfully!")

	return nil
}

// processWithOCR extracts text from an image using Azure OCR
func processWithOCR(filePath, language string) (string, error) {

	ocrResult, err := common.AzureOCR(filePath, language)

	if err != nil {
		return "", fmt.Errorf("error processing image with Azure OCR: %v", err)
	}

	fmt.Println("Successfully fetched OCR result")

	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")

	if err != nil {
		return "", fmt.Errorf("error getting OpenAI key: %v", err)
	}

	// Convert OCR result to markdown
	md, err := common.Ocr2md(openaiKey, "o1-mini", ocrResult)
	if err != nil {
		return "", fmt.Errorf("error creating markdown from OCR result: %v", err)
	}

	return md, nil
}

// processWithMistral extracts text from an image using Mistral's OCR API
func processWithMistral(filePath string, openaiKey string) (string, error) {
	// Use Mistral OCR to extract text from the image
	ocrResult, err := common.MistralOCR(filePath)
	if err != nil {
		return "", fmt.Errorf("error processing image with Mistral OCR: %v", err)
	}

	fmt.Println("Successfully fetched Mistral OCR result")

	// Convert OCR result to markdown using OpenAI
	md, err := common.Ocr2md(openaiKey, "o1-mini", ocrResult)
	if err != nil {
		return "", fmt.Errorf("error creating markdown from Mistral OCR result: %v", err)
	}

	return md, nil
}

// processWithVision extracts text from an image using OpenAI's Vision API
func processWithVision(filePath string, apiKey string) (string, error) {
	// Open the image file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	// Resize the image to fit within 1024x512 while maintaining aspect ratio
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	var newWidth, newHeight uint

	if width > height { // Landscape orientation
		newWidth = 1024
		newHeight = uint(float64(height) * (1024.0 / float64(width)))
	} else { // Portrait or square orientation
		newHeight = 512
		newWidth = uint(float64(width) * (512.0 / float64(height)))
	}

	resizedImg := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	// Convert image to base64
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, resizedImg, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encode image to JPEG: %v", err)
	}

	base64Img := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Create the request to OpenAI API
	reqBody := OpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{
				Role: "user",
				Content: []Content{
					{
						Type: "text",
						Text: "This is a image that is either a diagram, graph, chart or table. Explain what this visualization is and the insights. Output only the results as a complete paragraph, so this could be used as an caption.",
					},
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL:    fmt.Sprintf("data:image/jpeg;base64,%s", base64Img),
							Detail: "high",
						},
					},
				},
			},
		},
		MaxTokens: 300,
	}

	jsonReqBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Make the API request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonReqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	// Get the result
	if len(openAIResp.Choices) > 0 {
		fmt.Println("Successfully received response from Vision API")
		return openAIResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content in the Vision API response")
}
