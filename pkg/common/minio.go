package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	_ "github.com/joho/godotenv/autoload"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient represents a connection to the Minio service
type MinioClient struct {
	Client         *minio.Client
	Endpoint       string
	UseSSL         bool
	ImageBucket    string
	MarkdownBucket string
}

// NewMinioClient creates a new MinioClient instance
func NewMinioClient() (*MinioClient, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKeyID := os.Getenv("MINIO_USER")
	secretAccessKey := os.Getenv("MINIO_PASSWORD")
	useSSL := true

	if endpoint == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("missing required environment variables for Minio connection")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Minio client: %v", err)
	}

	return &MinioClient{
		Client:         client,
		Endpoint:       endpoint,
		UseSSL:         useSSL,
		ImageBucket:    "card-images",
		MarkdownBucket: "card-markdown",
	}, nil
}

// EnsureBucketExists checks if a bucket exists and creates it if it doesn't
func (m *MinioClient) EnsureBucketExists(bucketName string) error {
	exists, err := m.Client.BucketExists(context.Background(), bucketName)
	if err != nil {
		return fmt.Errorf("error checking if bucket %s exists: %v", bucketName, err)
	}

	if !exists {
		err = m.Client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("error creating bucket %s: %v", bucketName, err)
		}
		fmt.Printf("Successfully created bucket %s\n", bucketName)
	}

	return nil
}

// UploadFileToMinio uploads a file to a Minio bucket
func (m *MinioClient) UploadFileToMinio(bucketName, objectName string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	// Ensure the bucket exists
	if err := m.EnsureBucketExists(bucketName); err != nil {
		return minio.UploadInfo{}, err
	}

	// Upload the file
	info, err := m.Client.PutObject(
		context.Background(),
		bucketName,
		objectName,
		reader,
		size,
		minio.PutObjectOptions{ContentType: contentType},
	)

	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("error uploading file to Minio: %v", err)
	}

	return info, nil
}

// UploadFileFromPath uploads a file at the given path to a Minio bucket
func (m *MinioClient) UploadFileFromPath(bucketName, objectName, filePath string) (minio.UploadInfo, error) {
	// Read the file
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("error reading file: %v", err)
	}

	// Get file size
	fileSize := int64(len(fileContent))

	// Determine content type based on file extension
	contentType := "application/octet-stream"
	if ext := filepath.Ext(filePath); ext != "" {
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".md":
			contentType = "text/markdown"
		}
	}

	// Create a reader from the file content
	fileReader := bytes.NewReader(fileContent)

	// Upload the file
	return m.UploadFileToMinio(bucketName, objectName, fileReader, fileSize, contentType)
}

// UploadImageForCard uploads an image file for a specific card
func (m *MinioClient) UploadImageForCard(cardID int32, imagePath string) (string, error) {
	// Get the filename from the path
	fileName := filepath.Base(imagePath)

	// Upload the image
	_, err := m.UploadFileFromPath(m.ImageBucket, fileName, imagePath)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

// UploadMarkdownForCard uploads a markdown file for a specific card
func (m *MinioClient) UploadMarkdownForCard(cardID, version int32, content []byte) error {
	// Create the markdown filename
	markdownFileName := fmt.Sprintf("%d_%d.md", cardID, version)

	// Create a reader from the markdown content
	reader := bytes.NewReader(content)
	size := int64(len(content))

	// Upload the markdown file
	_, err := m.UploadFileToMinio(m.MarkdownBucket, markdownFileName, reader, size, "text/markdown")
	return err
}

// GetFileFromMinio downloads a file from a Minio bucket to a local path
func (m *MinioClient) GetFileFromMinio(bucketName, objectName, filePath string) error {
	return m.Client.FGetObject(context.Background(), bucketName, objectName, filePath, minio.GetObjectOptions{})
}

// GetMarkdownForCard downloads a markdown file for a specific card
func (m *MinioClient) GetMarkdownForCard(cardID, version int32, outputPath string) error {
	// Create the markdown filename
	markdownFileName := fmt.Sprintf("%d_%d.md", cardID, version)

	// Download the markdown file
	return m.GetFileFromMinio(m.MarkdownBucket, markdownFileName, outputPath)
}

// DeleteFileFromMinio deletes a file from a Minio bucket
func (m *MinioClient) DeleteFileFromMinio(bucketName, objectName string) error {
	return m.Client.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})
}

// GetImageURLForCard returns the public URL for a card's image
func (m *MinioClient) GetImageURLForCard(imageName string) string {
	protocol := "https"
	if !m.UseSSL {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s/%s/%s", protocol, m.Endpoint, m.ImageBucket, imageName)
}

// OpenBrowser opens a URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch os := runtime.GOOS; os {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported operating system for opening browser: %s", os)
	}

	return cmd.Run()
}
