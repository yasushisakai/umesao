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
)

func AzureOCRRequest(endpoint, key, path string) (string, error) {
	// Retrieve the Azure subscription key from the environment variable.

	// Read the image file into memory.
	fileData, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read image file: %v", err)
	}

	// Define the URL with the query parameter.
	url := fmt.Sprintf("%s/vision/v3.2/read/analyze?language=ja", endpoint)
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

func AzureOCRFetchResult(location, key string) (string, error) {

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
