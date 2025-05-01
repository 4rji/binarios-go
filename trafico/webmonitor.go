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
  cutoff := time.Now().Add(-60 * time.Second)
  logMutex.Lock()
  count := 0
  for _, entry := range requestsLog {
    if entry.Timestamp.After(cutoff) {
      count++
    }
  }
  uptime := int(time.Since(startTime).Seconds())
  logMutex.Unlock()

  greenThreshold := 1000
  yellowThreshold := 10000
  redThreshold := 100000

  if count > 90000 {
    greenThreshold *= 10
    yellowThreshold *= 10
    redThreshold *= 10
  }

  json.NewEncoder(w).Encode(map[string]interface{}{
    "requests_last_60s": count,
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

const monitorHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Requests Monitor - Traffic Light</title>
  <style>
    body {
      background-color: #000;
      color: #fff;
      font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
      text-align: center;
      margin: 0;
      padding: 20px;
    }
    h1 {
      color: #00d9ff;
      text-shadow: 2px 2px #111;
    }
    .instructions {
      background-color: #111;
      padding: 15px;
      border-radius: 10px;
      margin: 20px auto;
      width: fit-content;
      box-shadow: 0 0 10px #00d9ff70;
      color: #ccc;
    }
    .gauge-container {
      display: inline-block;
      vertical-align: top;
      margin: 20px;
      padding: 20px;
      background-color: #111;
      border-radius: 15px;
      box-shadow: 0 0 25px rgba(255, 255, 255, 0.1);
      width: 240px;
    }
    canvas {
      display: block;
      margin: 0 auto 10px auto;
      box-shadow: 0 0 30px rgba(255, 255, 255, 0.5);
      border-radius: 50%;
    }
    button {
      padding: 12px 25px;
      font-size: 16px;
      margin-top: 30px;
      cursor: pointer;
      background: linear-gradient(to right, #ff0044, #6600cc);
      color: #fff;
      border: none;
      border-radius: 8px;
      box-shadow: 0 0 10px #ff99cc;
      transition: all 0.3s;
    }
    button:hover {
      background-color: #9900cc;
    }
    .uptime {
      font-size: 20px;
      margin: 20px auto;
      color: #ffd700;
    }
  </style>
</head>
<body>
  <h1>Requests Monitor (Last 60 Seconds)</h1>
  <p>Acceda directamente en: <a href="http://{{.ServerIP}}:8080/monitor">http://{{.ServerIP}}:8080/monitor</a></p>
  <div class="instructions">
    <p>Para pruebas, ejecute en otra terminal:</p>
    <code>siege -c 255 -r 1000 http://{{.ServerIP}}:8080/</code><br>
    <code>while true; do curl -s http://{{.ServerIP}}:8080/ > /dev/null; done</code>
  </div>
  <div class="uptime">Tiempo activo: <span id="uptime"></span> segundos</div>
  <div class="gauge-container">
    <h2 id="greenTitle" style="color: #00ff00;">Green Stage (0 - 1000)</h2>
    <canvas id="greenGauge" width="200" height="200"></canvas>
    <div id="greenInfo"></div>
  </div>
  <div class="gauge-container">
    <h2 id="yellowTitle" style="color: #ffff00;">Yellow Stage (1000 - 10000)</h2>
    <canvas id="yellowGauge" width="200" height="200"></canvas>
    <div id="yellowInfo"></div>
  </div>
  <div class="gauge-container">
    <h2 id="redTitle" style="color: #ff4444;">Red Stage (10000 - 100000)</h2>
    <canvas id="redGauge" width="200" height="200"></canvas>
    <div id="redInfo"></div>
  </div>
  <br>
  <button onclick="resetRequests()">Reset Log</button>
  <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
  <script>
    const greenBase = 1000;
    const yellowBase = 10000;
    const redBase = 100000;
    let greenChart, yellowChart, redChart;
    let lastRedThreshold = redBase;

    function createGauge(ctx, color, maxVal) {
      return new Chart(ctx, {
        type: 'doughnut',
        data: {
          labels: ['Filled', 'Remaining'],
          datasets: [{ data: [0, maxVal], backgroundColor: [color, '#333'], borderWidth: 0 }]
        },
        options: {
          responsive: false,
          cutout: '70%',
          plugins: { legend: { display: false }, tooltip: { enabled: false } }
        }
      });
    }
    function updateGauge(chart, value, maxVal) {
      if (value > maxVal) value = maxVal;
      chart.data.datasets[0].data = [value, maxVal - value];
      chart.update();
    }
    function showAlert(msg) {
      const alertBox = document.getElementById('traffic-alert');
      alertBox.innerText = msg;
      alertBox.style.opacity = 1;
      setTimeout(() => alertBox.style.opacity = 0, 4000);
    }
    function fetchStatus() {
      fetch('/status')
        .then(response => response.json())
        .then(data => {
          let count = data.requests_last_60s;
          let uptime = data.uptime_seconds;
          let greenThreshold = data.green;
          let yellowThreshold = data.yellow;
          let redThreshold = data.red;
          document.getElementById('uptime').innerText = uptime;

          if (redThreshold > lastRedThreshold) {
            showAlert('Level Up: High Traffic Mode Activated');
            lastRedThreshold = redThreshold;

          document.getElementById('greenTitle').innerText = "Green Stage (0 - " + greenThreshold + ")";
          document.getElementById('yellowTitle').innerText = "Yellow Stage (" + greenThreshold + " - " + yellowThreshold + ")";
          document.getElementById('redTitle').innerText = "Red Stage (" + yellowThreshold + " - " + redThreshold + ")";

          }

          let greenValue = Math.min(count, greenThreshold);
          let yellowValue = count > greenThreshold ? Math.min(count - greenThreshold, yellowThreshold - greenThreshold) : 0;
          let redValue = count > yellowThreshold ? Math.min(count - yellowThreshold, redThreshold - yellowThreshold) : 0;

          updateGauge(greenChart, greenValue, greenThreshold);
          updateGauge(yellowChart, yellowValue, yellowThreshold - greenThreshold);
          updateGauge(redChart, redValue, redThreshold - yellowThreshold);

          document.getElementById('greenTitle').innerText = "Green Stage (0 - " + greenThreshold + ")";
          document.getElementById('yellowTitle').innerText = "Yellow Stage (" + greenThreshold + " - " + yellowThreshold + ")";
          document.getElementById('redTitle').innerText = "Red Stage (" + yellowThreshold + " - " + redThreshold + ")";

        })
        .catch(err => console.error(err));
    }
    function resetRequests() {
      fetch('/reset')
        .then(() => {
          lastRedThreshold = redBase;
          document.getElementById('greenTitle').innerText = "Green Stage (0 - " + greenThreshold + ")";
          document.getElementById('yellowTitle').innerText = "Yellow Stage (" + greenThreshold + " - " + yellowThreshold + ")";
          document.getElementById('redTitle').innerText = "Red Stage (" + yellowThreshold + " - " + redThreshold + ")";

          fetchStatus();
        })
        .catch(err => console.error(err));
    }
    window.onload = () => {
      let greenCtx = document.getElementById('greenGauge').getContext('2d');
      let yellowCtx = document.getElementById('yellowGauge').getContext('2d');
      let redCtx = document.getElementById('redGauge').getContext('2d');
      greenChart = createGauge(greenCtx, '#00FF00', greenBase);
      yellowChart = createGauge(yellowCtx, '#FFFF00', yellowBase - greenBase);
      redChart = createGauge(redCtx, '#FF0000', redBase - yellowBase);
      setInterval(fetchStatus, 1000);
      fetchStatus();
    };
  </script>

  <div id="traffic-alert" style="position: fixed; top: 20px; left: 50%; transform: translateX(-50%); background: #ff0044; color: #fff; padding: 10px 20px; border-radius: 8px; box-shadow: 0 0 10px #000; font-weight: bold; opacity: 0; transition: opacity 0.5s ease;"></div>
</body>
</html>`
