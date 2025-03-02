package common

import (
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/pgvector/pgvector-go"
	_ "github.com/joho/godotenv/autoload"
)

// TestRequireEnvVar tests the RequireEnvVar function
func TestRequireEnvVar(t *testing.T) {
	// Test with existing environment variable
	os.Setenv("TEST_ENV_VAR", "test-value")
	defer os.Unsetenv("TEST_ENV_VAR")

	val, err := RequireEnvVar("TEST_ENV_VAR")
	if err != nil {
		t.Errorf("Expected no error for existing env var, got: %v", err)
	}
	if val != "test-value" {
		t.Errorf("Expected 'test-value', got: '%s'", val)
	}

	// Test with non-existent environment variable
	_, err = RequireEnvVar("NON_EXISTENT_ENV_VAR")
	if err == nil {
		t.Error("Expected error for non-existent env var, got nil")
	}
}

// TestInitDB tests the InitDB function
func TestInitDB(t *testing.T) {
	// Skip this test if DB_STRING isn't set
	if os.Getenv("DB_STRING") == "" {
		t.Skip("Skipping test because DB_STRING environment variable is not set")
	}

	dbpool, queries, err := InitDB()
	if err != nil {
		t.Fatalf("Error initializing database: %v", err)
	}
	defer dbpool.Close()

	// Check if dbpool and queries are initialized
	if dbpool == nil {
		t.Error("Expected dbpool to be initialized, got nil")
	}
	
	// Verify queries is not empty by checking that it has methods
	queriesType := reflect.TypeOf(queries)
	if queriesType.NumMethod() == 0 {
		t.Error("Expected queries to have methods, got none")
	}
}

// TestParseCardID tests the ParseCardID function
func TestParseCardID(t *testing.T) {
	// Test with valid command-line arguments
	args := []string{"program", "123"}
	cardID, err := ParseCardID(args)
	if err != nil {
		t.Errorf("Expected no error for valid args, got: %v", err)
	}
	if cardID != 123 {
		t.Errorf("Expected cardID 123, got: %d", cardID)
	}

	// Test with invalid number
	args = []string{"program", "abc"}
	_, err = ParseCardID(args)
	if err == nil {
		t.Error("Expected error for invalid card ID, got nil")
	}

	// Test with too many arguments
	args = []string{"program", "123", "extra"}
	_, err = ParseCardID(args)
	if err == nil {
		t.Error("Expected error for too many arguments, got nil")
	}
}

// TestCalculateFileHash tests the CalculateFileHash function
func TestCalculateFileHash(t *testing.T) {
	// Test with known content
	content := []byte("test content")
	hash := CalculateFileHash(content)
	
	// Expected hash for "test content"
	expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	
	if hash != expectedHash {
		t.Errorf("Expected hash '%s', got: '%s'", expectedHash, hash)
	}
	
	// Test with empty content
	emptyHash := CalculateFileHash([]byte{})
	expectedEmptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	
	if emptyHash != expectedEmptyHash {
		t.Errorf("Expected empty hash '%s', got: '%s'", expectedEmptyHash, emptyHash)
	}
}

// TestConvertFloat64ToFloat32 tests the ConvertFloat64ToFloat32 function
func TestConvertFloat64ToFloat32(t *testing.T) {
	// Test with sample embedding
	embedding := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	float32Embedding := ConvertFloat64ToFloat32(embedding)
	
	// Check conversion results
	expectedEmbedding := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	
	if len(float32Embedding) != len(expectedEmbedding) {
		t.Errorf("Expected embedding length %d, got: %d", len(expectedEmbedding), len(float32Embedding))
	}
	
	for i := range expectedEmbedding {
		if float32Embedding[i] != expectedEmbedding[i] {
			t.Errorf("Expected embedding[%d] to be %f, got: %f", i, expectedEmbedding[i], float32Embedding[i])
		}
	}
}

// TestEmbeddingToPGVector tests the EmbeddingToPGVector function
func TestEmbeddingToPGVector(t *testing.T) {
	// Test with sample embedding
	embedding := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	pgvEmbed := EmbeddingToPGVector(embedding)
	
	// Check pgvector contents
	pgvExpected := pgvector.NewVector([]float32{1.0, 2.0, 3.0, 4.0, 5.0})
	
	if !reflect.DeepEqual(pgvEmbed, pgvExpected) {
		t.Errorf("Expected pgvector embedding %v, got: %v", pgvExpected, pgvEmbed)
	}
}

// TestCheckError tests the CheckError function
func TestCheckError(t *testing.T) {
	// Redirect os.Stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	// Mock an exit function to avoid os.Exit terminating the test
	origExit := osExit
	defer func() { osExit = origExit }()
	
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic("exit") // Use panic to simulate os.Exit without terminating test
	}
	
	// Test with error
	message := "Test error message"
	err := io.EOF
	
	defer func() {
		// Recover from panic and restore stdout
		recover()
		w.Close()
		os.Stdout = oldStdout
		
		if exitCode != 1 {
			t.Errorf("Expected exit code 1, got: %d", exitCode)
		}
		
		captured := make([]byte, 100)
		n, _ := r.Read(captured)
		output := string(captured[:n])
		
		expectedOutput := "Test error message: EOF\n"
		if output != expectedOutput {
			t.Errorf("Expected output '%s', got: '%s'", expectedOutput, output)
		}
	}()
	
	CheckError(err, message)
}

// Mock osExit is declared in common.go and used for testing

// TestDisplayCardImages would require mocking the database and MinioClient
// This is a simplified version that just checks function signature
func TestDisplayCardImagesSignature(t *testing.T) {
	// Verify the function signature using reflection
	funcType := reflect.TypeOf(DisplayCardImages)
	
	if funcType.NumIn() != 2 {
		t.Errorf("Expected DisplayCardImages to have 2 parameters, got: %d", funcType.NumIn())
	}
	
	if funcType.NumOut() != 1 {
		t.Errorf("Expected DisplayCardImages to have 1 return value, got: %d", funcType.NumOut())
	}
	
	// Verify parameter types - first should be int32, second should be database.Queries
	if funcType.In(0).Kind() != reflect.Int32 {
		t.Errorf("Expected first parameter to be int32, got: %v", funcType.In(0))
	}
	
	if funcType.In(1).String() != "database.Queries" {
		t.Errorf("Expected second parameter to be database.Queries, got: %v", funcType.In(1))
	}
	
	// Verify return type - should be error
	if funcType.Out(0).String() != "error" {
		t.Errorf("Expected return type to be error, got: %v", funcType.Out(0))
	}
}