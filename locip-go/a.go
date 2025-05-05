package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/oschwald/geoip2-golang"
)

const databaseLocation = "/opt/4rji/GeoLite2-City.mmdb"

func printRecord(tgt string) {
	db, err := geoip2.Open(databaseLocation)
	if err != nil {
		log.Fatalf("Error al abrir la base de datos: %v", err)
	}
	defer db.Close()

	ip := net.ParseIP(tgt)
	if ip == nil {
		log.Fatalf("IP no válida: %s", tgt)
	}

	record, err := db.City(ip)
	if err != nil {
		log.Fatalf("Error al obtener la información de la IP: %v", err)
	}

	ciudad := record.City.Names["es"]
	if ciudad == "" {
		ciudad = record.City.Names["en"]
	}

	var region string
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["es"]
		if region == "" {
			region = record.Subdivisions[0].Names["en"]
		}
	}

	pais := record.Country.Names["es"]
	if pais == "" {
		pais = record.Country.Names["en"]
	}

	fmt.Printf("[*] Target: %s Geo-located.\n", tgt)
	fmt.Printf("[+] %s, %s, %s\n", ciudad, region, pais)
	fmt.Printf("[+] Latitude: %f, Longitude: %f\n", record.Location.Latitude, record.Location.Longitude)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./script IP-target")
		os.Exit(1)
	}
	tgt := os.Args[1]
	printRecord(tgt)
}
