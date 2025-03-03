package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Import png decoder
	"io"
	"log"
	"net/http"
	"os"

	"github.com/nfnt/resize"

	_ "github.com/joho/godotenv/autoload"
)

type OpenAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

type Content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func main() {
	// Parse command line argument for image path
	if len(os.Args) < 2 {
		log.Fatal("Usage: upload_vision <image_path>")
	}
	imagePath := os.Args[1]

	// Open the image file
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatalf("Failed to open image file: %v", err)
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatalf("Failed to decode image: %v", err)
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
		log.Fatalf("Failed to encode image to JPEG: %v", err)
	}

	base64Img := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Get OpenAI API key from environment variable
	apiKey := os.Getenv("OPENAI_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

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
		log.Fatalf("Failed to marshal request body: %v", err)
	}

	// Make the API request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonReqBody))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Fatalf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	// Output the result
	if len(openAIResp.Choices) > 0 {
		fmt.Println(openAIResp.Choices[0].Message.Content)
	} else {
		log.Fatal("No content in the response")
	}
}
