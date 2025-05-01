package main

import (
  "encoding/json"
  "fmt"
  "html/template"
  "log"
  "net"
  "net/http"
  "os/exec"
  "runtime"
  "sync"
  "time"
)

type RequestEntry struct {
  IP        string
  Timestamp time.Time
}

var (
  requestsLog   []RequestEntry
  logMutex      sync.Mutex
  startTime     time.Time
)

func getServerIP() string {
  conn, err := net.Dial("udp", "10.255.255.255:1")
  if err != nil {
    return "127.0.0.1"
  }
  defer conn.Close()
  localAddr := conn.LocalAddr().(*net.UDPAddr)
  return localAddr.IP.String()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
  logMutex.Lock()
  requestsLog = append(requestsLog, RequestEntry{IP: r.RemoteAddr, Timestamp: time.Now()})
  logMutex.Unlock()
  http.Redirect(w, r, "/monitor", http.StatusFound)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
  logMutex.Lock()
  total := len(requestsLog)
  uptime := int(time.Since(startTime).Seconds())
  logMutex.Unlock()

  greenThreshold := 1000
  yellowThreshold := 10000
  redThreshold := 100000

  if total > 90000 {
    greenThreshold = 10000
    yellowThreshold = 100000
    redThreshold = 1000000
  } else if total < 10000 {
    greenThreshold = 1000
    yellowThreshold = 10000
    redThreshold = 100000
  }

  json.NewEncoder(w).Encode(map[string]interface{}{
    "requests_total":    total,
    "uptime_seconds":    uptime,
    "green":             greenThreshold,
    "yellow":            yellowThreshold,
    "red":               redThreshold,
  })
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
  logMutex.Lock()
  requestsLog = nil
  startTime = time.Now()
  logMutex.Unlock()
  w.Write([]byte("Requests log has been reset."))
}

func monitorHandler(w http.ResponseWriter, r *http.Request) {
  tmpl, err := template.New("monitor").Parse(monitorHTML)
  if err != nil {
    http.Error(w, "Template error", 500)
    return
  }
  tmpl.Execute(w, map[string]string{"ServerIP": getServerIP()})
}

func main() {
  startTime = time.Now()
  http.HandleFunc("/", indexHandler)
  http.HandleFunc("/status", statusHandler)
  http.HandleFunc("/reset", resetHandler)
  http.HandleFunc("/monitor", monitorHandler)

  serverIP := getServerIP()
  if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
    exec.Command("xdg-open", fmt.Sprintf("http://%s:8080/monitor", serverIP)).Start()
  } else if runtime.GOOS == "windows" {
    exec.Command("rundll32", "url.dll,FileProtocolHandler", fmt.Sprintf("http://%s:8080/monitor", serverIP)).Start()
  }

  log.Printf("Server running on: http://%s:8080", serverIP)
  http.ListenAndServe(":8080", nil)
}

const monitorHTML = `
<!DOCTYPE html>
<html>
<head>
  <title>Requests Monitor - Traffic</title>
  <style>
    body {
      background-color: #000;
      color: #fff;
      font-family: 'Segoe UI', sans-serif;
      text-align: center;
      padding: 20px;
    }
    .instructions {
      background: #111;
      padding: 10px;
      border-radius: 10px;
      margin: 20px auto;
      width: fit-content;
      box-shadow: 0 0 10px #00d9ff70;
    }
    .gauge-container {
      display: inline-block;
      margin: 20px;
      padding: 20px;
      background: #111;
      border-radius: 15px;
      width: 240px;
    }
    canvas {
      display: block;
      margin: 0 auto 10px auto;
      border-radius: 50%;
    }
    .uptime {
      margin-top: 20px;
      font-size: 20px;
      color: #ff0;
    }
  </style>
</head>
<body>
  <h1 style="color:#00d9ff;">Requests Monitor (Total)</h1>
  <p>Open in browser: <a href="http://{{.ServerIP}}:8080/monitor">http://{{.ServerIP}}:8080/monitor</a></p>
  <div class="instructions">
    <p>Example test:</p>
    <code>siege -c 250 -r 100 http://{{.ServerIP}}:8080/</code><br>
    <code>while true; do curl -s http://{{.ServerIP}}:8080/ > /dev/null; done</code>
  </div>
  <div class="uptime">Uptime: <span id="uptime"></span> seconds</div>
  <div class="gauge-container">
    <h2 id="greenTitle" style="color:#00ff00;">Green Stage (0 - 1000)</h2>
    <canvas id="greenGauge" width="200" height="200"></canvas>
    <div id="greenInfo"></div>
  </div>
  <div class="gauge-container">
    <h2 id="yellowTitle" style="color:#ffff00;">Yellow Stage (1000 - 10000)</h2>
    <canvas id="yellowGauge" width="200" height="200"></canvas>
    <div id="yellowInfo"></div>
  </div>
  <div class="gauge-container">
    <h2 id="redTitle" style="color:#ff4444;">Red Stage (10000 - 100000)</h2>
    <canvas id="redGauge" width="200" height="200"></canvas>
    <div id="redInfo"></div>
  </div>
  <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
  <script>
    const greenBase = 1000, yellowBase = 10000, redBase = 100000;
    let greenChart, yellowChart, redChart, lastRedThreshold = redBase;

    function createGauge(ctx, color, maxVal) {
      return new Chart(ctx, {
        type: 'doughnut',
        data: {
          datasets: [{ data: [0, maxVal], backgroundColor: [color, '#333'] }]
        },
        options: {
          cutout: '70%',
          plugins: { legend: { display: false }, tooltip: { enabled: false } }
        }
      });
    }

    function updateGauge(chart, value, maxVal) {
      chart.data.datasets[0].data = [value, Math.max(0, maxVal - value)];
      chart.update();
    }

    function fetchStatus() {
      fetch('/status')
        .then(r => r.json())
        .then(data => {
          let count = data.requests_total;
          let green = data.green, yellow = data.yellow, red = data.red;
          document.getElementById('uptime').innerText = data.uptime_seconds;

          document.getElementById('greenTitle').innerText = "Green Stage (0 - " + green + ")";
          document.getElementById('yellowTitle').innerText = "Yellow Stage (" + green + " - " + yellow + ")";
          document.getElementById('redTitle').innerText = "Red Stage (" + yellow + " - " + red + ")";

          updateGauge(greenChart, Math.min(count, green), green);
          updateGauge(yellowChart, Math.max(0, Math.min(count - green, yellow - green)), yellow - green);
          updateGauge(redChart, Math.max(0, Math.min(count - yellow, red - yellow)), red - yellow);

          document.getElementById('greenInfo').innerText = Math.min(count, green) + ' / ' + green;
          document.getElementById('yellowInfo').innerText = Math.max(0, Math.min(count - green, yellow - green)) + ' / ' + (yellow - green);
          document.getElementById('redInfo').innerText = Math.max(0, Math.min(count - yellow, red - yellow)) + ' / ' + (red - yellow);
        });
    }

    window.onload = () => {
      greenChart = createGauge(document.getElementById('greenGauge').getContext('2d'), '#00ff00', greenBase);
      yellowChart = createGauge(document.getElementById('yellowGauge').getContext('2d'), '#ffff00', yellowBase - greenBase);
      redChart = createGauge(document.getElementById('redGauge').getContext('2d'), '#ff0000', redBase - yellowBase);
      setInterval(fetchStatus, 1000);
      fetchStatus();
    };
  </script>
</body>
</html>
`