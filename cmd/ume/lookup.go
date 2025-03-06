package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/yasushisakai/umesao/database"
	"github.com/yasushisakai/umesao/pkg/common"
)

// SearchResult represents a search result with distance
type SearchResult struct {
	CardID   int32
	Ver      int32
	Idx      int32
	Model    string
	Text     string
	Distance float32
}

// lookupImpl implements the lookup command functionality
func lookupImpl(searchQuery string) error {
	now := time.Now()

	// Get environment variables for OpenAI API
	openaiKey, err := common.RequireEnvVar("OPENAI_KEY")
	if err != nil {
		return fmt.Errorf("error getting OpenAI API key: %v", err)
	}

	// Calculate embedding for the search query
	queryEmbeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1536, []string{searchQuery})
	if err != nil {
		return fmt.Errorf("error generating query embedding: %v", err)
	}

	if len(queryEmbeddings) == 0 {
		return fmt.Errorf("no embeddings generated for the query")
	}

	// Convert the query embedding to pgvector
	pgvQueryEmbed := common.EmbeddingToPGVector(queryEmbeddings[0])

	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Check if we have any chunks in the database
	var chunkCount int
	err = dbpool.QueryRow(context.Background(), "SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	if err != nil {
		return fmt.Errorf("error counting chunks: %v", err)
	}

	// If no chunks, exit early
	if chunkCount == 0 {
		return fmt.Errorf("no chunks found in database. Please upload content first")
	}

	// Search for the closest embeddings using only the latest version of each card
	searchResults, err := queries.SearchLatestDistance(context.Background(), database.SearchLatestDistanceParams{
		Embedding: pgvQueryEmbed,
		Limit:     10,
	})
	if err != nil {
		return fmt.Errorf("error searching for latest embeddings: %v", err)
	}

	if len(searchResults) == 0 {
		return fmt.Errorf("no matching results found")
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

	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}

	// Display the results
	fmt.Println("\nResults:")
	fmt.Println("\nCard\tVer\tDist\tText")
	fmt.Println("------------------------------------------------------------------------------")

	uniques := make(map[int32]bool)
	var uniqueCardIDs []int32

	for _, result := range results {
		if _, ok := uniques[result.CardID]; !ok {
			uniques[result.CardID] = true
			uniqueCardIDs = append(uniqueCardIDs, result.CardID)

			fmt.Printf("%4d\t%2d\t%5.3f\t\"%s\"\n",
				result.CardID,
				result.Ver,
				result.Distance,
				string([]rune(result.Text)[:10]))
		}
	}

	fmt.Printf("\nTime taken: %v\n", time.Since(now))

	return nil
}
