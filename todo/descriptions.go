package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// loadDescriptions loads script descriptions from the JSON file
func loadDescriptions() (Descriptions, error) {
	file, err := os.Open("/opt/4rji/bin/descriptions.json")
	if err != nil {
		return nil, fmt.Errorf("error opening descriptions.json: %v", err)
	}
	defer file.Close()

	var descriptions Descriptions
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&descriptions); err != nil {
		return nil, fmt.Errorf("error decoding descriptions.json: %v", err)
	}

	return descriptions, nil
}
