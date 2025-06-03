package main

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

// Descriptions is a map of script names to their detailed descriptions
type Descriptions map[string]DetailedDescription

// Icons for different script types
var scriptIcons = map[string]string{
	"net":     "•",
	"system":  "•",
	"file":    "•",
	"user":    "•",
	"config":  "•",
	"default": "•",
}
