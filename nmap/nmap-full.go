package main

import (
    "bufio"
    "bytes"
    "fmt"
    "html"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "strings"
    "time"

   "github.com/rivo/tview"
   "github.com/gdamore/tcell/v2"
)

func run(cmd string, args ...string) (string, error) {
    c := exec.Command(cmd, args...)
    var out bytes.Buffer
    c.Stdout = &out
    c.Stderr = &out
    err := c.Run()
    return out.String(), err
}

func buildListHTML(data []byte) string {
    var b strings.Builder
    for _, line := range strings.Split(string(data), "\n") {
        if t := strings.TrimSpace(line); t != "" {
            b.WriteString(fmt.Sprintf("<li>%s</li>", html.EscapeString(t)))
        }
    }
    return b.String()
}

// Stream command output to a tview.TextView in real time
func streamCmdToTextView(tv *tview.TextView, cmd string, args ...string) error {
    c := exec.Command(cmd, args...)
    stdout, err := c.StdoutPipe()
    if err != nil {
        return err
    }
    stderr, err := c.StderrPipe()
    if err != nil {
        return err
    }
    if err := c.Start(); err != nil {
        return err
    }
    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            line := scanner.Text()
            tv.Write([]byte(line + "\n"))
        }
    }()
    go func() {
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            line := scanner.Text()
            tv.Write([]byte(line + "\n"))
        }
    }()
    return c.Wait()
}

