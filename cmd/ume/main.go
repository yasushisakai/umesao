package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yasushisakai/umesao/pkg/common"
)

// CommandFunc is a function type for subcommands
type CommandFunc func([]string) error

// Command represents a subcommand with its name, description, and function
type Command struct {
	Name        string
	Description string
	Func        CommandFunc
}

func main() {
	// Define available commands
	commands := []Command{
		{
			Name:        "lookup",
			Description: "Search for text in the database (default if no command is specified)",
			Func:        lookupCmd,
		},
		{
			Name:        "upload",
			Description: "Upload an image file, extract text, and store the results",
			Func:        uploadCmd,
		},
		{
			Name:        "edit",
			Description: "Download and edit a card's markdown content",
			Func:        editCmd,
		},
		{
			Name:        "delete",
			Description: "Delete a card and all its associated data",
			Func:        deleteCmd,
		},
		{
			Name:        "help",
			Description: "Show help information",
			Func:        helpCmd,
		},
	}

	// If no arguments provided, show help
	if len(os.Args) < 2 {
		fmt.Println("Error: No command or search query provided")
		showHelp(commands)
		os.Exit(1)
	}

	// Get the command or search query
	cmdOrQuery := os.Args[1]

	// Check if the user is asking for help
	if cmdOrQuery == "-h" || cmdOrQuery == "--help" {
		showHelp(commands)
		return
	}

	// If asking for help about a specific command
	if cmdOrQuery == "help" && len(os.Args) > 2 {
		helpSubcommand := os.Args[2]
		switch helpSubcommand {
		case "lookup":
			fmt.Println("Usage: ume lookup <search_query>")
			fmt.Println("       ume <search_query>")
			fmt.Println("\nSearch for text in the database and display the results.")
			fmt.Println("\nThis command will:")
			fmt.Println("1. Generate an embedding for your search query")
			fmt.Println("2. Find text chunks in the database that are semantically similar")
			fmt.Println("3. Display the top matching cards")
			fmt.Println("4. Offer to display an image for a selected card")
			return
		case "upload":
			fmt.Println("Usage: ume upload [--method=ocr|vision] <image_file>")
			fmt.Println("\nUpload an image file, extract text, and store the results in the database.")
			fmt.Println("\nOptions:")
			fmt.Println("  --method=ocr     Use Azure OCR service (default)")
			fmt.Println("  --method=vision  Use OpenAI's Vision API")
			fmt.Println("\nThis command will:")
			fmt.Println("1. Upload the image to storage")
			fmt.Println("2. Extract text using the specified method (OCR or Vision)")
			fmt.Println("3. Convert the result to markdown")
			fmt.Println("4. Generate embeddings for the markdown content")
			fmt.Println("5. Store everything in the database")
			return
		case "edit":
			fmt.Println("Usage: ume edit [options] <card_id>")
			fmt.Println("\nDownload and edit a card's markdown content.")
			fmt.Println("\nOptions:")
			fmt.Println("  -v, --verbose    Enable verbose output")
			fmt.Println("\nThis command will:")
			fmt.Println("1. Download the latest markdown version for the specified card")
			fmt.Println("2. Open it in the neovim editor for you to edit")
			fmt.Println("3. If you make changes, upload the new version")
			fmt.Println("4. Generate new embeddings for the updated content")
			return
		case "delete":
			fmt.Println("Usage: ume delete [options] <card_id>")
			fmt.Println("\nDelete a card and all its associated data (images, markdown files, and embeddings).")
			fmt.Println("\nOptions:")
			fmt.Println("  -q, --quiet    Suppress confirmation and verbose output")
			fmt.Println("\nThis command will:")
			fmt.Println("1. Confirm you want to delete the card (unless --quiet is specified)")
			fmt.Println("2. Delete object files from Minio storage (images and markdown)")
			fmt.Println("3. Delete the card from the database (related data is cascade deleted)")
			return
		}
	} else if cmdOrQuery == "help" {
		showHelp(commands)
		return
	}

	// Check if the first argument is a known command
	var cmd *Command
	for i, c := range commands {
		if c.Name == cmdOrQuery {
			cmd = &commands[i]
			break
		}
	}

	// If no command is found, assume it's a search query for the lookup command
	if cmd == nil {
		// Use the lookup command with the original arguments
		cmd = &commands[0] // lookup is the first command
	}

	// Execute the command
	err := cmd.Func(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// showHelp displays the help information for all commands
func showHelp(commands []Command) {
	fmt.Printf("Usage: ume [command] [arguments]\n\n")
	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println("\nIf no command is specified, the input is treated as a search query for the lookup command.")
	fmt.Println("Example: ume \"search query\" is equivalent to ume lookup \"search query\"")
}

// helpCmd shows the help information
func helpCmd(args []string) error {
	// Get available commands by recursively calling main()
	commands := []Command{
		{
			Name:        "lookup",
			Description: "Search for text in the database (default if no command is specified)",
			Func:        lookupCmd,
		},
		{
			Name:        "upload",
			Description: "Upload an image file, extract text, and store the results",
			Func:        uploadCmd,
		},
		{
			Name:        "edit",
			Description: "Download and edit a card's markdown content",
			Func:        editCmd,
		},
		{
			Name:        "delete",
			Description: "Delete a card and all its associated data",
			Func:        deleteCmd,
		},
		{
			Name:        "help",
			Description: "Show help information",
			Func:        helpCmd,
		},
	}

	// If a specific command is specified, show help for that command
	if len(args) > 1 {
		cmdName := args[1]
		fmt.Printf("Help for command: %s\n\n", cmdName)
		for _, cmd := range commands {
			if cmd.Name == cmdName {
				switch cmdName {
				case "lookup":
					fmt.Println("Usage: ume lookup <search_query>")
					fmt.Println("       ume <search_query>")
					fmt.Println("\nSearch for text in the database and display the results.")
					fmt.Println("\nThis command will:")
					fmt.Println("1. Generate an embedding for your search query")
					fmt.Println("2. Find text chunks in the database that are semantically similar")
					fmt.Println("3. Display the top matching cards")
					fmt.Println("4. Offer to display an image for a selected card")
				case "upload":
					fmt.Println("Usage: ume upload [--method=ocr|vision] <image_file>")
					fmt.Println("\nUpload an image file, extract text, and store the results in the database.")
					fmt.Println("\nOptions:")
					fmt.Println("  --method=ocr     Use Azure OCR service (default)")
					fmt.Println("  --method=vision  Use OpenAI's Vision API")
					fmt.Println("\nThis command will:")
					fmt.Println("1. Upload the image to storage")
					fmt.Println("2. Extract text using the specified method (OCR or Vision)")
					fmt.Println("3. Convert the result to markdown")
					fmt.Println("4. Generate embeddings for the markdown content")
					fmt.Println("5. Store everything in the database")
				case "edit":
					fmt.Println("Usage: ume edit [options] <card_id>")
					fmt.Println("\nDownload and edit a card's markdown content.")
					fmt.Println("\nOptions:")
					fmt.Println("  -v, --verbose    Enable verbose output")
					fmt.Println("\nThis command will:")
					fmt.Println("1. Download the latest markdown version for the specified card")
					fmt.Println("2. Open it in the neovim editor for you to edit")
					fmt.Println("3. If you make changes, upload the new version")
					fmt.Println("4. Generate new embeddings for the updated content")
				case "delete":
					fmt.Println("Usage: ume delete [options] <card_id>")
					fmt.Println("\nDelete a card and all its associated data (images, markdown files, and embeddings).")
					fmt.Println("\nOptions:")
					fmt.Println("  -q, --quiet    Suppress confirmation and verbose output")
					fmt.Println("\nThis command will:")
					fmt.Println("1. Confirm you want to delete the card (unless --quiet is specified)")
					fmt.Println("2. Delete object files from Minio storage (images and markdown)")
					fmt.Println("3. Delete the card from the database (related data is cascade deleted)")
				}
				return nil
			}
		}
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	// Otherwise, show general help
	showHelp(commands)
	return nil
}

// lookupCmd handles the lookup command
func lookupCmd(args []string) error {
	// Process args based on whether this was called directly or as the default command
	var searchQuery string

	// If called as default (args[0] is not "lookup"), use args[0] as the search query
	if args[0] != "lookup" {
		searchQuery = args[0]
	} else if len(args) > 1 {
		// If called explicitly (args[0] is "lookup"), use args[1] as the search query
		searchQuery = args[1]
	} else {
		// Not enough arguments
		return fmt.Errorf("usage: ume lookup <search_query>\n       ume <search_query>")
	}

	fmt.Printf("Searching for: \"%s\"\n", searchQuery)

	// Initialize command-specific flags
	// (no flags for lookup currently, but structure is here for future use)
	lookupFlags := flag.NewFlagSet("lookup", flag.ExitOnError)
	// Example flag: limit := lookupFlags.Int("limit", 10, "limit the number of results")

	// Parse the flags (skipping the first argument which is the command name or search query)
	var flagArgs []string
	if args[0] == "lookup" {
		flagArgs = args[1:]
	} else {
		flagArgs = args[0:]
	}

	// Just to handle potential flags in the future
	lookupFlags.Parse(flagArgs)

	// Implement the lookup functionality (from cmd/lookup/main.go)
	// This is the actual command implementation
	return lookupImpl(searchQuery)
}

// uploadCmd handles the upload command
func uploadCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: ume upload [--method=ocr|vision] <image_file>")
	}

	// Specify upload flags
	uploadFlags := flag.NewFlagSet("upload", flag.ExitOnError)
	methodFlag := uploadFlags.String("method", "ocr", "Method to use for text extraction: ocr (default) or vision")

	// Parse flags (skipping the first argument which is the command name)
	uploadFlags.Parse(args[1:])

	// Get the file path
	filePath := uploadFlags.Arg(0)
	if filePath == "" {
		return fmt.Errorf("no file specified")
	}

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Get the absolute path of the file
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("error getting absolute path: %v", err)
	}

	// Validate method flag
	method := *methodFlag
	if method != "ocr" && method != "vision" {
		return fmt.Errorf("invalid method: %s. Must be either 'ocr' or 'vision'", method)
	}

	// Implement the upload functionality with the specified method
	return uploadImpl(absPath, method)
}

