package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

const dbPath = "/opt/4rji/GeoLite2-City.mmdb"

// ANSI escape codes for colors
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

func checkDatabase() {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Warning: GeoLite2 database not found at %s. Local lookup features will be unavailable.\\n", dbPath)
		// Do not exit, allow the program to continue for -i option or other fallbacks.
	}
}

func printFullRecord(ipStr string) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		fmt.Printf("Error: Could not access local GeoLite2 database to look up %s.\\n", ipStr)
		fmt.Printf("Reason: %v\\n", err)
		fmt.Printf("Please ensure the database file exists at %s and is valid.\\n", dbPath)
		fmt.Printf("Alternatively, try 'locip -i %s' for an online lookup.\\n", ipStr)
		return
	}
	defer db.Close()

	ip := net.ParseIP(ipStr)
	record, err := db.City(ip)
	if err != nil {
		fmt.Printf("Error: Could not retrieve GeoIP details for %s from the local database.\\n", ipStr)
		fmt.Printf("Reason: %v\\n", err)
		fmt.Printf("Consider using 'locip -i %s' for an alternative online lookup.\\n", ipStr)
		return
	}

	region := "Unknown"
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}
	fmt.Printf("[*] Target: %s Geo-located.\n", ipStr)
	fmt.Printf("[+] %s, %s, %s\n", record.City.Names["en"], region, record.Country.Names["en"])
	fmt.Printf("[+] Latitude: %f, Longitude: %f\n", record.Location.Latitude, record.Location.Longitude)
}

func printCityOnly(ipStr string, db *geoip2.Reader) {
	ip := net.ParseIP(ipStr)
	record, err := db.City(ip)
	if err != nil {
		fmt.Printf("[!] Could not retrieve city details for %s from local database: %v\\n", ipStr, err)
		fmt.Printf("[+] %s -> Error looking up in local DB\\n", ipStr)
		return
	}

	city := record.City.Names["en"]
	region := "Unknown"
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}
	country := record.Country.Names["en"]

	if city == "" && region == "" && country == "" {
		fmt.Printf("[+] %s -> Information not available in local DB\\n", ipStr)
	} else {
		fmt.Printf("[+] %s -> %s, %s, %s\\n", ipStr, city, region, country)
	}
}

type IPInfo struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
	Readme   string `json:"readme"`
}

func printUsage() {
	fmt.Println("Usage: locip [options] [target]")
	fmt.Println("\nLooks up geolocation information for IP addresses using a local GeoLite2 database or ipinfo.io.")
	fmt.Println("\nOptions:")
	fmt.Println("  -i [ip_address]   Query ipinfo.io for the given IP address (or your public IP if none provided).")
	fmt.Println("                    Displays detailed information including city, region, country, location, etc.")
	fmt.Println("\nTargets (Uses local GeoLite2 Database):")
	fmt.Println("  <ip_address>      Show full geolocation details (city, region, country, lat/long) for the given IP address.")
	fmt.Println("  <filepath>        Process a file containing a list of IP addresses (one per line).")
	fmt.Println("                    For each IP, shows city, region, and country.")
	fmt.Println("\nDefault Behavior (Uses local GeoLite2 Database):")
	fmt.Println("  If no arguments are provided, the script attempts to read and process 'ips.txt'")
	fmt.Println("  from the current directory. It expects one IP address per line and will show")
	fmt.Println("  city, region, and country for each.")
	fmt.Println("\nExamples:")
	fmt.Println("  locip -i 8.8.8.8       # Query ipinfo.io for 8.8.8.8")
	fmt.Println("  locip -i               # Query ipinfo.io for your public IP")
	fmt.Println("  locip 1.1.1.1          # Use local DB for full details of 1.1.1.1")
	fmt.Println("  locip my_ip_list.txt   # Use local DB to process IPs in my_ip_list.txt")
	fmt.Println("  locip                  # Use local DB to process IPs in ips.txt (if it exists)")
	fmt.Println("  locipinst              # Run this command to install/update the GeoLite2 database (if locipinst script is available)")
}

func queryIpInfo(ipAddress string) {
	url := "https://ipinfo.io/"
	if ipAddress != "" {
		url += ipAddress + "/"
	}
	url += "json"

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching IP info: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var ipInfo IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&ipInfo); err != nil {
		log.Printf("Error decoding IP info JSON: %v\n", err)
		return
	}

	fmt.Printf("%sIP:%s %s\n", ColorCyan, ColorReset, ipInfo.IP)
	if ipInfo.City != "" {
		fmt.Printf("%sCity:%s %s\n", ColorCyan, ColorReset, ipInfo.City)
	}
	if ipInfo.Region != "" {
		fmt.Printf("%sRegion:%s %s\n", ColorCyan, ColorReset, ipInfo.Region)
	}
	if ipInfo.Country != "" {
		fmt.Printf("%sCountry:%s %s\n", ColorCyan, ColorReset, ipInfo.Country)
	}
	if ipInfo.Loc != "" {
		fmt.Printf("%sLocation:%s %s\n", ColorCyan, ColorReset, ipInfo.Loc)
	}
	if ipInfo.Org != "" {
		fmt.Printf("%sOrganization:%s %s\n", ColorCyan, ColorReset, ipInfo.Org)
	}
	if ipInfo.Postal != "" {
		fmt.Printf("%sPostal Code:%s %s\n", ColorCyan, ColorReset, ipInfo.Postal)
	}
	if ipInfo.Timezone != "" {
		fmt.Printf("%sTimezone:%s %s\n", ColorCyan, ColorReset, ipInfo.Timezone)
	}
}