func main() {
    if os.Geteuid() != 0 {
        log.Fatal("[!] Run as root.")
    }
    if len(os.Args) < 2 {
        fmt.Println("Usage: ./network_scan_report <CIDR>")
        os.Exit(1)
    }

    target := os.Args[1]
    ts := time.Now().Format("20060102_150405")
    outDir := "recon_" + ts
    os.MkdirAll(outDir, 0755)
    htmlPath := outDir + "/report.html"

    app := tview.NewApplication()

    // Global color scheme
    tview.Styles.PrimitiveBackgroundColor = tcell.ColorDarkBlue
    tview.Styles.ContrastBackgroundColor = tcell.ColorDarkBlue
    tview.Styles.MoreContrastBackgroundColor = tcell.ColorDarkBlue
    tview.Styles.BorderColor = tcell.ColorGreen
    tview.Styles.TitleColor = tcell.ColorGreen
    tview.Styles.GraphicsColor = tcell.ColorLightCyan
    tview.Styles.PrimaryTextColor = tcell.ColorWhite
    tview.Styles.SecondaryTextColor = tcell.ColorLightGrey

    procPane := tview.NewTextView()
    procPane.SetTitle("nmap processes")
    procPane.SetBorder(true)
    procPane.SetBackgroundColor(tcell.ColorDarkBlue)

    tailPane := tview.NewTextView()
    tailPane.SetTitle("ports.nmap (tail)")
    tailPane.SetBorder(true)
    tailPane.SetBackgroundColor(tcell.ColorDarkBlue)

    logPane := tview.NewTextView()
    logPane.SetTitle("scan log")
    logPane.SetBorder(true)
    logPane.SetBackgroundColor(tcell.ColorDarkBlue)
    logPane.SetChangedFunc(func() {
        logPane.ScrollToEnd()
        app.Draw()
    })

    leftFlex := tview.NewFlex().SetDirection(tview.FlexRow).
        AddItem(procPane, 0, 1, false).
        AddItem(tailPane, 0, 1, false)
    leftFlex.SetBackgroundColor(tcell.ColorDarkBlue)

    flex := tview.NewFlex().
        AddItem(leftFlex, 0, 1, false).
        AddItem(logPane, 0, 2, true)
    flex.SetBackgroundColor(tcell.ColorDarkBlue)

    // Monitor de procesos nmap
    go func() {
        for {
            out, _ := run("bash", "-c", "ps aux | grep [n]map")
            app.QueueUpdateDraw(func() { procPane.SetText(out) })
            time.Sleep(time.Second)
        }
    }()

    // Monitor tail del archivo ports.nmap
    go func() {
        var lastContent string
        for {
            // Buscar el archivo ports.nmap más reciente en el directorio recon_*
            files, _ := ioutil.ReadDir(".")
            var newest string
            var newestTime time.Time
            for _, f := range files {
                if f.IsDir() && strings.HasPrefix(f.Name(), "recon_") {
                    p := f.Name() + "/ports.nmap"
                    info, err := os.Stat(p)
                    if err == nil && info.ModTime().After(newestTime) {
                        newest = p
                        newestTime = info.ModTime()
                    }
                }
            }
            if newest != "" {
                data, err := ioutil.ReadFile(newest)
                if err == nil {
                    lines := strings.Split(string(data), "\n")
                    if len(lines) > 20 {
                        lines = lines[len(lines)-20:]
                    }
                    content := strings.Join(lines, "\n")
                    if content != lastContent {
                        lastContent = content
                        app.QueueUpdateDraw(func() { tailPane.SetText(content) })
                    }
                }
            } else {
                app.QueueUpdateDraw(func() { tailPane.SetText("") })
            }
            time.Sleep(2 * time.Second)
        }
    }()

    // Menú de selección de modo de escaneo
    scanModes := []string{
        "Fast   – TCP top 1000",
        "Medium – TCP/UDP top 20 + OS",
        "Full   – TCP all + UDP + OS",
    }
    scanList := tview.NewList().
        ShowSecondaryText(false)
    scanList.SetTitle("Select scan mode").SetBorder(true).SetBackgroundColor(tcell.ColorDarkBlue)
    for i, mode := range scanModes {
        idx := i + 1
        scanList.AddItem(fmt.Sprintf("%d) %s", idx, mode), "", rune('1'+i), nil)
    }
    scanList.SetDoneFunc(func() {
        app.SetRoot(flex, true)
    })

    scanList.SetSelectedFunc(func(ix int, mainText, secText string, shortcut rune) {
        app.SetRoot(flex, true)
        go func() {
            logLn := func(s string) { fmt.Fprintf(logPane, "%s\n", s) }

            logLn("[1] Host discovery")
            streamCmdToTextView(logPane, "nmap", "-sn", target, "-oG", outDir+"/pingsweep.gnmap")

            pf, _ := os.Open(outDir + "/pingsweep.gnmap")
            scanner := bufio.NewScanner(pf)
            hf, _ := os.Create(outDir + "/hosts.txt")
            for scanner.Scan() {
                line := scanner.Text()
                if strings.HasSuffix(line, "Up") {
                    if f := strings.Fields(line); len(f) >= 2 {
                        hf.WriteString(f[1] + "\n")
                    }
                }
            }
            hf.Close()
            pf.Close()

            switch ix {
            case 0: // Fast
                logLn("[2] Port scan (fast)")
                streamCmdToTextView(logPane, "nmap", "-sS", "-sV", "-T4", "--top-ports", "1000", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/ports.nmap")
            case 1: // Medium
                logLn("[2] Port scan (medium)")
                streamCmdToTextView(logPane, "nmap", "-sS", "-sU", "-T4", "--top-ports", "20", "-O", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/ports.nmap")
            case 2: // Full
                logLn("[2] Port scan (full)")
                streamCmdToTextView(logPane, "nmap", "-sS", "-sU", "-T4", "-p-", "-O", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/ports.nmap")
            }

            logLn("[3] SMB enumeration")
            streamCmdToTextView(logPane, "nmap", "--script", "smb-enum-shares,smb-os-discovery", "-p", "445", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/smb.nmap")

            logLn("[4] SNMP info")
            streamCmdToTextView(logPane, "nmap", "-sU", "-p", "161", "--script", "snmp-info", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/snmp.nmap")

            logLn("[5] Vuln scan")
            streamCmdToTextView(logPane, "nmap", "--script", "vuln", "-iL", outDir+"/hosts.txt", "-oN", outDir+"/vuln.nmap")

            logLn("[+] Generating HTML…")

            hosts, _ := ioutil.ReadFile(outDir + "/hosts.txt")
            ports, _ := ioutil.ReadFile(outDir + "/ports.nmap")
            smb, _ := ioutil.ReadFile(outDir + "/smb.nmap")
            snmp, _ := ioutil.ReadFile(outDir + "/snmp.nmap")
            vuln, _ := ioutil.ReadFile(outDir + "/vuln.nmap")

            escPorts := html.EscapeString(string(ports))
            escPorts = strings.ReplaceAll(escPorts, " open ", `<span style=\"color:#28a745;font-weight:bold;\">open</span>`)
            escPorts = strings.ReplaceAll(escPorts, " closed ", `<span style=\"color:#dc3545;font-weight:bold;\">closed</span>`)
            escPorts = strings.ReplaceAll(escPorts, " filtered ", `<span style=\"color:#ffc107;font-weight:bold;\">filtered</span>`)

            htmlContent := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset=\"UTF-8\">
<title>Report</title><style>pre{background:#1e1e1e;color:#e8e8e8;padding:1em;border-radius:8px;font-family:monospace;}</style></head><body>
<h1>Network Scan Report</h1><small>%s</small>
<h2>Live Hosts</h2><ul>%s</ul>
<h2>Port Scan</h2><pre>%s</pre>
<h2>SMB</h2><pre>%s</pre>
<h2>SNMP</h2><pre>%s</pre>
<h2>Vuln</h2><pre>%s</pre></body></html>`,
                time.Now().Format("2006-01-02 15:04:05"),
                buildListHTML(hosts), escPorts,
                html.EscapeString(string(smb)),
                html.EscapeString(string(snmp)),
                html.EscapeString(string(vuln)))

            ioutil.WriteFile(htmlPath, []byte(htmlContent), 0644)
            logLn("[✓] Report written to " + htmlPath)
        }()
    })

    if err := app.SetRoot(scanList, true).Run(); err != nil {
        panic(err)
    }
}
