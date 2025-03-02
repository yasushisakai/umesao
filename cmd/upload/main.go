package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: upload <file>")
		os.Exit(1)
	}

	file := os.Args[1]

	// Read the file
	fileContent, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Get the filename from the path
	fileName := filepath.Base(file)

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

	// Create a new card
	cardID, err := queries.CreateCard(context.Background())
	if err != nil {
		fmt.Printf("Error creating card: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created new card with ID: %d\n", cardID)

	// Initialize Minio client from common package
	minioClient, err := common.NewMinioClient()
	if err != nil {
		fmt.Printf("Error initializing Minio client: %v\n", err)
		os.Exit(1)
	}
	
	// Upload the image file for the card
	fileName, err := minioClient.UploadImageForCard(cardID, file)
	if err != nil {
		fmt.Printf("Error uploading image file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully uploaded image %s\n", fileName)

	// Associate the image with the card in the database
	err = queries.CreateImage(context.Background(), database.CreateImageParams{
		CardID:   cardID,
		Filename: fileName,
	})

	if err != nil {
		fmt.Printf("Error associating image with card: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully associated image %s with card %d in the database\n", fileName, cardID)

	// OCR Pipeline
	// 1. Get environment variables
	azureEndpoint := os.Getenv("AZURE_ENDPOINT")
	if azureEndpoint == "" {
		fmt.Println("AZURE_ENDPOINT is not set")
		os.Exit(1)
	}

	azureKey := os.Getenv("AZURE_KEY")
	if azureKey == "" {
		fmt.Println("AZURE_KEY is not set")
		os.Exit(1)
	}

	openaiKey := os.Getenv("OPENAI_KEY")
	if openaiKey == "" {
		fmt.Println("OPENAI_KEY is not set")
		os.Exit(1)
	}

	// 2. Send OCR request
	location, err := common.AzureOCRRequest(azureEndpoint, azureKey, file)
	if err != nil {
		fmt.Printf("Error sending OCR request: %v\n", err)
		os.Exit(1)
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
		fmt.Println("Too many failed OCR fetch attempts")
		os.Exit(1)
	}

	fmt.Println("Successfully fetched OCR result")

	// 4. Convert OCR result to markdown
	md, err := common.Ocr2md(openaiKey, "o1-mini", ocrResult)
	if err != nil {
		fmt.Printf("Error creating markdown from OCR result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully converted OCR result to markdown")

	// 5. Extract chunks from markdown
	chunks := common.ExtractChunks(md)
	fmt.Printf("Extracted %d chunks from markdown\n", len(chunks))

	// 6. Generate embeddings for chunks
	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, chunks)
	if err != nil {
		fmt.Printf("Error generating embeddings: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d embeddings\n", len(embeddings))

	// 7. Calculate hash of markdown content
	hash := sha256.Sum256([]byte(md))
	hashString := hex.EncodeToString(hash[:])

	// 8. Set the markdown version for new cards
	markdownVersion := 1
	
	// 9. Upload the markdown file using the common function
	err = minioClient.UploadMarkdownForCard(cardID, int32(markdownVersion), []byte(md))
	if err != nil {
		fmt.Printf("Error uploading markdown file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully uploaded markdown file for card %d, version %d\n", cardID, markdownVersion)

	// 12. Store the markdown hash in the database
	err = queries.CreateMarkdown(context.Background(), database.CreateMarkdownParams{
		CardID: cardID,
		Ver:    int32(markdownVersion),
		Hash:   hashString, // Using Content field to store hash until database code is regenerated
	})

	if err != nil {
		fmt.Printf("Error storing markdown hash in database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully stored markdown hash in database for card %d, version %d\n", cardID, markdownVersion)

	// 13. Store embeddings in the database
	for i, embedding := range embeddings {

		if strings.TrimSpace(chunks[i]) == "" {
			continue
		}

		// fmt.Printf("Storing embedding %d in database, %s\n", i, chunks[i])

		// Convert []float64 to []float32
		float32Embedding := make([]float32, len(embedding))
		for j, val := range embedding {
			float32Embedding[j] = float32(val)
		}
		pgvEmbed := pgvector.NewVector(float32Embedding)
		err = queries.CreateEmbeddings(context.Background(), database.CreateEmbeddingsParams{
			CardID:    cardID,
			Ver:       int32(markdownVersion),
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

	fmt.Printf("Successfully stored %d embeddings in database for card %d, version %d\n", len(embeddings), cardID, markdownVersion)
	fmt.Println("Upload process completed successfully!")
}
