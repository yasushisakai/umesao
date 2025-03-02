package common

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
)

// RequireEnvVar checks if an environment variable is set and returns its value or an error
func RequireEnvVar(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s environment variable is not set", name)
	}
	return value, nil
}

// InitDB sets up database connection pool and initializes database queries
func InitDB() (*pgxpool.Pool, *database.Queries, error) {
	dbString, err := RequireEnvVar("DB_STRING")
	if err != nil {
		return nil, nil, err
	}

	dbpool, err := pgxpool.New(context.Background(), dbString)
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to database: %v", err)
	}

	// Create database queries
	queries := database.New(dbpool)
	return dbpool, queries, nil
}

// ParseCardIDString parses a string to extract a card ID
func ParseCardIDString(cardIDStr string) (int, error) {
	// Parse card ID from string
	cardID, err := strconv.Atoi(cardIDStr)
	if err != nil {
		return 0, fmt.Errorf("error parsing card ID: %v", err)
	}
	
	return cardID, nil
}

// ParseCardID parses command-line arguments to extract a card ID, prompting the user if needed
func ParseCardID(args []string) (int, error) {
	var cardIDStr string

	// Check if arguments were provided
	if len(args) == 2 {
		// Get card ID from command line argument
		cardIDStr = args[1]
	} else if len(args) == 1 {
		// Read from stdin if no arguments provided
		fmt.Println("Enter card ID:")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			cardIDStr = scanner.Text()
		} else {
			if err := scanner.Err(); err != nil {
				return 0, fmt.Errorf("error reading from stdin: %v", err)
			} else {
				return 0, fmt.Errorf("no input provided")
			}
		}
	} else {
		return 0, fmt.Errorf("invalid number of arguments")
	}

	return ParseCardIDString(cardIDStr)
}

// CalculateFileHash calculates SHA-256 hash of content and returns hex string
func CalculateFileHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// ConvertFloat64ToFloat32 converts float64 embeddings to float32 format needed for pgvector
func ConvertFloat64ToFloat32(embedding []float64) []float32 {
	float32Embedding := make([]float32, len(embedding))
	for i, val := range embedding {
		float32Embedding[i] = float32(val)
	}
	return float32Embedding
}

// EmbeddingToPGVector converts a float64 embedding to pgvector.Vector
func EmbeddingToPGVector(embedding []float64) pgvector.Vector {
	return pgvector.NewVector(ConvertFloat64ToFloat32(embedding))
}

// For testing purposes, we can override the exit behavior
var osExit = func(code int) {
	os.Exit(code)
}

// CheckError handles errors, prints a message, and exits if an error is present
func CheckError(err error, message string) {
	if err != nil {
		fmt.Printf("%s: %v\n", message, err)
		osExit(1)
	}
}

// DisplayCardImages retrieves image for a card and displays it in browser
func DisplayCardImages(cardID int32, queries database.Queries) error {
	// Get the image associated with the card
	image, err := queries.GetCardImage(context.Background(), cardID)
	if err != nil {
		return fmt.Errorf("error getting card image: %v", err)
	}

	// Initialize Minio client
	minioClient, err := NewMinioClient()
	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}

	// Get the URL to the image
	imageURL := minioClient.GetImageURLForCard(image)

	// Open the image URL in the default browser
	fmt.Printf("Opening image in browser: %s\n", imageURL)
	if err := OpenBrowser(imageURL); err != nil {
		return fmt.Errorf("error opening image: %v", err)
	}

	return nil
}
