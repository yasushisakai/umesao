package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
)

// ocr2md sends an OCR result to OpenAI's API and returns the formatted Markdown output.
// Parameters:
//
//	key   - OpenAI API key.
//	model - The model to use (e.g., "o1-mini").
//	ocr   - OCR result text as a JSON string.
//
// Returns:
//
//	A string containing the formatted markdown and an error if any occurred.
func Ocr2md(key, model, ocr string) (string, error) {
	// OpenAI API endpoint
	url := "https://api.openai.com/v1/chat/completions"

	// Define the request payload
	reqPayload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "assistant",
				"content": "You are a helpful assistant. Please output only the final Markdown without any additional explanation or commentary. Even the code block(triple single quotes) that indicates this is a markdown is unwanted.",
			},
			{
				"role":    "user",
				"content": "Reconstruct the following OCR file into a Markdown file. If parts of the output look like an error, delete or modify them. You might need to change the heading or create lists or even tables. Here is the OCR result:\n\n" + ocr,
			},
		},
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", errors.New("API request failed: " + string(bodyBytes))
	}

	// Parse response JSON
	var resPayload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`

			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resPayload); err != nil {
		return "", err
	}

	// Extract Markdown content
	if len(resPayload.Choices) == 0 {
		return "", errors.New("no valid response from API")
	}

	if resPayload.Choices[0].FinishReason != "stop" {
		return "", errors.New("finish reason is not 'stop.'")
	}

	return resPayload.Choices[0].Message.Content, nil
}

type EmbeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

/* sorting by index */
type ByIndex []EmbeddingData

func (a ByIndex) Len() int           { return len(a) }
func (a ByIndex) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByIndex) Less(i, j int) bool { return a[i].Index < a[j].Index }

/* calculate a list of embeddings data from a list of strings */
func LineEmbeddings(key, model string, dimension uint, texts []string) ([][]float64, error) {

	url := "https://api.openai.com/v1/embeddings"

	reqPayload := map[string]interface{}{
		"input":           texts,
		"model":           model,
		"encoding_format": "float",
		"dimension":       dimension,
	}

	jsonData, err := json.Marshal(reqPayload)

	if err != nil {
		return [][]float64{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))

	if err != nil {
		return [][]float64{}, err
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	var resPayload struct {
		Data []EmbeddingData `json:"data"`
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return [][]float64{}, err
	}

	// sort
	if err := json.NewDecoder(resp.Body).Decode(&resPayload); err != nil {
		return [][]float64{}, err
	}

	data := resPayload.Data
	sort.Sort(ByIndex(data))

	result := make([][]float64, len(data))
	for i, eData := range data {
		result[i] = eData.Embedding
	}

	return result, nil
}
