package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pgvector/pgvector-go"
	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

type SearchResult struct {
	CardID   int32
	Ver      int32
	Idx      int32
	Model    string
	Text     string
	Distance float32
}

func main() {
	// Check if a search query was provided
	if len(os.Args) != 2 {
		fmt.Println("Usage: lookup <search_query>")
		os.Exit(1)
	}

	now := time.Now()
	defer func() {
		fmt.Printf("\nTime taken: %v\n", time.Since(now))
	}()

	// Get the search query from command line
	searchQuery := os.Args[1]
	fmt.Printf("Searching for: \"%s\"\n", searchQuery)

	// Get environment variables for OpenAI API
	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	common.CheckError(err, "Error getting OpenAI API key")

	// Calculate embedding for the search query
	queryEmbeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, []string{searchQuery})
	common.CheckError(err, "Error generating query embedding")

	if len(queryEmbeddings) == 0 {
		fmt.Println("No embeddings generated for the query")
		os.Exit(1)
	}

	// Convert the query embedding from []float64 to []float32 and create pgvector
	pgvQueryEmbed := pgvector.NewVector(common.ConvertFloat64ToFloat32(queryEmbeddings[0]))

	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	common.CheckError(err, "Error initializing database")
	defer dbpool.Close()

	// Check if we have any chunks in the database
	var chunkCount int
	err = dbpool.QueryRow(context.Background(), "SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	common.CheckError(err, "Error counting chunks")

	// If no chunks, exit early
	if chunkCount == 0 {
		fmt.Println("No chunks found in database. Please upload content first.")
		os.Exit(0)
	}

	// Search for the closest embeddings using only the latest version of each card
	searchResults, err := queries.SearchLatestDistance(context.Background(), database.SearchLatestDistanceParams{
		Embedding: pgvQueryEmbed,
		Limit:     10,
	})
	common.CheckError(err, "Error searching for latest embeddings")

	if len(searchResults) == 0 {
		fmt.Println("No matching results found")
		os.Exit(0)
	}

	// Convert the search results to our custom type
	var results []SearchResult

	for _, result := range searchResults {
		// Convert the distance from interface{} to float32
		var distance float32
		switch v := result.Distance.(type) {
		case float32:
			distance = v
		case float64:
			distance = float32(v)
		default:
			fmt.Printf("Unexpected distance type: %T with value: %v\n", result.Distance, result.Distance)
			distance = 0
		}

		results = append(results, SearchResult{
			CardID:   result.CardID,
			Ver:      result.Ver,
			Idx:      result.Idx,
			Model:    result.Model,
			Text:     result.Text,
			Distance: distance,
		})
	}

	// Sort the results by distance (cosine similarity)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	// Display the results
	fmt.Println("\nResults:")
	fmt.Println("\nCard\tVer\tDist\tText")
	fmt.Println("--------------------------------------------------")

	uniques := make(map[int32]bool)
	var uniqueCardIDs []int32

	for _, result := range results {
		if _, ok := uniques[result.CardID]; !ok {
			uniques[result.CardID] = true
			uniqueCardIDs = append(uniqueCardIDs, result.CardID)
			fmt.Printf("%4d\t%2d\t%5.3f\t\"%s\"\n", result.CardID, result.Ver, result.Distance, string([]rune(result.Text)[:10]))
		}
	}

	// Initialize Minio client if needed for image display
	minioClient, err := common.NewMinioClient()
	if err != nil {
		fmt.Printf("Note: Unable to initialize Minio client for image display: %v\n", err)
	} else {
		// Offer to display an image for a selected result
		fmt.Println("\nEnter a card ID to view its image (or press Enter to skip):")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := scanner.Text()
			if input != "" {
				selectedID, err := strconv.Atoi(input)
				if err != nil {
					fmt.Printf("Invalid card ID: %v\n", err)
				} else {
					// Display the selected card's image
					err = common.DisplayCardImages(int32(selectedID), queries)
					if err != nil {
						fmt.Printf("Note: %v (no image found or error displaying)\n", err)
					}
				}
			}
		}
	}
}
