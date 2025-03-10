package common

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMistralOCR(t *testing.T) {
	// Set up a mock server to handle the OCR request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Verify headers
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Errorf("Expected Authorization header 'Bearer test-key', got %s", authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type header 'application/json', got %s", contentType)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Read and validate request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var reqBody MistralOCRRequest
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Validate request structure
		if reqBody.Model != "mistral-ocr-latest" {
			t.Errorf("Expected model 'mistral-ocr-latest', got '%s'", reqBody.Model)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if reqBody.Document.Type != "image_url" {
			t.Errorf("Expected document type 'image_url', got '%s'", reqBody.Document.Type)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check that the image URL contains base64 data
		if len(reqBody.Document.ImageURL) < 30 || !strings.HasPrefix(reqBody.Document.ImageURL, "data:image/jpeg;base64,") {
			t.Errorf("Image URL format is incorrect: %s...", reqBody.Document.ImageURL[:30])
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return a successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := MistralOCRResponse{
			Text: "This is a test OCR result.",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create a test environment
	originalMistralKey := os.Getenv("MISTRAL_KEY")

	// Set up test environment
	os.Setenv("MISTRAL_KEY", "test-key")

	// Save the original function and replace with our test version
	originalHTTPNewRequest := httpNewRequest
	defer func() {
		httpNewRequest = originalHTTPNewRequest
		os.Setenv("MISTRAL_KEY", originalMistralKey)
	}()

	// Replace the http.NewRequest function to use our test server
	httpNewRequest = func(method, url string, body io.Reader) (*http.Request, error) {
		return http.NewRequest(method, server.URL, body)
	}

	// Use a sample image for the test
	// We need to make sure the sample.jpg exists in the repo
	result, err := MistralOCR("../../sample.jpg")
	if err != nil {
		t.Fatalf("MistralOCR returned an error: %v", err)
	}

	expectedResult := "This is a test OCR result."
	if result != expectedResult {
		t.Errorf("Expected OCR result '%s', got '%s'", expectedResult, result)
	}
}

// Using the httpNewRequest variable defined in mistral.go

