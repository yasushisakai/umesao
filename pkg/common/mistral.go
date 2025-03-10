package common

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Import png decoder for automatic format detection
	"io"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

// Variable for easier testing - allows mocking HTTP requests
var httpNewRequest = http.NewRequest

// MistralOCRRequest represents the request to Mistral OCR API
type MistralOCRRequest struct {
	Model    string   `json:"model"`
	Document Document `json:"document"`
}

// Document represents the document part of the OCR request
type Document struct {
	Type     string `json:"type"`
	ImageURL string `json:"image_url"`
}

// MistralOCRResponse represents the response from Mistral OCR API
type MistralOCRResponse struct {
	Text string `json:"text"`
}

// MistralOCR sends an image to Mistral's OCR API and returns the extracted text.
// Parameters:
//
//	path - Path to the image file.
//
// Returns:
//
//	A string containing the OCR result text and an error if any occurred.
func MistralOCR(path string) (string, error) {
	// 0. load ENV "MISTRAL_KEY"
	mistralKey, err := RequireEnvVar("MISTRAL_KEY")
	if err != nil {
		return "", fmt.Errorf("failed to get env MISTRAL_KEY: %v", err)
	}

	// 1. Read and process the image file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	// Decode the image (supports multiple formats through image decoders)
	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	// 2. Convert image to base64
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encode image to JPEG: %v", err)
	}

	base64Img := base64.StdEncoding.EncodeToString(buf.Bytes())
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64Img)

	// 3. Build the Mistral OCR API request
	reqBody := MistralOCRRequest{
		Model: "mistral-ocr-latest",
		Document: Document{
			Type:     "image_url",
			ImageURL: imageURL,
		},
	}

	jsonReqBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %v", err)
	}

	// 4. Make the API request
	url := "https://api.mistral.ai/v1/ocr"
	req, err := httpNewRequest("POST", url, bytes.NewBuffer(jsonReqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mistralKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 5. Parse the response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ocrResp MistralOCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return ocrResp.Text, nil
}
