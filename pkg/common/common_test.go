package common

import (
	"crypto/sha256"
	"os"
	"testing"

	_ "github.com/joho/godotenv/autoload"
)

// TestGetImageURLForCard tests the GetImageURLForCard function
func TestGetImageURLForCard(t *testing.T) {
	// Create a test MinioClient
	client := &MinioClient{
		Endpoint:       "localhost:9000",
		UseSSL:         false,
		ImageBucket:    "card-images",
		MarkdownBucket: "card-markdown",
	}

	// Test with HTTP (non-SSL)
	imageName := "test-image.jpg"
	url := client.GetImageURLForCard(imageName)
	expectedURL := "http://localhost:9000/card-images/test-image.jpg"

	if url != expectedURL {
		t.Errorf("Expected URL '%s', got: '%s'", expectedURL, url)
	}

	// Test with HTTPS (SSL)
	client.UseSSL = true
	url = client.GetImageURLForCard(imageName)
	expectedURL = "https://localhost:9000/card-images/test-image.jpg"

	if url != expectedURL {
		t.Errorf("Expected URL '%s', got: '%s'", expectedURL, url)
	}
}

func TestUploadCardImage(t *testing.T) {
	// Skip this test if environment variables aren't set
	if os.Getenv("MINIO_ENDPOINT") == "" || os.Getenv("MINIO_USER") == "" || os.Getenv("MINIO_PASSWORD") == "" {
		t.Skip("Skipping test because Minio environment variables are not set")
	}

	client, err := NewMinioClient()
	if err != nil {
		t.Fatalf("Error creating Minio client: %s", err)
	}

	// upload
	projectRoot := "../../" // Relative path from pkg/common to project root
	samplePath := projectRoot + "sample.jpg"

	// Check if sample file exists
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Fatalf("Test file %s does not exist", samplePath)
	}

	info, err := client.UploadFileFromPath("card-images", "sample.jpg", samplePath)
	if err != nil {
		t.Fatalf("Error uploading file: %s", err)
	}
	defer client.DeleteFileFromMinio("card-images", "sample.jpg")

	if info.Size == 0 {
		t.Errorf("Expected file size to be greater than 0, got: %d", info.Size)
	}

	if info.Bucket != "card-images" {
		t.Errorf("Expected bucket to be 'card-images', got: '%s'", info.Bucket)
	}

	// get
	err = client.GetFileFromMinio("card-images", "sample.jpg", "temp.jpg")
	if err != nil {
		t.Fatalf("Error getting file: %s", err)
	}
	defer os.Remove("temp.jpg")

	// open the original file
	original_bytes, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatalf("Error reading original file: %s", err)
	}
	original_hash := sha256.Sum256(original_bytes)

	// open the temp file
	downloaded_bytes, err := os.ReadFile("temp.jpg")
	if err != nil {
		t.Fatalf("Error reading downloaded file: %s", err)
	}
	downloaded_hash := sha256.Sum256(downloaded_bytes)

	if original_hash != downloaded_hash {
		t.Errorf("Expected file hashes to be equal, got: %x != %x", original_hash, downloaded_hash)
	}
}
