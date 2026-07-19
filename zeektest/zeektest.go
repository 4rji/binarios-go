// m2.go
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Hit struct{ Source map[string]any `json:"_source"` }
type SearchResp struct {
	Hits struct{ Hits []Hit `json:"hits"` } `json:"hits"`
}

func getDot(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, p := range strings.Split(path, ".") {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := next[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}
func pick(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := getDot(m, k); ok && v != nil {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				if t == float64(int64(t)) {
					return fmt.Sprintf("%d", int64(t))
				}
				return fmt.Sprintf("%g", t)
			case bool:
				if t {
					return "true"
				}
				return "false"
			default:
				b, _ := json.Marshal(t)
				return string(b)
			}
		}
	}
	return ""
}

func parseZeekDNS(line string) (src, dst, query, qtype, rcode string) {
	// Zeek dns.log TSV:
	// 0 ts 1 uid 2 id.orig_h 3 id.orig_p 4 id.resp_h 5 id.resp_p
	// 6 proto 7 trans_id 8 rtt 9 query 10 qclass 11 qclass_name
	// 12 qtype 13 qtype_name 14 rcode 15 rcode_name ...
	f := strings.Split(line, "\t")
	if len(f) < 16 {
		return
	}
	src = f[2]
	dst = f[4]
	query = f[9]
	qtype = f[13] // legible (A, AAAA, etc.)
	rcode = f[15] // legible (NOERROR, NXDOMAIN, etc.)
	return
}

func main() {
	esURL := flag.String("url", "https://localhost:9200", "ES URL")
	user := flag.String("user", "elastic", "user")
	pass := flag.String("pass", "rancid12", "pass")
	index := flag.String("index", "filebeat-*", "index")
	size := flag.Int("size", 10, "size")
	caPath := flag.String("cacert", "", "CA cert path")
	insecure := flag.Bool("insecure", false, "skip TLS verify")
	flag.Parse()

	q := map[string]any{
		"size": *size,
		"sort": []map[string]string{{"@timestamp": "desc"}},
		"query": map[string]any{
			"term": map[string]string{"event.dataset": "zeek.dns"},
		},
	}
	body, _ := json.Marshal(q)

	tlsCfg := &tls.Config{InsecureSkipVerify: *insecure || *caPath == ""}
	if *caPath != "" && !*insecure {
		pem, err := os.ReadFile(*caPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read cacert:", err)
			os.Exit(1)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(pem)
		tlsCfg.RootCAs = pool
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s/_search", *esURL, *index), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(*user, *pass)

	res, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request:", err)
		os.Exit(1)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		fmt.Fprintf(os.Stderr, "Error: %d %s\n", res.StatusCode, string(b))
		os.Exit(1)
	}

	var sr SearchResp
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		fmt.Fprintln(os.Stderr, "decode:", err)
		os.Exit(1)
	}
	if len(sr.Hits.Hits) == 0 {
		fmt.Println("No hay resultados")
		return
	}

	fmt.Println("| timestamp | src_ip | dst_ip | query | qtype | rcode |")
	fmt.Println("|---|---|---|---|---|---|")
	for _, h := range sr.Hits.Hits {
		s := h.Source
		ts := pick(s, "@timestamp", "event.ingested")

		src := pick(s, "source.ip", "client.ip", "id.orig_h", "zeek.orig_h")
		dst := pick(s, "destination.ip", "server.ip", "id.resp_h", "zeek.resp_h")
		qry := pick(s, "zeek.dns.query", "dns.question.name", "dns.query", "query")
		qt := pick(s, "zeek.dns.qtype_name", "zeek.dns.qtype", "dns.question.type", "qtype")
		rc := pick(s, "zeek.dns.rcode_name", "zeek.dns.rcode", "dns.response_code", "rcode")

		// Fallback: parse TSV desde message/event.original si sigue vacío
		if (src == "" || dst == "" || qry == "" || qt == "" || rc == "") {
			raw := pick(s, "event.original", "message")
			if strings.Contains(raw, "\t") {
				ps, pd, pq, pqt, prc := parseZeekDNS(raw)
				if src == "" { src = ps }
				if dst == "" { dst = pd }
				if qry == "" { qry = pq }
				if qt == ""  { qt  = pqt }
				if rc == ""  { rc  = prc }
			}
		}

		fmt.Printf("| %s | %s | %s | %s | %s | %s |\n", ts, src, dst, qry, qt, rc)
	}
}
