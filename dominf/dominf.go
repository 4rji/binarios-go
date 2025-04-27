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
        fmt.Println("  Ninguna IP expuesta encontrada")
    } else {
        fmt.Println("[✓] Posibles IPs de origen:")
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
        tlsConn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
        conn = tlsConn
    } else {
        conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
    }
    buf := make([]byte, 512)
    n, _ := conn.Read(buf)
    return string(buf[:n])
}

func subBrute(domain, wordlist string) {
    file, err := os.Open(wordlist)
    if err != nil {
        fmt.Println("error abrir wordlist:", err)
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
        fmt.Printf("Uso: %s dominio.com\n", os.Args[0])
        os.Exit(1)
    }
    domain := os.Args[1]
    reader := bufio.NewReader(os.Stdin)

    for {
        fmt.Println(`
Menú:
 1) Bypass Cloudflare
 2) Port scan ligero
 3) Banner grabbing
 4) PTR (reverse lookup)
 5) Real IP (dig, curl headers)
 6) Zone transfer AXFR
 7) Subdomain brute-force
 8) DNS resolution
 0) Salir`)
        fmt.Print("Seleccione opción: ")
        inp, _ := reader.ReadString('\n')
        opt, err := strconv.Atoi(strings.TrimSpace(inp))
        if err != nil {
            fmt.Println("Opción inválida")
            continue
        }
        switch opt {
        case 0:
            fmt.Println("Saliendo.")
            os.Exit(0)
        case 1:
            bypassCF(domain)
        case 2:
            fmt.Print("Puertos (ej:80,443,1-1024) [default 80,443]: ")
            pstr, _ := reader.ReadString('\n')
            ports := parsePorts(strings.TrimSpace(pstr))
            if len(ports) == 0 {
                ports = parsePorts("80,443")
            }
            open := portScan(domain, ports)
            if len(open) == 0 {
                fmt.Println("[×] No open ports")
            } else {
                for _, p := range open {
                    fmt.Println("[+] open", p)
                }
            }
        case 3:
            fmt.Print("Puertos (ej:80,443): ")
            pstr, _ := reader.ReadString('\n')
            ports := parsePorts(strings.TrimSpace(pstr))
            for _, p := range ports {
                fmt.Printf("-- %d --\n%s\n", p, bannerGrab(domain, p))
            }
        case 4:
            for _, ip := range resolve(domain, "A") {
                for _, h := range ptrLookup(ip) {
                    fmt.Println("[+] PTR:", h)
                }
            }
        case 5:
            fmt.Println("[dig A+AAAA]", runCmd("dig", "+short", domain))
            fmt.Println("[curl -I]", runCmd("curl", "-sI", domain))
        case 6:
            fmt.Println(runCmd("dig", "AXFR", domain, "@8.8.8.8"))
        case 7:
            fmt.Print("Wordlist (ej: wordlist.txt): ")
            w, _ := reader.ReadString('\n')
            subBrute(domain, strings.TrimSpace(w))
        case 8:
            for _, t := range []string{"A", "AAAA"} {
                for _, ip := range resolve(domain, t) {
                    fmt.Printf("%s: %s\n", t, ip)
                }
            }
        default:
            fmt.Println("Opción no válida")
        }
    }
}
