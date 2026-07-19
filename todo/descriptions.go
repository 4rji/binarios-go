package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func (d Descriptions) Lookup(name string) (DetailedDescription, bool) {
	if d == nil {
		return DetailedDescription{}, false
	}

	if desc, ok := d[name]; ok {
		return desc, true
	}

	lowerName := strings.ToLower(name)
	for key, desc := range d {
		if strings.EqualFold(key, name) || strings.EqualFold(desc.Name, name) || strings.ToLower(desc.Name) == lowerName {
			return desc, true
		}
	}

	return DetailedDescription{}, false
}
