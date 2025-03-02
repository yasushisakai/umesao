package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

// editImpl implements the edit command functionality
func editImpl(cardID int, verbose bool) error {
	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Get the latest markdown version for the card
	latestVersion, err := queries.GetLatestMarkdownVersion(context.Background(), int32(cardID))
	if err != nil {
		return fmt.Errorf("error getting latest markdown version: %v", err)
	}

	// Display image for the card if available
	err = common.DisplayCardImages(int32(cardID), *queries)
	if err != nil {
		fmt.Printf("Note: %v (no image found or error displaying)\n", err)
	}

	// Initialize Minio client
	minioClient, err := common.NewMinioClient()
	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}

	// Create a temporary file to store the markdown content
	tempFile := fmt.Sprintf("/tmp/%d_%d.md", cardID, latestVersion)

	// Download the markdown file using the common function
	err = minioClient.GetMarkdownForCard(int32(cardID), latestVersion, tempFile)
	if err != nil {
		return fmt.Errorf("error downloading markdown file: %v", err)
	}

	if verbose {
		fmt.Printf("Successfully downloaded markdown file to %s\n", tempFile)
	}

	// Read the markdown file content
	mdContent, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("error reading markdown file: %v", err)
	}

	// Calculate hash of the markdown content
	downloadHashString := common.CalculateFileHash(mdContent)

	// Open the file in neovim for editing
	cmd := exec.Command("nvim", tempFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error opening file in neovim: %v", err)
	}

	// Read the file content after editing
	editedContent, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("error reading edited file: %v", err)
	}

	// Calculate hash of the edited content
	editedHashString := common.CalculateFileHash(editedContent)

	// Check if the content has changed
	if downloadHashString == editedHashString {
		fmt.Println("No changes detected. Exiting.")
		os.Remove(tempFile)
		return nil
	}

	if verbose {
		fmt.Println("Changes detected. Updating markdown version in Minio and database.")
	}

	// Increment version number
	newVersion := latestVersion + 1

	// Upload the edited markdown file using the common function
	err = minioClient.UploadMarkdownForCard(int32(cardID), newVersion, editedContent)
	if err != nil {
		return fmt.Errorf("error uploading edited markdown file: %v", err)
	}

	if verbose {
		fmt.Printf("Successfully uploaded edited markdown for card %d, version %d\n", cardID, newVersion)
	}

	// Store the new markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: int32(cardID),
		Ver:    newVersion,
		Hash:   editedHashString,
	})
	if err != nil {
		return fmt.Errorf("error storing new markdown hash in database: %v", err)
	}

	if verbose {
		fmt.Printf("Successfully stored new markdown hash in database for card %d, version %d\n", cardID, newVersion)
	}

	// Get environment variables for OpenAI API
	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	if err != nil {
		return fmt.Errorf("error getting OpenAI API key: %v", err)
	}

	// Extract chunks from the edited markdown
	mdString := string(editedContent)
	chunks := common.ExtractChunks(mdString)
	if verbose {
		fmt.Printf("Extracted %d chunks from markdown\n", len(chunks))
	}

	// Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	if err != nil {
		return fmt.Errorf("error generating embeddings: %v", err)
	}

	if verbose {
		fmt.Printf("Generated %d embeddings\n", len(embeddings))
	}

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

		if err != nil {
			return fmt.Errorf("error storing embedding %d in database: %v", i, err)
		}
	}

	// Always show this important message even in non-verbose mode
	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, newVersion)

	// Clean up the temporary file
	os.Remove(tempFile)

	return nil
}
