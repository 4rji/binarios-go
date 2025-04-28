package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

const dbPath = "/opt/4rji/GeoLite2-City.mmdb"

func checkDatabase() {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("La base de datos en %s no existe.\n", dbPath)
		os.Exit(1)
	}
}

func printFullRecord(ipStr string) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ip := net.ParseIP(ipStr)
	record, err := db.City(ip)
	if err != nil {
		log.Fatal(err)
	}

	region := "Unknown"
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}
	fmt.Printf("[*] Target: %s Geo-located.\n", ipStr)
	fmt.Printf("[+] %s, %s, %s\n", record.City.Names["en"], region, record.Country.Names["en"])
	fmt.Printf("[+] Latitude: %f, Longitude: %f\n", record.Location.Latitude, record.Location.Longitude)
}

func printCityOnly(ipStr string) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ip := net.ParseIP(ipStr)
	record, err := db.City(ip)
	if err != nil {
		log.Fatal(err)
	}

	city := record.City.Names["en"]
	region := "Unknown"
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}
	country := record.Country.Names["en"]

	if city == "" && region == "" && country == "" {
		fmt.Printf("[+] %s -> Información no disponible\n", ipStr)
	} else {
		fmt.Printf("[+] %s -> %s, %s, %s\n", ipStr, city, region, country)
	}
}

func main() {
	checkDatabase()

	args := os.Args[1:]
	if len(args) == 1 {
		arg := args[0]
		if _, err := os.Stat(arg); err == nil {
			file, err := os.Open(arg)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				ip := strings.TrimSpace(scanner.Text())
				if ip != "" {
					printCityOnly(ip)
				}
			}
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
		} else {
			printFullRecord(arg)
		}
	} else {
		if _, err := os.Stat("ips.txt"); os.IsNotExist(err) {
			fmt.Println("ips.txt no encontrado.")
			os.Exit(1)
		}
		file, err := os.Open("ips.txt")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			ip := strings.TrimSpace(scanner.Text())
			if ip != "" {
				printCityOnly(ip)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}
}
