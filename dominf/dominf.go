// dominf.go - Suite rápida en Go interactiva
package main

import (
    "bufio"
    "context"
    "crypto/tls"
    "fmt"
    "net"
    "os"
    "os/exec"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
    "github.com/fatih/color"
    "io"
)

// resolver DNS público
var resolver = &net.Resolver{
    PreferGo: true,
    Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
        d := &net.Dialer{Timeout: 3 * time.Second}
        return d.DialContext(ctx, network, "8.8.8.8:53")
    },
}

func runCmd(name string, args ...string) string {
    out, err := exec.Command(name, args...).CombinedOutput()
    if err != nil {
        return fmt.Sprintf("error: %v", err)
    }
    return string(out)
}

func resolve(domain, rtype string) []string {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    var out []string
    if rtype == "A" {
        ips, _ := resolver.LookupIP(ctx, "ip4", domain)
        for _, ip := range ips {
            out = append(out, ip.String())
        }
    } else {
        ips, _ := resolver.LookupIP(ctx, "ip6", domain)
        for _, ip := range ips {
            out = append(out, ip.String())
        }
    }
    return out
}

func ptrLookup(ip string) []string {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    hosts, _ := resolver.LookupAddr(ctx, ip)
    return hosts
}

func bypassCF(domain string) {
    fmt.Printf("[!] Bypass Cloudflare: %s\n", domain)
    subs := []string{"direct","ftp","mail","smtp","dev","origin","beta","panel"}
    found := map[string]struct{}{}
    for _, d := range append([]string{domain}, subs...) {
        for _, t := range []string{"A","AAAA"} {
            for _, ip := range resolve(d, t) {
                fmt.Printf("  %s %s -> %s\n", t, d, ip)
                found[ip] = struct{}{}
            }
        }
    }
    func() {
        conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", domain+":443", &tls.Config{ServerName: domain, InsecureSkipVerify: true})
        if err != nil { return }
        defer conn.Close()
        for _, ip := range conn.ConnectionState().PeerCertificates[0].IPAddresses {
            fmt.Printf("  cert IP: %s\n", ip)
            found[ip.String()] = struct{}{}
        }
    }()
    if len(found) == 0 {
        fmt.Println("  No exposed IP found")
    } else {
        fmt.Println("[✓] Possible origin IPs:")
        for ip := range found {
            fmt.Println("   ", ip)
        }
    }
}

func portScan(host string, ports []int) []int {
    var open []int
    var wg sync.WaitGroup
    var mu sync.Mutex
    for _, p := range ports {
        wg.Add(1)
        go func(port int) {
            defer wg.Done()
            conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 800*time.Millisecond)
            if err == nil {
                conn.Close()
                mu.Lock()
                open = append(open, port)
                mu.Unlock()
            }
        }(p)
    }
    wg.Wait()
    sort.Ints(open)
    return open
}

func bannerGrab(host string, port int) string {
    conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
    if err != nil { return "" }
    defer conn.Close()

    if port == 443 {
        tlsConn := tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: true})
        defer tlsConn.Close()
        tlsConn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
        buf := make([]byte, 512)
        n, _ := tlsConn.Read(buf)
        return string(buf[:n])
    } else if port == 80 {
        conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
    }
    
    buf := make([]byte, 512)
    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    n, _ := conn.Read(buf)
    return string(buf[:n])
}


func subBrute(domain, wordlist string) {
    file, err := os.Open(wordlist)
    if err != nil {
        fmt.Println("error opening wordlist:", err)
        return
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        sub := scanner.Text()
        full := sub + "." + domain
        if ips := resolve(full, "A"); len(ips) > 0 {
            for _, ip := range ips {
                fmt.Printf("[+] %s -> %s\n", full, ip)
            }
        }
    }
}

func parsePorts(expr string) []int {
    var out []int
    for _, part := range strings.Split(expr, ",") {
        part = strings.TrimSpace(part)
        if strings.Contains(part, "-") {
            rng := strings.SplitN(part, "-", 2)
            a, _ := strconv.Atoi(rng[0])
            b, _ := strconv.Atoi(rng[1])
            for i := a; i <= b; i++ {
                out = append(out, i)
            }
        } else if part != "" {
            x, _ := strconv.Atoi(part)
            out = append(out, x)
        }
    }
    return out
}

