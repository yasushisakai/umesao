package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yasushisakai/umesao/pkg/common"
)

// deleteImpl implements the delete command functionality
func deleteImpl(cardID int, quiet bool) error {
	// Initialize database connection
	dbpool, queries, err := common.InitDB()
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Display card information before deletion to confirm
	if !quiet {
		fmt.Printf("You are about to delete card %d and all associated data.\n", cardID)
	}

	// Try to get the image info for this card
	imageInfo, err := queries.GetCardImage(context.Background(), int32(cardID))
	if err == nil {
		if !quiet {
			fmt.Printf("Card %d has image: %s (method: %s)\n", cardID, imageInfo.Filename, imageInfo.Method)
		}
	} else {
		fmt.Printf("Note: Could not find image for card %d: %v\n", cardID, err)
	}

	// Try to get the latest markdown version for the card
	latestVersion, err := queries.GetLatestMarkdownVersion(context.Background(), int32(cardID))
	if err == nil {
		if !quiet {
			fmt.Printf("Card %d has markdown version: %d\n", cardID, latestVersion)
		}
	} else {
		fmt.Printf("Note: Could not find markdown for card %d: %v\n", cardID, err)
	}

	// Ask for confirmation, if quiet is on, assume yes
	if !quiet {
		fmt.Print("Are you sure you want to delete this card? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}

		// Check user confirmation
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Initialize Minio client to delete files
	minioClient, err := common.NewMinioClient()
	if err != nil {
		return fmt.Errorf("error initializing Minio client: %v", err)
	}

	// Try to delete image file if it exists
	if imageInfo.Filename != "" {
		if !quiet {
			fmt.Printf("Deleting image file: %s\n", imageInfo.Filename)
		}
		err := minioClient.DeleteFileFromMinio(minioClient.ImageBucket, imageInfo.Filename)
		if err != nil && !quiet {
			fmt.Printf("Warning: Failed to delete image file %s: %v\n", imageInfo.Filename, err)
		}
	}

	// Try to delete all markdown files for this card if any exist
	if latestVersion > 0 {
		if !quiet {
			fmt.Printf("Deleting markdown files for card %d (versions 1-%d)\n", cardID, latestVersion)
		}
		
		// Delete each version
		for version := int32(1); version <= latestVersion; version++ {
			markdownFileName := fmt.Sprintf("%d_%d.md", cardID, version)
			err := minioClient.DeleteFileFromMinio(minioClient.MarkdownBucket, markdownFileName)
			if err != nil && !quiet {
				fmt.Printf("Warning: Failed to delete markdown file %s: %v\n", markdownFileName, err)
			}
		}
	}

	// Delete the card (cascade deletion will take care of database records)
	err = queries.DeleteCard(context.Background(), int32(cardID))
	if err != nil {
		return fmt.Errorf("error deleting card: %v", err)
	}

	fmt.Printf("Deleted card %d and all associated data.\n", cardID)
	return nil
}

