package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Configuration for included categories and excluded terms
var (
	includedCategories = []string{"Pet", "House Floor", "House Wallpaper", "House", "NPC Skin"}
	excludeWords       = []string{"Quest", "DONT", "Don't", "Bug"}
)

// Item represents a filtered item from the CSV
type Item struct {
	ID       string
	Name     string
	Category string
}

// Map of item ID to name for JSON output
type ItemMap map[string]string

func main() {
	// Setup command-line flags
	inputFile := flag.String("file", "", "Path to the CSV file")
	outputDir := flag.String("outdir", "output", "Directory to save the JSON files")
	flag.Parse()

	// Check if input file was provided
	if *inputFile == "" {
		fmt.Println("Please provide an input CSV file with -file flag")
		fmt.Println("Example: ./csv_parser -file items.csv [-outdir output_directory]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Process the CSV file
	items, err := processCSV(*inputFile)
	if err != nil {
		fmt.Printf("Error processing CSV: %v\n", err)
		os.Exit(1)
	}

	// Sort items by ID
	sortItemsByID(items)

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Convert all items to the map format and save
	allItemsMap := convertItemsToMap(items)
	allItemsPath := filepath.Join(*outputDir, "allitems.json")
	if err := saveMapToJSON(allItemsPath, allItemsMap); err != nil {
		fmt.Printf("Error saving all items: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("All items saved to %s\n", allItemsPath)

	// Group items by category
	categoryMap := make(map[string][]Item)
	for _, item := range items {
		categoryMap[item.Category] = append(categoryMap[item.Category], item)
	}

	// Save each category to its own JSON file
	for category, categoryItems := range categoryMap {
		// Create a valid filename from the category
		filename := sanitizeFilename(category) + ".json"
		categoryPath := filepath.Join(*outputDir, filename)
		
		// Convert category items to map and save
		itemMap := convertItemsToMap(categoryItems)
		if err := saveMapToJSON(categoryPath, itemMap); err != nil {
			fmt.Printf("Error saving category %s: %v\n", category, err)
			continue
		}
		fmt.Printf("Category '%s' saved to %s (%d items)\n", category, categoryPath, len(categoryItems))
	}

	// Display summary
	fmt.Printf("\nTotal items processed: %d\n", len(items))
	fmt.Printf("Items exported to %d category files\n", len(categoryMap))
}

// processCSV reads and filters items from the CSV file
func processCSV(filePath string) ([]Item, error) {
	// Open the CSV file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	
	// Read the header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	// Find the column indices
	var itemIDIndex, nameIndex, categoryIndex int = -1, -1, -1
	for i, column := range header {
		trimmedColumn := strings.TrimSpace(column)
		if trimmedColumn == "Item ID" {
			itemIDIndex = i
		} else if trimmedColumn == "Name" {
			nameIndex = i
		} else if trimmedColumn == "Category" {
			categoryIndex = i
		}
	}

	// Verify that we found all required columns
	if itemIDIndex == -1 || nameIndex == -1 || categoryIndex == -1 {
		return nil, fmt.Errorf("could not find required columns (Item ID, Name, Category) in the CSV")
	}

	// Process each row and filter according to criteria
	var items []Item
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading row: %w", err)
		}

		// Skip if row doesn't have enough columns
		if len(record) <= max(itemIDIndex, nameIndex, categoryIndex) {
			continue
		}

		// Get field values
		itemID := strings.TrimSpace(record[itemIDIndex])
		name := strings.TrimSpace(record[nameIndex])
		category := strings.TrimSpace(record[categoryIndex])

		// Skip if any field is empty
		if itemID == "" || name == "" || category == "" {
			continue
		}

		// Check if category is in the included list
		categoryIncluded := false
		for _, includedCategory := range includedCategories {
			if category == includedCategory {
				categoryIncluded = true
				break
			}
		}
		if !categoryIncluded {
			continue
		}

		// Check if name contains any excluded words
		nameContainsExcluded := false
		for _, word := range excludeWords {
			if strings.Contains(name, word) {
				nameContainsExcluded = true
				break
			}
		}
		if nameContainsExcluded {
			continue
		}

		// Add the item to our results
		items = append(items, Item{
			ID:       itemID,
			Name:     name,
			Category: category,
		})
	}

	return items, nil
}

// convertItemsToMap converts a slice of items to a map with ID as key and Name as value
func convertItemsToMap(items []Item) ItemMap {
	result := make(ItemMap)
	for _, item := range items {
		result[item.ID] = item.Name
	}
	return result
}

// saveMapToJSON writes the item map to a JSON file
func saveMapToJSON(filePath string, itemMap ItemMap) error {
	// Marshal the items to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(itemMap, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Write to file
	return os.WriteFile(filePath, jsonData, 0644)
}

// sortItemsByID sorts the items by their ID
func sortItemsByID(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		// Try to convert to integers for numeric comparison
		idI, errI := strconv.Atoi(items[i].ID)
		idJ, errJ := strconv.Atoi(items[j].ID)
		
		// If both can be converted to integers, compare numerically
		if errI == nil && errJ == nil {
			return idI < idJ
		}
		
		// Otherwise, compare as strings
		return items[i].ID < items[j].ID
	})
}

// sanitizeFilename creates a valid filename from a string
func sanitizeFilename(name string) string {
	// Replace spaces with underscores
	name = strings.ReplaceAll(name, " ", "_")
	
	// Remove any other invalid characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	
	return name
}

// max returns the maximum of the given integers
func max(values ...int) int {
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}