func main() {
    if len(os.Args) != 2 {
        color.Red("Usage: %s dominio.com\n", os.Args[0])
        os.Exit(1)
    }
    domain := os.Args[1]
    logFileName := domain + "_dominf.txt"
    logFile, err := os.Create(logFileName)
    if err != nil {
        color.Red("No se pudo crear el archivo de log: %v\n", err)
        os.Exit(1)
    }
    defer logFile.Close()
    mw := io.MultiWriter(os.Stdout, logFile)
    color.Output = mw
    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Println("")
        color.White("--------------------------------------------------\n")
        color.Cyan("Menu:\n 1) Bypass Cloudflare\n 2) Light port scan\n 3) Banner grabbing\n 4) PTR (reverse lookup)\n 5) Real IP (dig, curl headers)\n 6) Zone transfer AXFR\n 7) Subdomain brute-force\n 8) DNS resolution\n 0) Exit\n")
        color.Yellow("\nSelect option: ")
        inp, _ := reader.ReadString('\n')
        opt, err := strconv.Atoi(strings.TrimSpace(inp))
        if err != nil {
            color.Red("\nInvalid option\n")
            continue
        }
        color.Magenta("\n--- Selected option: %d ---\n", opt)
        switch opt {
        case 0:
            color.White("--------------------------------------------------\n")
            color.Green("\nExiting.\n")
            color.Magenta("\n[✓] Results saved to %s\n", logFileName)
            os.Exit(0)
        case 1:
            color.White("--------------------------------------------------\n")
            color.Blue("\n[!] Bypass Cloudflare: %s\n", domain)
            subs := []string{"direct","ftp","mail","smtp","dev","origin","beta","panel"}
            found := map[string]struct{}{}
            for _, d := range append([]string{domain}, subs...) {
                for _, t := range []string{"A","AAAA"} {
                    for _, ip := range resolve(d, t) {
                        color.Yellow("  %s %s -> %s\n", t, d, ip)
                        found[ip] = struct{}{}
                    }
                }
            }
            func() {
                conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", domain+":443", &tls.Config{ServerName: domain, InsecureSkipVerify: true})
                if err != nil { return }
                defer conn.Close()
                for _, ip := range conn.ConnectionState().PeerCertificates[0].IPAddresses {
                    color.Yellow("  cert IP: %s\n", ip)
                    found[ip.String()] = struct{}{}
                }
            }()
            if len(found) == 0 {
                color.Red("  No exposed IP found\n")
            } else {
                color.Green("[✓] Possible origin IPs:")
                for ip := range found {
                    color.White("   %s\n", ip)
                }
            }
        case 2:
            color.White("--------------------------------------------------\n")
            color.Yellow("\nPorts (e.g. 80,443 or 22,80,443) [default: 21,22,23,25,53,80,110,143,443,3306,3389,5900]\n(NO ranges like 1-1000, specify ports separated by commas): ")
            pstr, _ := reader.ReadString('\n')
            ports := parsePorts(strings.TrimSpace(pstr))
            if len(ports) == 0 {
                ports = []int{21,22,23,25,53,80,110,143,443,3306,3389,5900}
            }
            open := portScan(domain, ports)
            if len(open) == 0 {
                color.Red("[×] No open ports\n")
            } else {
                for _, p := range open {
                    color.Green("[+] open %d\n", p)
                }
            }
        
        case 3:
            color.White("--------------------------------------------------\n")
            color.Yellow("\nPorts (e.g. 80,443 or Enter for common: 21,22,23,25,53,80,110,143,443,3306,3389,5900): ")
            pstr, _ := reader.ReadString('\n')
            pstr = strings.TrimSpace(pstr)
            var ports []int
            if pstr == "" {
                ports = []int{21,22,23,25,53,80,110,143,443,3306,3389,5900}
            } else {
                ports = parsePorts(pstr)
            }
            openPorts := portScan(domain, ports)
            if len(openPorts) == 0 {
                color.Red("[×] No open ports\n")
            } else {
                color.Green("[✓] Open ports detected: %v\n", openPorts)
                for _, p := range openPorts {
                    color.White("--------------------------------------------------\n")
                    banner := bannerGrab(domain, p)
                    color.White("-- %d --\n%s\n", p, banner)
                }
            }
        case 4:
            color.White("--------------------------------------------------\n")
            for _, ip := range resolve(domain, "A") {
                for _, h := range ptrLookup(ip) {
                    color.Green("[+] PTR: %s\n", h)
                }
            }
        case 5:
            color.White("--------------------------------------------------\n")
            dig := runCmd("dig", "+short", domain)
            curl := runCmd("curl", "-sI", domain)
            color.Cyan("[dig A+AAAA]\n%s\n[curl -I]\n%s\n", dig, curl)
        case 6:
            color.White("--------------------------------------------------\n")
            axfr := runCmd("dig", "AXFR", domain, "@8.8.8.8")
            color.Cyan("%s\n", axfr)
        case 7:
            color.White("--------------------------------------------------\n")
            color.Yellow("\nWordlist (e.g. wordlist.txt): ")
            w, _ := reader.ReadString('\n')
            file, err := os.Open(strings.TrimSpace(w))
            if err != nil {
                color.Red("error opening wordlist: %v\n", err)
                break
            }
            defer file.Close()
            scanner := bufio.NewScanner(file)
            for scanner.Scan() {
                sub := scanner.Text()
                full := sub + "." + domain
                if ips := resolve(full, "A"); len(ips) > 0 {
                    for _, ip := range ips {
                        color.Green("[+] %s -> %s\n", full, ip)
                    }
                }
            }
        case 8:
            color.White("--------------------------------------------------\n")
            for _, t := range []string{"A", "AAAA"} {
                for _, ip := range resolve(domain, t) {
                    color.Cyan("%s: %s\n", t, ip)
                }
            }
        default:
            color.White("--------------------------------------------------\n")
            color.Red("Invalid option\n")
        }
    }
}
