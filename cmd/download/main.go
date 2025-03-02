package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

func main() {
	var cardIDStr string
	
	// Check if arguments were provided
	if len(os.Args) == 2 {
		// Get card ID from command line argument
		cardIDStr = os.Args[1]
	} else if len(os.Args) == 1 {
		// Read from stdin if no arguments provided
		fmt.Println("Enter card ID:")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			cardIDStr = scanner.Text()
		} else {
			if err := scanner.Err(); err != nil {
				fmt.Printf("Error reading from stdin: %v\n", err)
			} else {
				fmt.Println("No input provided")
			}
			fmt.Println("Usage: download <card_id>")
			os.Exit(1)
		}
	} else {
		fmt.Println("Usage: download <card_id>")
		os.Exit(1)
	}
	
	// Parse card ID from string
	cardID, err := strconv.Atoi(cardIDStr)
	if err != nil {
		fmt.Printf("Error parsing card ID: %v\n", err)
		os.Exit(1)
	}

	// Initialize database connection
	dbString := os.Getenv("DB_STRING")
	if dbString == "" {
		fmt.Println("DB_STRING environment variable is not set")
		os.Exit(1)
	}

	dbpool, err := pgxpool.New(context.Background(), dbString)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	// Create database queries
	queries := database.New(dbpool)

	// Get the latest markdown version for the card
	latestVersion, err := queries.GetLatestMarkdownVersion(context.Background(), int32(cardID))
	if err != nil {
		fmt.Printf("Error getting latest markdown version: %v\n", err)
		os.Exit(1)
	}
	
	// Get any images associated with the card
	images, err := queries.GetCardImages(context.Background(), int32(cardID))
	if err != nil {
		fmt.Printf("Error getting card images: %v\n", err)
		os.Exit(1)
	}

	// Initialize Minio client from common package
	minioClient, err := common.NewMinioClient()
	if err != nil {
		fmt.Printf("Error initializing Minio client: %v\n", err)
		os.Exit(1)
	}
	
	// Display images if available
	if len(images) > 0 {
		fmt.Printf("Card %d has %d associated images\n", cardID, len(images))
		
		// Get the URL to the image using the common function
		imageURL := minioClient.GetImageURLForCard(images[0])
		
		// Open the image URL in the default browser using the common function
		fmt.Printf("Opening image in browser: %s\n", imageURL)
		if err := common.OpenBrowser(imageURL); err != nil {
			fmt.Printf("Error opening image: %v\n", err)
		}
	} else {
		fmt.Printf("Card %d has no associated images\n", cardID)
	}

	// Create a temporary file to store the markdown content
	tempFile := fmt.Sprintf("/tmp/%d_%d.md", cardID, latestVersion)

	// Download the markdown file using the common function
	err = minioClient.GetMarkdownForCard(int32(cardID), latestVersion, tempFile)
	if err != nil {
		fmt.Printf("Error downloading markdown file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully downloaded markdown file to %s\n", tempFile)

	// Read the markdown file content
	mdContent, err := os.ReadFile(tempFile)
	if err != nil {
		fmt.Printf("Error reading markdown file: %v\n", err)
		os.Exit(1)
	}

	// Calculate hash of the markdown content
	hash := sha256.Sum256(mdContent)
	downloadHashString := hex.EncodeToString(hash[:])
	
	// Open the file in neovim for editing
	cmd := exec.Command("nvim", tempFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error opening file in neovim: %v\n", err)
		os.Exit(1)
	}
	
	// Read the file content after editing
	editedContent, err := os.ReadFile(tempFile)
	if err != nil {
		fmt.Printf("Error reading edited file: %v\n", err)
		os.Exit(1)
	}
	
	// Calculate hash of the edited content
	editedHash := sha256.Sum256(editedContent)
	editedHashString := hex.EncodeToString(editedHash[:])
	
	// Check if the content has changed
	if downloadHashString == editedHashString {
		fmt.Println("No changes detected. Exiting.")
		os.Exit(0)
	}
	
	fmt.Println("Changes detected. Updating markdown version in Minio and database.")
	
	// Increment version number
	newVersion := latestVersion + 1
	
	// Upload the edited markdown file using the common function
	err = minioClient.UploadMarkdownForCard(int32(cardID), newVersion, editedContent)
	if err != nil {
		fmt.Printf("Error uploading edited markdown file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully uploaded edited markdown for card %d, version %d\n", cardID, newVersion)
	
	// Store the new markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: int32(cardID),
		Ver:    newVersion,
		Hash:   editedHashString,
	})
	
	if err != nil {
		fmt.Printf("Error storing new markdown hash in database: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully stored new markdown hash in database for card %d, version %d\n", cardID, newVersion)
	
	// Get environment variables for OpenAI API
	openaiKey := os.Getenv("OPENAI_KEY")
	if openaiKey == "" {
		fmt.Println("OPENAI_KEY is not set")
		os.Exit(1)
	}
	
	// Extract chunks from the edited markdown
	mdString := string(editedContent)
	chunks := common.ExtractChunks(mdString)
	fmt.Printf("Extracted %d chunks from markdown\n", len(chunks))
	
	// Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	if err != nil {
		fmt.Printf("Error generating embeddings: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated %d embeddings\n", len(embeddings))
	
	// Store embeddings in the database
	for i, embedding := range embeddings {
		// Convert []float64 to []float32
		float32Embedding := make([]float32, len(embedding))
		for j, val := range embedding {
			float32Embedding[j] = float32(val)
		}
		pgvEmbed := pgvector.NewVector(float32Embedding)
		err = queries.CreateEmbeddings(context.Background(), database.CreateEmbeddingsParams{
			CardID:    int32(cardID),
			Ver:       newVersion,
			Idx:       int32(i),
			Model:     "text-embedding-3-small",
			Text:      chunks[i],
			Embedding: pgvEmbed,
		})
		
		if err != nil {
			fmt.Printf("Error storing embedding %d in database: %v\n", i, err)
			os.Exit(1)
		}
	}
	
	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, newVersion)
	fmt.Println("Download and edit process completed successfully!")
	
	// Clean up the temporary file
	os.Remove(tempFile)
}