package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// parseReadme parses the README file to extract script information
func parseReadme(filename string) ([]Script, error) {
	fmt.Printf("%sLoading /opt/4rji/bin/README.md...%s\n", ColorCyan, ColorReset)
	file, err := os.Open("/opt/4rji/bin/README.md")
	if err != nil {
		return nil, fmt.Errorf("error opening README.md: %v", err)
	}
	defer file.Close()

	var scripts []Script
	scanner := bufio.NewScanner(file)
	reCategory := regexp.MustCompile(`^#+\s*.*`)
	seenScripts := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		if !reCategory.MatchString(line) && strings.TrimSpace(line) != "" {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				scriptFile := parts[0]
				if seenScripts[scriptFile] {
					continue
				}
				seenScripts[scriptFile] = true

				desc := ""
				if len(parts) > 1 {
					desc = strings.Join(parts[1:], " ")
				}
				scripts = append(scripts, Script{Name: scriptFile, Desc: desc})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning README.md: %v", err)
	}

	fmt.Printf("%sLoaded %d scripts from README%s\n", ColorGreen, len(scripts), ColorReset)
	return scripts, nil
}

// getCombinedScripts combines scripts from README and bin directory
func getCombinedScripts(readmePath, binDir string) ([]Script, error) {
	readmeScripts, err := parseReadme(readmePath)
	if err != nil {
		return nil, err
	}
	readmeMap := make(map[string]bool)
	for _, s := range readmeScripts {
		readmeMap[s.Name] = true
	}
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return readmeScripts, nil
	}
	var extraScripts []Script
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if readmeMap[name] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mode := info.Mode()
		if mode&0111 == 0 {
			continue
		}
		extraScripts = append(extraScripts, Script{
			Name: name,
			Desc: "Enter to see description",
		})
	}
	allScripts := append(readmeScripts, extraScripts...)
	return allScripts, nil
}

// searchInFiles searches for a pattern in all files within the bin directory
func searchInFiles(pattern string) ([]Script, error) {
	binDir := "/opt/4rji/bin"
	excludedFiles := []string{
		"comprimidos",
		"linenum.sh.enc",
		"linpeas.sh",
		"impacto.zip",
		"winPEASx64.exe",
		"winPEASx86.exe",
		"tk.enc",
		"README.md",
		"pspy64",
		"chisel4",
		"SharpHound.ps1",
		"yazi",
		"agent.zip",
		"meg",
		"LinEnum.sh",
		"chisel",
		"assetfinder",
		"kerbrute_linux_amd64",
		"proxyserver",
		"chise2",
		"airsendm",
		"amigo",
		"amigom",
		"backd",
		"backdm",
		"copyrsm",
		"dominf",
		"ftpbrute",
		"locip",
		"locipm",
		"miniserver1",
		"miniserverw",
		"nets",
		"netsm",
		"nmap-fullm",
		"nmap-halfm",
		"nmapX",
		"nmapXm",
		"nv-agent",
		"pingg",
		"pingm",
		"siegee",
		"trafico",
		"traficom",
	}
	args := []string{"-r", pattern, binDir}
	for _, file := range excludedFiles {
		args = append(args, "--exclude="+file)
	}
	args = append(args, "--exclude-dir=comprimidos")

	cmd := exec.Command("grep", args...)
	output, err := cmd.CombinedOutput()
	// grep returns 1 if no matches are found, that's not an error for us
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []Script{}, nil
		}
		return nil, fmt.Errorf("error executing grep: %v", err)
	}

	var foundScripts []Script
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		filePath := parts[0]
		content := strings.TrimSpace(parts[1])
		fileName := filepath.Base(filePath)
		foundScripts = append(foundScripts, Script{
			Name: fileName,
			Desc: content,
		})
	}
	return foundScripts, nil
}