// processIPFile reads a file containing IP addresses (one per line)
// and prints city-only information for each valid IP using the local GeoLite2 database.
func processIPFile(filePath string) {
	// Attempt to open the database ONCE for the entire file processing.
	db, errDB := geoip2.Open(dbPath)
	if errDB != nil {
		fmt.Printf("Error: Cannot process file '%s' using the local GeoLite2 database.\\n", filePath)
		fmt.Printf("Reason: Failed to open database at '%s': %v\\n", dbPath, errDB)
		fmt.Println("Please ensure the database file exists and is valid, or use the '-i <ip>' option for individual online lookups.")
		return // Stop processing this file if DB can't be opened.
	}
	defer db.Close()

	file, err := os.Open(filePath)
	if err != nil {
		if filePath == "ips.txt" && os.IsNotExist(err) {
			fmt.Printf("Default file '%s' not found.\\n", filePath)
			printUsage() // This already exits if it's the default file and not found.
			return       // Return to be safe, though printUsage exits.
		}
		fmt.Printf("Error: Could not open IP list file '%s': %v\\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Printf("[*] Processing IPs from file: %s (using local DB)\\n", filePath)
	scanner := bufio.NewScanner(file)
	foundIPs := false
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" {
			printCityOnly(ip, db) // Pass the opened db
			foundIPs = true
		}
	}
	if !foundIPs {
		fmt.Printf("No IP addresses found in %s.\n", filePath)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error: Could not read IP list file '%s': %v\\n", filePath, err)
		os.Exit(1)
	}
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		// No arguments: Default behavior is to process ips.txt.
		// BUT, if the GeoLite2 DB is missing, just show usage and exit cleanly.
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			printUsage()
			os.Exit(0) // Exit cleanly with status 0, no warnings/errors.
		}
		// If we are here, it means no args were given AND the DB likely exists (or os.Stat had a different error).
		// Proceed with normal DB check and default file processing.
		checkDatabase() // This will be silent if DB exists, or print warning for other DB issues.
		processIPFile("ips.txt")
		return
	}

	// If arguments ARE provided, it's okay for checkDatabase() to print a warning
	// if the DB is missing, as local DB operations might be intended by the user.
	checkDatabase()

	firstArg := args[0]

	if firstArg == "-h" || firstArg == "--help" {
		printUsage()
		return
	}

	if firstArg == "-i" {
		ipToQuery := ""
		if len(args) > 1 {
			// Check if the next argument is another flag or an actual IP/domain
			if !strings.HasPrefix(args[1], "-") {
				ipToQuery = args[1]
			} else {
				// -i was given, but next arg looks like another flag, so query self IP
				// and then it will be an invalid arg combination by printUsage
			}
		}
		queryIpInfo(ipToQuery) // queryIpInfo handles empty string for self-IP
		// If -i was followed by more than one non-flag argument, or a flag after IP
		if (ipToQuery != "" && len(args) > 2) || (ipToQuery == "" && len(args) > 1) {
			// Example: locip -i 8.8.8.8 something_else OR locip -i -h
			if !(ipToQuery != "" && len(args) == 2 && (args[1] == "-h" || args[1] == "--help")) { // allow locip -i ip -h
				if !(ipToQuery == "" && len(args) == 1 && (args[0] == "-h" || args[0] == "--help")) { // allow locip -i -h
					fmt.Println("\nWarning: Extra arguments provided with -i option. Processing -i and ignoring others, or use -h for help.")
					// If it was "locip -i someotherflag", it would be an error.
					// If it was "locip -i ip someotherflag", it is also an error.
					if (ipToQuery == "" && len(args) > 1) || (ipToQuery != "" && len(args) > 2) {
						printUsage()
						os.Exit(1)
					}
				}
			}
		}
		return
	}

	// At this point, not -i, not -h, not --help, and not 0 arguments.
	// It must be a single argument: either an IP or a filepath for GeoLite2.
	if len(args) == 1 {
		target := args[0]
		// Check if the argument is a file that exists and is not a directory.
		if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
			processIPFile(target) // Process the specified file using local GeoLite2
		} else {
			// Treat as a single IP for full record using local GeoLite2
			// We can add a simple IP validation here if needed, but GeoLite2 will error out anyway.
			printFullRecord(target)
		}
		return
	}

	// If we reach here, it's an invalid combination of arguments.
	fmt.Println("Invalid arguments or combination.")
	printUsage()
	os.Exit(1)
}
