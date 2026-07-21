package main

import (
	"encoding/json"
	"strings"
)

// Script represents a script with its name and description
type Script struct {
	Name string
	Desc string
}

// DetailedDescription represents a detailed description of a script
type DetailedDescription struct {
	Name         string `json:"name"`
	ShortDesc    string `json:"short_desc"`
	DetailedDesc string `json:"detailed_desc"`
}

func (d *DetailedDescription) UnmarshalJSON(data []byte) error {
	type rawDetailedDescription struct {
		Name         string          `json:"name"`
		ShortDesc    string          `json:"short_desc"`
		DetailedDesc json.RawMessage `json:"detailed_desc"`
	}

	var raw rawDetailedDescription
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Name = raw.Name
	d.ShortDesc = raw.ShortDesc

	if len(raw.DetailedDesc) == 0 || string(raw.DetailedDesc) == "null" {
		return nil
	}

	var single string
	if err := json.Unmarshal(raw.DetailedDesc, &single); err == nil {
		d.DetailedDesc = single
		return nil
	}

	var multi []string
	if err := json.Unmarshal(raw.DetailedDesc, &multi); err == nil {
		d.DetailedDesc = strings.Join(multi, "\n\n")
		return nil
	}

	return nil
}

// Descriptions is a map of script names to their detailed descriptions
type Descriptions map[string]DetailedDescription
