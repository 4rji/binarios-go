package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func containsWord(text, pattern string) bool {
	regexPattern := `\b` + regexp.QuoteMeta(pattern) + `\b`
	matched, err := regexp.MatchString("(?i)"+regexPattern, text)
	return err == nil && matched
}

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(text string) string {
	return ansiEscapeRE.ReplaceAllString(text, "")
}

type excludedScripts map[string]struct{}

func loadExcludedScripts(binDir string) (excludedScripts, error) {
	excluded := make(excludedScripts)
	path := filepath.Join(binDir, ".todoignore")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return excluded, nil
		}
		return nil, fmt.Errorf("error opening .todoignore: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if comment := strings.Index(line, "#"); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		if line != "" {
			excluded[line] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning .todoignore: %v", err)
	}

	return excluded, nil
}

func (excluded excludedScripts) has(name string) bool {
	_, ok := excluded[name]
	return ok
}

func (excluded excludedScripts) filter(scripts []Script) []Script {
	if len(excluded) == 0 {
		return scripts
	}
	filtered := scripts[:0]
	for _, script := range scripts {
		if excluded.has(script.Name) {
			continue
		}
		filtered = append(filtered, script)
	}
	return filtered
}

func parseReadme(filename string) ([]Script, error) {
	file, err := os.Open(filename)
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

	return scripts, nil
}

func getCombinedScripts(readmePath, binDir string) ([]Script, error) {
	readmeScripts, err := parseReadme(readmePath)
	if err != nil {
		return nil, err
	}
	excluded, err := loadExcludedScripts(binDir)
	if err != nil {
		return nil, err
	}
	readmeScripts = excluded.filter(readmeScripts)
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
		if excluded.has(name) {
			continue
		}
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

func searchInFiles(pattern string) ([]Script, error) {
	var foundScripts []Script
	scriptMap := make(map[string]Script)

	binDir := "/opt/4rji/bin"
	excluded, err := loadExcludedScripts(binDir)
	if err != nil {
		return nil, err
	}
	excludedFiles := []string{
		"comprimidos",
		"linenum.sh.enc",
		"linpeas.sh",
		"impacto.zip",
		"winPEASx64.exe",
		"winPEASx86.exe",
		"tk.enc",
		"README.md",
		".todoignore",
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

	searchPattern := fmt.Sprintf("\\b%s\\b", regexp.QuoteMeta(pattern))
	args := []string{"-r", "-i", "-E", searchPattern, binDir}
	for _, file := range excludedFiles {
		args = append(args, "--exclude="+file)
	}
	for file := range excluded {
		args = append(args, "--exclude="+file)
	}
	args = append(args, "--exclude-dir=comprimidos")

	cmd := exec.Command("grep", args...)
	output, err := cmd.CombinedOutput()

	if err == nil {
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
			if excluded.has(fileName) {
				continue
			}
			scriptMap[fileName] = Script{
				Name: fileName,
				Desc: content,
			}
		}
	}

	readmeScripts, err := parseReadme("/opt/4rji/bin/README.md")
	if err == nil {
		for _, script := range readmeScripts {
			if excluded.has(script.Name) {
				continue
			}
			if containsWord(script.Name, pattern) || containsWord(script.Desc, pattern) {
				if existing, exists := scriptMap[script.Name]; exists {
					if script.Desc != "" && existing.Desc == "" {
						scriptMap[script.Name] = script
					}
				} else {
					scriptMap[script.Name] = script
				}
			}
		}
	}

	descriptions, err := loadDescriptions()
	if err == nil {
		for scriptName, desc := range descriptions {
			if excluded.has(scriptName) || excluded.has(desc.Name) {
				continue
			}
			if containsWord(scriptName, pattern) ||
				containsWord(desc.ShortDesc, pattern) ||
				containsWord(desc.DetailedDesc, pattern) {
				if existing, exists := scriptMap[scriptName]; exists {
					if desc.ShortDesc != "" && existing.Desc == "" {
						scriptMap[scriptName] = Script{
							Name: scriptName,
							Desc: desc.ShortDesc,
						}
					}
				} else {
					scriptMap[scriptName] = Script{
						Name: scriptName,
						Desc: desc.ShortDesc,
					}
				}
			}
		}
	}

	for _, script := range scriptMap {
		foundScripts = append(foundScripts, script)
	}

	return foundScripts, nil
}

func filterScripts(scripts []Script, query string) []Script {
	if query == "" {
		return scripts
	}
	type scored struct {
		s     Script
		score int
	}
	var results []scored
	queryLower := strings.ToLower(query)
	for _, s := range scripts {
		score := 0
		nameLower := strings.ToLower(s.Name)
		if strings.EqualFold(s.Name, query) {
			score += 170
		} else if containsWord(s.Name, query) {
			score += 120
		} else if strings.Contains(nameLower, queryLower) {
			score += 80
		}
		if containsWord(s.Desc, query) {
			score += 60
		} else if strings.Contains(strings.ToLower(s.Desc), queryLower) {
			score += 25
		}
		if score > 0 {
			results = append(results, scored{s: s, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return strings.ToLower(results[i].s.Name) < strings.ToLower(results[j].s.Name)
		}
		return results[i].score > results[j].score
	})
	out := make([]Script, len(results))
	for i, r := range results {
		out[i] = r.s
	}
	return out
}
