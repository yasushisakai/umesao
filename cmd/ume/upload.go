package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

// uploadImpl implements the upload command functionality
func uploadImpl(filePath string) error {
	// Check if the file exists and is readable
	_, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error accessing file: %v", err)
	}

	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Create a new card
	cardID, err := queries.CreateCard(context.Background())
	if err != nil {
		return fmt.Errorf("error creating card: %v", err)
	}

	fmt.Printf("Created new card with ID: %d\n", cardID)

	// Initialize Minio client from common package
	minioClient, err := common.NewMinioClient()
	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}
	
	// Upload the image file for the card
	imageName, err := minioClient.UploadImageForCard(cardID, filePath)
	if err != nil {
		return fmt.Errorf("error uploading image file: %v", err)
	}
	
	fmt.Printf("Successfully uploaded image %s\n", imageName)

	// Associate the image with the card in the database
	err = queries.CreateImage(context.Background(), database.CreateImageParams{
		CardID:   cardID,
		Filename: imageName,
	})

	if err != nil {
		return fmt.Errorf("error associating image with card: %v", err)
	}

	fmt.Printf("Successfully associated image %s with card %d in the database\n", imageName, cardID)

	// OCR Pipeline
	// 1. Get environment variables
	azureEndpoint, err := common.RequireEnvVar("AZURE_ENDPOINT")
	if err != nil {
		return fmt.Errorf("error getting Azure endpoint: %v", err)
	}

	azureKey, err := common.RequireEnvVar("AZURE_KEY")
	if err != nil {
		return fmt.Errorf("error getting Azure key: %v", err)
	}

	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	if err != nil {
		return fmt.Errorf("error getting OpenAI API key: %v", err)
	}

	// 2. Send OCR request
	location, err := common.AzureOCRRequest(azureEndpoint, azureKey, filePath)
	if err != nil {
		return fmt.Errorf("error sending OCR request: %v", err)
	}

	// 3. Fetch OCR result
	var ocrResult string
	attempt := 3

	for {
		time.Sleep(3 * time.Second)
		ocrResult, err = common.AzureOCRFetchResult(location, azureKey)
		if err != nil && attempt > 0 {
			fmt.Printf("OCR fetch did not succeed: %s\nRetrying in 3 seconds...\n", err)
			attempt = attempt - 1
		} else {
			break
		}
	}

	if attempt < 0 {
		return fmt.Errorf("too many failed OCR fetch attempts")
	}

	fmt.Println("Successfully fetched OCR result")

	// 4. Convert OCR result to markdown
	md, err := common.Ocr2md(openaiKey, "o1-mini", ocrResult)
	if err != nil {
		return fmt.Errorf("error creating markdown from OCR result: %v", err)
	}

	fmt.Println("Successfully converted OCR result to markdown")

	// 5. Extract chunks from markdown
	chunks := common.ExtractChunks(md)
	fmt.Printf("Extracted %d chunks from markdown\n", len(chunks))

	// 6. Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	if err != nil {
		return fmt.Errorf("error generating embeddings: %v", err)
	}

	fmt.Printf("Generated %d embeddings\n", len(embeddings))

	// 7. Calculate hash of markdown content
	hashString := common.CalculateFileHash([]byte(md))

	// 8. Set the markdown version for new cards
	markdownVersion := 1
	
	// 9. Upload the markdown file using the common function
	err = minioClient.UploadMarkdownForCard(cardID, int32(markdownVersion), []byte(md))
	if err != nil {
		return fmt.Errorf("error uploading markdown file: %v", err)
	}
	
	fmt.Printf("Successfully uploaded markdown file for card %d, version %d\n", cardID, markdownVersion)

	// 12. Store the markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: cardID,
		Ver:    int32(markdownVersion),
		Hash:   hashString,
	})

	if err != nil {
		return fmt.Errorf("error storing markdown hash in database: %v", err)
	}

	fmt.Printf("Successfully stored markdown hash in database for card %d, version %d\n", cardID, markdownVersion)

	// 13. Store embeddings in the database
	for i, embedding := range embeddings {
		if strings.TrimSpace(chunks[i]) == "" {
			continue
		}

		pgvEmbed := pgvector.NewVector(common.ConvertFloat64ToFloat32(embedding))
		err = queries.CreateEmbeddings(context.Background(), database.CreateEmbeddingsParams{
			CardID:    cardID,
			Ver:       int32(markdownVersion),
			Idx:       int32(i),
			Model:     "text-embedding-3-small",
			Text:      chunks[i],
			Embedding: pgvEmbed,
		})

		if err != nil {
			return fmt.Errorf("error storing embedding %d in database: %v", i, err)
		}
	}

	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, markdownVersion)
	fmt.Println("Upload process completed successfully!")
	
	return nil
}