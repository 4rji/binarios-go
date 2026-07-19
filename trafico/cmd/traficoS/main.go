package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	domains = []string{
		"facebook.com",
		"twitter.com",
		"instagram.com",
		"linkedin.com",
		"gmail.com",
		"outlook.com",
		"yahoo.com",
		"youtube.com",
		"tiktok.com",
		"reddit.com",
		"snapchat.com",
		"pinterest.com",
		"threads.net",
	}

	client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
		},
		Timeout: 0,
	}
)

func makeRequest(url string) {
	_, _ = client.Get(url)
}

func randomHTTPSURL() string {
	domain := domains[rand.Intn(len(domains))]
	return fmt.Sprintf("https://%s", domain)
}

func worker(speed time.Duration, wg *sync.WaitGroup, stop <-chan struct{}) {
	defer wg.Done()
	for {
		select {
		case <-stop:
			return
		default:
			url := randomHTTPSURL()
			go makeRequest(url)
			fmt.Printf("\033[1;34m[REQUEST] %s\033[0m\n", url)
			if speed > 0 {
				time.Sleep(speed)
			}
		}
	}
}

func main() {
	fmt.Println("\033[1;32m==============================")
	fmt.Println("  Concurrent HTTPS Traffic Simulator")
	fmt.Println("==============================\033[0m")
	fmt.Println("\033[1;33mINFO:")
	fmt.Println("- Uses predefined domains only (HTTPS).")
	fmt.Println("- Every request goes to test$RANDOM subdomains.")
	fmt.Println("- Workers are goroutines making requests in parallel.")
	fmt.Println("- Default: 100 workers")
	fmt.Println("- Minimum: 1 (one at a time)")
	fmt.Println("- Recommended: 50-200")
	fmt.Println("- Max tested: ~1000 (depends on system)\033[0m\n")

	rand.Seed(time.Now().UnixNano())
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("\033[1;36m> Session duration in minutes (0 = infinite): \033[0m")
	dStr, _ := reader.ReadString('\n')
	dStr = strings.TrimSpace(dStr)
	if dStr == "" {
		dStr = "0"
	}
	dur, _ := strconv.Atoi(dStr)

	fmt.Print("\033[1;36m> Delay between requests in seconds (0 = no delay): \033[0m")
	sStr, _ := reader.ReadString('\n')
	sStr = strings.TrimSpace(sStr)
	if sStr == "" {
		sStr = "0"
	}
	delay, _ := strconv.ParseFloat(sStr, 64)

	fmt.Print("\033[1;36m> Number of concurrent workers [Default: 100]: \033[0m")
	wStr, _ := reader.ReadString('\n')
	wStr = strings.TrimSpace(wStr)
	if wStr == "" {
		wStr = "100"
	}
	workers, _ := strconv.Atoi(wStr)

	fmt.Printf("\n\033[1;32m[STARTING] Launching %d workers...\033[0m\n\n", workers)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(time.Duration(delay*float64(time.Second)), &wg, stop)
	}

	if dur > 0 {
		time.Sleep(time.Duration(dur) * time.Minute)
		close(stop)
	} else {
		select {}
	}
	wg.Wait()
}
