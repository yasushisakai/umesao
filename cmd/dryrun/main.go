package main

import (
	"fmt"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/yasushisakai/umesao/pkg/common"
)

func main() {

	if len(os.Args) != 2 {
		fmt.Println("Usage: add <image file>")
		os.Exit(1)
	}

	file := os.Args[1]

	endpoint := os.Getenv("AZURE_ENDPOINT")

	if endpoint == "" {
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

	location, err := common.AzureOCRRequest(endpoint, azureKey, file)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var ocrResult string
	attempt := 3

	for {
		time.Sleep(3 * time.Second)
		ocrResult, err = common.AzureOCRFetchResult(location, azureKey)
		if err != nil && attempt > 0 {
			fmt.Printf("did not succeed: %s\nretry in 3sec.", err)
			attempt = attempt - 1
		} else {
			break
		}
	}

	fmt.Printf("ocr:\n\t%s\n", ocrResult)

	if attempt < 0 {
		fmt.Println("too many failed attempts")
		os.Exit(1)
	}

	md, err := common.Ocr2md(openaiKey, "o1-mini", ocrResult)

	if err != nil {
		fmt.Printf("error creating md from ocr result: %s", err)
		os.Exit(1)
	}

	fmt.Printf("markdown:\n\t%s\n", md)

	chunks := common.ExtractChunks(md)

	embeddings, err := common.LineEmbeddings(openaiKey, "text-embedding-3-small", 1024, chunks)

	for i, e := range embeddings {
		fmt.Printf("%3d: %v\n", i, e[:10])
	}

}
