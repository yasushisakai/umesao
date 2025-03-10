package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

func AzureOCR(filePath, language string) (string, error) {

	azureEndpoint, err := RequireEnvVar("AZURE_ENDPOINT")

	if err != nil {
		return "", fmt.Errorf("Failed to get Azure endpoint: %v", err)
	}

	azureKey, err := RequireEnvVar("AZURE_KEY")

	if err != nil {
		return "", fmt.Errorf("Failed to get Azure key: %v", err)
	}

	// Send OCR request to Azure with the specified language
	location, err := AzureOCRRequestWithLanguage(azureEndpoint, azureKey, filePath, language)
	if err != nil {
		return "", fmt.Errorf("error sending OCR request: %v", err)
	}

	// Fetch OCR result
	var ocrResult string
	attempt := 3

	for {
		time.Sleep(3 * time.Second)
		ocrResult, err = AzureOCRFetchResult(azureKey, location)
		if err != nil && attempt > 0 {
			fmt.Printf("OCR fetch did not succeed: %s\nRetrying in 3 seconds...\n", err)
			attempt = attempt - 1
		} else {
			break
		}
	}

	if attempt < 0 {
		return "", fmt.Errorf("too many failed OCR fetch attempts")
	}

	return ocrResult, nil

}

// AzureOCRRequestWithLanguage sends an OCR request to Azure with a specified language
func AzureOCRRequestWithLanguage(endpoint, key, path, language string) (string, error) {

	// Read the image file into memory.
	fileData, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read image file: %v", err)
	}

	// Define the URL with the query parameter.
	url := fmt.Sprintf("%s/vision/v3.2/read/analyze?language=%s", endpoint, language)
	// Create a new POST request with the image data as the body.
	req, err := http.NewRequest("POST", url, bytes.NewReader(fileData))
	if err != nil {
		log.Fatalf("Failed to create HTTP request: %v", err)
	}

	// Set the necessary headers.
	req.Header.Set("Ocp-Apim-Subscription-Key", key)
	req.Header.Set("Content-Type", "application/octet-stream")

	// Use the default HTTP client to send the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Retrieve the "Operation-Location" header from the response.
	operationLocation := resp.Header.Get("Operation-Location")

	if operationLocation == "" {
		return "", errors.New("Operation-Location not found in response.")
	}

	return operationLocation, nil
}

func AzureOCRFetchResult(key, location string) (string, error) {

	req, err := http.NewRequest("GET", location, bytes.NewBufferString(""))

	if err != nil {
		return "", err
	}

	req.Header.Set("Ocp-Apim-Subscription-Key", key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", errors.New("API request failed: " + string(bodyBytes))
	}

	var ocrResultPayload struct {
		Status        string `json:"status"`
		AnalyzeResult struct {
			ReadResult []struct {
				Lines []struct {
					BoundingBox []uint16 `json:"boundingBox"`
					Text        string   `json:"text"`
					Appearance  struct {
						Style struct {
							Confidence float64 `json:"confidence"`
						} `json:"style"`
					} `json:"appearance"`
				} `json:"lines"`
			} `json:"readResults"`
		} `json:"analyzeResult"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ocrResultPayload); err != nil {
		log.Print("decode\n")
		return "", err
	}

	if ocrResultPayload.Status != "succeeded" {
		return "", errors.New("ocr failed")
	}

	payloadBytes, err := json.Marshal(ocrResultPayload)

	if err != nil {
		log.Print("marshal\n")
		return "", err
	}

	return string(payloadBytes), nil

}
