package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

func main() {
	// Parse card ID from command-line arguments
	cardID, err := common.ParseCardID(os.Args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Usage: download <card_id>")
		os.Exit(1)
	}

	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		common.CheckError(err, "Error initializing database")
	}
	defer dbpool.Close()

	// Get the latest markdown version for the card
	latestVersion, err := queries.GetLatestMarkdownVersion(context.Background(), int32(cardID))
	common.CheckError(err, "Error getting latest markdown version")
	
	// Display image for the card if available
	err = common.DisplayCardImages(int32(cardID), queries)
	if err != nil {
		fmt.Printf("Note: %v (no image found or error displaying)\n", err)
	}
	
	// Initialize Minio client
	minioClient, err := common.NewMinioClient()
	common.CheckError(err, "Error initializing Minio client")

	// Create a temporary file to store the markdown content
	tempFile := fmt.Sprintf("/tmp/%d_%d.md", cardID, latestVersion)

	// Download the markdown file using the common function
	err = minioClient.GetMarkdownForCard(int32(cardID), latestVersion, tempFile)
	common.CheckError(err, "Error downloading markdown file")

	fmt.Printf("Successfully downloaded markdown file to %s\n", tempFile)

	// Read the markdown file content
	mdContent, err := os.ReadFile(tempFile)
	common.CheckError(err, "Error reading markdown file")

	// Calculate hash of the markdown content
	downloadHashString := common.CalculateFileHash(mdContent)
	
	// Open the file in neovim for editing
	cmd := exec.Command("nvim", tempFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err = cmd.Run()
	common.CheckError(err, "Error opening file in neovim")
	
	// Read the file content after editing
	editedContent, err := os.ReadFile(tempFile)
	common.CheckError(err, "Error reading edited file")
	
	// Calculate hash of the edited content
	editedHashString := common.CalculateFileHash(editedContent)
	
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
	common.CheckError(err, "Error uploading edited markdown file")
	
	fmt.Printf("Successfully uploaded edited markdown for card %d, version %d\n", cardID, newVersion)
	
	// Store the new markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: int32(cardID),
		Ver:    newVersion,
		Hash:   editedHashString,
	})
	common.CheckError(err, "Error storing new markdown hash in database")
	
	fmt.Printf("Successfully stored new markdown hash in database for card %d, version %d\n", cardID, newVersion)
	
	// Get environment variables for OpenAI API
	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	common.CheckError(err, "Error getting OpenAI API key")
	
	// Extract chunks from the edited markdown
	mdString := string(editedContent)
	chunks := common.ExtractChunks(mdString)
	fmt.Printf("Extracted %d chunks from markdown\n", len(chunks))
	
	// Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	common.CheckError(err, "Error generating embeddings")
	
	fmt.Printf("Generated %d embeddings\n", len(embeddings))
	
	// Store embeddings in the database
	for i, embedding := range embeddings {
		pgvEmbed := pgvector.NewVector(common.ConvertFloat64ToFloat32(embedding))
		err = queries.CreateEmbeddings(context.Background(), database.CreateEmbeddingsParams{
			CardID:    int32(cardID),
			Ver:       newVersion,
			Idx:       int32(i),
			Model:     "text-embedding-3-small",
			Text:      chunks[i],
			Embedding: pgvEmbed,
		})
		
		common.CheckError(err, fmt.Sprintf("Error storing embedding %d in database", i))
	}
	
	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, newVersion)
	fmt.Println("Download and edit process completed successfully!")
	
	// Clean up the temporary file
	os.Remove(tempFile)
}