// deleteCmd handles the delete command
func deleteCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: ume delete [options] <card_id>")
	}

	// No flags for delete command
	deleteFlags := flag.NewFlagSet("delete", flag.ExitOnError)
	quietFlag := deleteFlags.Bool("q", false, "Surpress verbose output")
	quietLongFlag := deleteFlags.Bool("quiet", false, "Surpress verbose output")

	// Parse flags (skipping the first argument which is the command name)
	deleteFlags.Parse(args[1:])

	// Get the card ID
	cardIDStr := deleteFlags.Arg(0)
	if cardIDStr == "" {
		return fmt.Errorf("no card ID specified")
	}

	// Parse the card ID
	cardID, err := common.ParseCardIDString(cardIDStr)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	// Check if either quiet flag is set
	quiet := *quietFlag || *quietLongFlag

	// Implement the delete functionality
	return deleteImpl(cardID, quiet)
}

// editCmd handles the edit command
func editCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: ume edit [options] <card_id>")
	}

	// Specify edit flags
	editFlags := flag.NewFlagSet("edit", flag.ExitOnError)
	verboseFlag := editFlags.Bool("v", false, "Enable verbose output")
	verboseLongFlag := editFlags.Bool("verbose", false, "Enable verbose output")

	// Parse flags (skipping the first argument which is the command name)
	editFlags.Parse(args[1:])

	// Get the card ID
	cardIDStr := editFlags.Arg(0)
	if cardIDStr == "" {
		return fmt.Errorf("no card ID specified")
	}

	// Parse the card ID
	cardID, err := common.ParseCardIDString(cardIDStr)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	// Check if either verbose flag is set
	verbose := *verboseFlag || *verboseLongFlag

	// Implement the edit functionality with verbose flag
	return editImpl(cardID, verbose)
}

// Implementation functions are defined in separate files:
// - lookup.go: lookupImpl
// - upload.go: uploadImpl
// - edit.go:   editImpl
// - delete.go: deleteImpl
