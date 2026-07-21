import { useEffect, useMemo, useRef, useState } from "react";
import RFB from "@novnc/novnc";
import {
  Activity,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Columns2,
  Download,
  Dumbbell,
  FileText,
  KeyRound,
  LogIn,
  LogOut,
  Monitor,
  Moon,
  Network,
  Play,
  Power,
  RotateCcw,
  Server,
  Shield,
  Sun,
  Terminal,
  Trash2,
  RectangleHorizontal,
  X
} from "lucide-react";

const labTopologyImage = new URL("../../topology.png", import.meta.url).href;
const credentialsImage = new URL("../../credentials.png", import.meta.url).href;
const teamPackPdf = new URL("../../2025MWCCDCITeamPack_.pdf", import.meta.url).href;
const regionalTeamPackPdf = new URL("../../2026MWCCDCRegionalTeamPack.pdf", import.meta.url).href;
const tuxLogoImage = "https://upload.wikimedia.org/wikipedia/commons/3/35/Tux.svg";

const api = {
  baseUrl() {
    return window.labPortal?.apiBaseUrl || import.meta.env.VITE_API_BASE_URL || "";
  },
  websocketUrl(path) {
    const url = new URL(path, this.baseUrl() || window.location.origin);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  },
  async request(path, options = {}, token = "") {
    const authHeaders = token ? { Authorization: `Bearer ${token}` } : {};
    const response = await fetch(`${this.baseUrl()}${path}`, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...authHeaders,
        ...options.headers
      }
    });

    if (!response.ok) {
      const payload = await response.json().catch(() => ({}));
      const details = payload.details
        ? ` ${typeof payload.details === "string" ? payload.details : JSON.stringify(payload.details)}`
        : "";
      throw new Error(`${payload.error || `Request failed: ${response.status}`}${details}`);
    }

    if (response.status === 204) return null;
    return response.json();
  }
};

const statusLabels = {
  queued: "Queued",
  creating: "Creating",
  ready: "Ready",
  failed: "Failed",
  deleting: "Deleting",
  deleted: "Deleted",
  stopped: "Stopped",
  resetting: "Resetting"
};

const startingStatuses = new Set(["queued", "creating", "resetting"]);

const catalogMachineCategories = new Set(["linux", "windows"]);
const errorAutoDismissMs = 10000;

function useAutoDismissMessage(message, setMessage, delayMs = errorAutoDismissMs) {
  useEffect(() => {
    if (!message) return undefined;

    const timer = window.setTimeout(() => setMessage(""), delayMs);
    return () => window.clearTimeout(timer);
  }, [message, setMessage, delayMs]);
}

function osThemeClass(category = "linux") {
  return `os-${category}`;
}

const viewLabels = {
  catalog: { eyebrow: "catalog", title: "CCDC catalog" },
  topology: { eyebrow: "topology", title: "Lab topology" },
  resources: { eyebrow: "resources", title: "Books" },
  active: { eyebrow: "active", title: "Deployments" },
  guide: { eyebrow: "guide", title: "Hardening guide" },
  training1: { eyebrow: "training", title: "Training 1" },
  admin: { eyebrow: "admin", title: "Operations" }
};

const trainingOneSections = [
  {
    title: "Users And Privileges",
    items: [
      "Local accounts that should not exist.",
      "Accounts with interactive shells.",
      "Users added to administrative groups.",
      "Weak or forced passwords.",
      "Service accounts used as real users."
    ]
  },
  {
    title: "SSH",
    items: [
      "`authorized_keys` files in sensitive accounts.",
      "Permissions and special attributes on SSH files.",
      "Unauthorized public keys.",
      "SSH configuration that allows overly permissive access."
    ]
  },
  {
    title: "Scheduled Tasks",
    items: [
      "Root crontab.",
      "Files under system cron paths.",
      "Tasks that download files from the network.",
      "Recurring tasks that restore malicious changes."
    ]
  },
  {
    title: "Web And PHP",
    items: [
      "Unexpected content under `/var/www`.",
      "Recently modified PHP files.",
      "Use of dangerous PHP functions.",
      "Files that allow system command execution.",
      "Duplicate copies of the same suspicious page."
    ]
  },
  {
    title: "Services",
    items: [
      "Enabled services that do not match the system role.",
      "Services started recently.",
      "Insecure file transfer services.",
      "Active web services with unknown content."
    ]
  },
  {
    title: "Packages",
    items: [
      "Clients or servers for insecure protocols.",
      "Recently installed packages.",
      "Packages that do not make sense for the expected machine function.",
      "Differences between base packages and packages added during the incident."
    ]
  },
  {
    title: "Permissions And Attributes",
    items: [
      "Files marked immutable.",
      "Sensitive files with overly open permissions.",
      "Directories where unprivileged users can write executable content.",
      "System files modified without justification."
    ]
  },
  {
    title: "Network",
    items: [
      "Open ports.",
      "Processes listening on external interfaces.",
      "Repeated outbound connections.",
      "Services accepting connections from unexpected networks."
    ]
  },
  {
    title: "Persistence Evidence",
    items: [
      "Mechanisms that restore files after deletion.",
      "Changes that survive reboots.",
      "Automatic downloads from internal or external hosts.",
      "Hidden files or nonstandard locations."
    ]
  }
];

const hiddenActiveOutputKeys = new Set([
  "vm_name",
  "proxmox_vmid",
  "proxmox_node",
  "platform",
  "status",
  "novnc_url"
]);

function fmtDate(value) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    day: "numeric"
  }).format(new Date(value));
}

function fmtTimeLeft(expiresAt, now = Date.now()) {
  const expiresAtMs = new Date(expiresAt).getTime();
  if (!Number.isFinite(expiresAtMs)) return "Unavailable";

  const totalSeconds = Math.max(0, Math.ceil((expiresAtMs - now) / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (totalSeconds <= 0) return "Expired";
  if (hours > 0) return `${hours}h ${String(minutes).padStart(2, "0")}m ${String(seconds).padStart(2, "0")}s`;
  if (minutes > 0) return `${minutes}m ${String(seconds).padStart(2, "0")}s`;
  return `${seconds}s`;
}

function deploymentResourceName(deployment) {
  return deployment?.resourceName || deployment?.proxmoxVmName || "Pending";
}

function fmtValue(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

function fmtResourceValue(value, unit) {
  if (value === null || value === undefined) return "Unavailable";
  if (unit === "MB") {
    const gb = value / 1024;
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`;
  }
  return `${value} ${unit}`;
}

function fmtBytes(value) {
  if (!Number.isFinite(value) || value <= 0) return "Unavailable";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size >= 10 ? size.toFixed(0) : size.toFixed(1)} ${units[unitIndex]}`;
}

function resourcePercent(metric) {
  if (!metric || metric.used === null || metric.total === null || metric.total <= 0) return null;
  return Math.min(100, Math.max(0, Math.round((metric.used / metric.total) * 100)));
}

function fmtElapsed(startedAt, now = Date.now()) {
  const started = new Date(startedAt).getTime();
  if (!Number.isFinite(started)) return "0s";

  const totalSeconds = Math.max(0, Math.floor((now - started) / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes === 0) return `${seconds}s`;
  return `${minutes}m ${String(seconds).padStart(2, "0")}s`;
}

function isStartingDeployment(deployment) {
  return startingStatuses.has(deployment?.status);
}

function consoleUrl(deployment) {
  const consoleOutput = deployment?.outputs?.find((output) => (
    ["console_url", "novnc_console_url", "novnc_url", "vnc_url"].includes(output.key)
  ));
  const url = consoleOutput?.value || deployment?.consoleUrl || "";
  if (!url) return "";

  try {
    const parsedUrl = new URL(url, window.location.origin);
    parsedUrl.searchParams.set("resize", "scale");
    return parsedUrl.toString();
  } catch {
    return url;
  }
}

function guideStepKey(deploymentId, tabId, stepIndex) {
  return `${deploymentId}:${tabId}:${stepIndex}`;
}

export default function App() {
  const [user, setUser] = useState(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [authToken, setAuthToken] = useState("");
  const [labs, setLabs] = useState([]);
  const [deployments, setDeployments] = useState([]);
  const [downloadResources, setDownloadResources] = useState([]);
  const [selectedLabId, setSelectedLabId] = useState(null);
  const [selectedDeploymentId, setSelectedDeploymentId] = useState(null);
  const [view, setView] = useState("catalog");
  const [error, setError] = useState("");
  const [busyLabId, setBusyLabId] = useState("");
  const [busyResourceId, setBusyResourceId] = useState("");
  const [guideTabId, setGuideTabId] = useState("");
  const [completedGuideSteps, setCompletedGuideSteps] = useState({});
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [resourceSummary, setResourceSummary] = useState(null);
  const [consoleMode, setConsoleMode] = useState("normal");
  const [normalConsolePercent, setNormalConsolePercent] = useState(() => (
    Number(window.localStorage.getItem("labPortalNormalConsolePercent")) || 64
  ));
  const [theaterConsolePercent, setTheaterConsolePercent] = useState(() => (
    Number(window.localStorage.getItem("labPortalTheaterConsolePercent")) || 82
  ));
  const [darkMode, setDarkMode] = useState(() => (
    window.localStorage.getItem("labPortalDarkMode") !== "light"
  ));

  useAutoDismissMessage(error, setError);

  const catalogLabs = useMemo(
    () => labs.filter((lab) => catalogMachineCategories.has(lab.category || "linux")),
    [labs]
  );
  const catalogLabSections = useMemo(() => ([
    {
      key: "linux",
      label: "Linux",
      labs: catalogLabs.filter((lab) => (lab.category || "linux") === "linux")
    },
    {
      key: "windows",
      label: "Windows",
      labs: catalogLabs.filter((lab) => lab.category === "windows")
    }
  ]).filter((section) => section.labs.length > 0), [catalogLabs]);
  const selectedLab = useMemo(
    () => catalogLabs.find((lab) => lab.id === selectedLabId) || null,
    [catalogLabs, selectedLabId]
  );
  const activeDeployments = deployments.filter((deployment) => deployment.status !== "deleted");
  const selectedDeployment = activeDeployments.find((deployment) => deployment.id === selectedDeploymentId)
    || activeDeployments[0];
  const guideTabs = selectedDeployment?.lab?.hardeningGuide || [];
  const selectedGuideTab = guideTabs.find((tab) => tab.id === guideTabId) || guideTabs[0];
  const viewLabel = viewLabels[view] || viewLabels.active;
  const theaterActive = view === "guide" && consoleMode === "theater";

  async function loadData(currentToken = authToken) {
    if (!currentToken) return;
    const [labsPayload, deploymentsPayload, resourcesPayload, downloadsPayload] = await Promise.all([
      api.request("/api/labs", {}, currentToken),
      api.request("/api/deployments", {}, currentToken),
      api.request("/api/resources", {}, currentToken).catch((err) => ({
        status: "unavailable",
        source: "unavailable",
        resources: [],
        warnings: [err.message]
      })),
      api.request("/api/downloads", {}, currentToken).catch(() => ({ resources: [] }))
    ]);
    setLabs(labsPayload.labs);
    setDeployments(deploymentsPayload.deployments);
    setResourceSummary(resourcesPayload);
    setDownloadResources(downloadsPayload.resources);
    setSelectedLabId((current) => {
      if (labsPayload.labs.some((lab) => lab.id === current && catalogMachineCategories.has(lab.category || "linux"))) {
        return current;
      }
      return null;
    });
  }

  function selectDeployment(deployment) {
    setSelectedDeploymentId(deployment.id);
    setGuideTabId(deployment.lab?.hardeningGuide?.[0]?.id || "");
  }

  function openDeploymentGuide(deployment = selectedDeployment) {
    if (!deployment) {
      setView("active");
      return;
    }

    setSelectedDeploymentId(deployment.id);
    setGuideTabId((current) => {
      const tabs = deployment.lab?.hardeningGuide || [];
      if (tabs.some((tab) => tab.id === current)) return current;
      return tabs[0]?.id || "";
    });
    setView("guide");
  }

  function toggleGuideStep(deploymentId, tabId, stepIndex) {
    const key = guideStepKey(deploymentId, tabId, stepIndex);
    setCompletedGuideSteps((current) => ({
      ...current,
      [key]: !current[key]
    }));
  }

  async function login(event) {
    event.preventDefault();
    setError("");
    try {
      const payload = await api.request("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password })
      });
      setUser(payload.user);
      setAuthToken(payload.token);
      await loadData(payload.token);
    } catch (err) {
      setError(err.message);
    }
  }

  async function logout() {
    const token = authToken;
    setError("");
    setUser(null);
    setAuthToken("");
    setView("catalog");

    if (!token) return;
    try {
      await api.request("/api/auth/logout", { method: "POST" }, token);
    } catch {
      // The local session is already cleared; server cleanup can fail if the token expired.
    }
  }

  async function deploy(labId) {
    setBusyLabId(labId);
    setError("");
    setSelectedLabId(null);
    try {
      const payload = await api.request("/api/deployments", {
        method: "POST",
        body: JSON.stringify({ labId })
      }, authToken);
      setDeployments((current) => [payload.deployment, ...current]);
      setSelectedDeploymentId(payload.deployment.id);
      setGuideTabId(payload.deployment.lab?.hardeningGuide?.[0]?.id || "");
      setView("guide");
    } catch (err) {
      setError(err.message);
    } finally {
      setBusyLabId("");
    }
  }

  async function downloadResource(resource) {
    setBusyResourceId(resource.id);
    setError("");
    try {
      const response = await fetch(`${api.baseUrl()}/api/downloads/${resource.id}`, {
        headers: {
          Authorization: `Bearer ${authToken}`
        }
      });

      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || `Download failed: ${response.status}`);
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = resource.fileName;
      document.body.append(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusyResourceId("");
    }
  }

  async function action(path, method = "POST") {
    setError("");
    try {
      await api.request(path, { method }, authToken);
      await loadData(authToken);
    } catch (err) {
      setError(err.message);
    }
  }

  useEffect(() => {
    if (!user) return undefined;
    const timer = setInterval(() => {
      loadData(authToken).catch((err) => setError(err.message));
    }, 2500);
    return () => clearInterval(timer);
  }, [user, authToken]);

  useEffect(() => {
    if (!user || activeDeployments.length === 0) return undefined;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [user, activeDeployments.length]);

  useEffect(() => {
    if (!selectedDeployment) return;
    const tabs = selectedDeployment.lab?.hardeningGuide || [];
    if (!tabs.length) {
      if (guideTabId) setGuideTabId("");
      return;
    }
    if (!tabs.some((tab) => tab.id === guideTabId)) {
      setGuideTabId(tabs[0].id);
    }
  }, [selectedDeployment, guideTabId]);

  useEffect(() => {
    if (!user) return undefined;
    const timer = setTimeout(() => {
      setSidebarCollapsed(true);
    }, 10000);
    return () => clearTimeout(timer);
  }, [user, view, selectedDeploymentId]);

  useEffect(() => {
    window.localStorage.setItem("labPortalDarkMode", darkMode ? "dark" : "light");
  }, [darkMode]);

  useEffect(() => {
    window.localStorage.setItem("labPortalNormalConsolePercent", String(normalConsolePercent));
  }, [normalConsolePercent]);

  useEffect(() => {
    window.localStorage.setItem("labPortalTheaterConsolePercent", String(theaterConsolePercent));
  }, [theaterConsolePercent]);

  useEffect(() => {
    if (view !== "guide" && consoleMode !== "normal") {
      setConsoleMode("normal");
    }
  }, [view, consoleMode]);

  useEffect(() => {
    if (!selectedLab) return undefined;

    function closeMachineDetail(event) {
      if (event.key === "Escape") {
        setSelectedLabId(null);
      }
    }

    window.addEventListener("keydown", closeMachineDetail);
    return () => window.removeEventListener("keydown", closeMachineDetail);
  }, [selectedLab]);

  if (!user) {
    return (
      <main className="login-shell">
        <section className="login-panel">
          <div>
            <p className="eyebrow">4rji</p>
            <h1>CCDC lab portal</h1>
          </div>
          <form onSubmit={login} className="login-form">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
            <button type="submit">
              <LogIn size={18} />
              Sign in
            </button>
          </form>
          {error && <p className="error">{error}</p>}
        </section>
      </main>
    );
  }

  return (
    <main className={`app-shell ${darkMode ? "dark-mode" : ""} ${sidebarCollapsed ? "sidebar-collapsed" : ""} ${theaterActive ? "theater-active" : ""}`}>
      {!theaterActive && (
        <aside
          className={`sidebar ${sidebarCollapsed ? "collapsed" : ""}`}
        >
          <div className="brand">
            <div className="brand-copy">
              <span>CCDC lab portal</span>
            </div>
            <button
              className="icon-button sidebar-toggle"
              title={sidebarCollapsed ? "Show sidebar" : "Hide sidebar"}
              onClick={() => setSidebarCollapsed((current) => !current)}
              aria-label={sidebarCollapsed ? "Show sidebar" : "Hide sidebar"}
              aria-expanded={!sidebarCollapsed}
            >
              {sidebarCollapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
            </button>
          </div>
          <nav>
            <button className={view === "catalog" ? "active" : ""} onClick={() => setView("catalog")}>
              <Server size={18} />
              <span className="sidebar-label">Catalog</span>
            </button>
            <button className={view === "topology" ? "active" : ""} onClick={() => setView("topology")}>
              <Network size={18} />
              <span className="sidebar-label">Topology</span>
            </button>
            <button className={view === "training1" ? "active" : ""} onClick={() => setView("training1")}>
              <Dumbbell size={18} />
              <span className="sidebar-label">Training 1</span>
            </button>
            <button className={view === "resources" ? "active" : ""} onClick={() => setView("resources")}>
              <Download size={18} />
              <span className="sidebar-label">Resources</span>
            </button>
            <button className={view === "active" ? "active" : ""} onClick={() => setView("active")}>
              <Activity size={18} />
              <span className="sidebar-label">Active</span>
            </button>
            <button
              className={view === "guide" ? "active" : ""}
              onClick={() => openDeploymentGuide()}
              disabled={!selectedDeployment}
            >
              <Terminal size={18} />
              <span className="sidebar-label">Guide</span>
            </button>
            <button className={view === "admin" ? "active" : ""} onClick={() => setView("admin")}>
              <Monitor size={18} />
              <span className="sidebar-label">Admin</span>
            </button>
          </nav>
          <footer>
            <div className="user-card">
              <span className="user-initial" aria-hidden="true">{user.username.slice(0, 1).toUpperCase()}</span>
              <span className="sidebar-label user-copy">
                <strong>{user.username}</strong>
                <small>{user.role}</small>
              </span>
            </div>
            <button className="icon-button logout-button" onClick={logout} title="Logout" aria-label="Logout">
              <LogOut size={18} />
            </button>
          </footer>
        </aside>
      )}

      <section className="workspace">
        {!theaterActive && (
          <header className="topbar">
            <div className={`topbar-title ${view === "guide" ? "guide-topbar-title" : ""}`}>
              {view === "guide" ? (
                <GuideMachineTabs
                  deployments={activeDeployments}
                  selectedDeployment={selectedDeployment}
                  onSelectDeployment={openDeploymentGuide}
                />
              ) : (
                <div>
                  <p className="eyebrow">{viewLabel.eyebrow}</p>
                  <h2>{viewLabel.title}</h2>
                </div>
              )}
            </div>
            <div className="topbar-actions">
              {error && <p className="error">{error}</p>}
              <button
                className="icon-button theme-toggle"
                onClick={() => setDarkMode((current) => !current)}
                title={darkMode ? "Light mode" : "Dark mode"}
                aria-label={darkMode ? "Light mode" : "Dark mode"}
              >
                {darkMode ? <Sun size={18} /> : <Moon size={18} />}
              </button>
            </div>
          </header>
        )}

        {view === "catalog" && (
          <div className={`catalog-layout ${selectedLab ? "has-detail" : ""}`}>
            <section className="machine-browser">
              {catalogLabSections.map((section) => (
                <div key={section.key} className="machine-card-section">
                  <div className="section-heading machine-row-heading">
                    <h3>{section.label}</h3>
                  </div>
                  <div className="machine-card-row">
                    {section.labs.map((lab) => (
                      <button
                        key={lab.id}
                        className={`machine-card ${osThemeClass(lab.category)} ${selectedLab?.id === lab.id ? "selected" : ""}`}
                        onClick={() => setSelectedLabId(lab.id)}
                        aria-pressed={selectedLab?.id === lab.id}
                      >
                        <span className="machine-card-heading">
                          <strong>{lab.name}</strong>
                          <small>{lab.description}</small>
                        </span>
                        <span className="machine-card-meta">
                          <span>
                            <strong>LAN</strong>
                            <span>{lab.network?.lan || "Pending"}</span>
                          </span>
                          <span>
                            <strong>Public</strong>
                            <span>{lab.network?.public || "Pending"}</span>
                          </span>
                        </span>
                        <span className="access-methods">
                          {(lab.accessMethods || []).map((method) => (
                            <span key={method}>{method}</span>
                          ))}
                        </span>
                        <span className="machine-card-mark" aria-hidden="true">
                          {lab.category === "windows" ? (
                            <span className="windows-mark">
                              <span />
                              <span />
                              <span />
                              <span />
                            </span>
                          ) : (
                            <img className="linux-mark" src={tuxLogoImage} alt="" />
                          )}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </section>

            {selectedLab && (
              <>
                <div
                  className="catalog-detail-backdrop"
                  onPointerDown={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                  }}
                  onClick={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    setSelectedLabId(null);
                  }}
                  aria-hidden="true"
                />
                <section className={`detail-panel machine-detail catalog-detail-float ${osThemeClass(selectedLab.category)}`}>
                  <div className="deployment-heading">
                    <div>
                      <p className="eyebrow">{selectedLab.platform} machine</p>
                      <h3>{selectedLab.name}</h3>
                      <p>{selectedLab.description}</p>
                    </div>
                    <div className="catalog-detail-actions">
                      <span className="platform-pill">{selectedLab.difficulty}</span>
                      <button
                        className="icon-button machine-detail-close"
                        onClick={() => setSelectedLabId(null)}
                        title="Close machine details"
                        aria-label="Close machine details"
                      >
                        <X size={18} />
                      </button>
                    </div>
                  </div>

                  <dl className="machine-network-detail">
                    <div><dt>LAN</dt><dd>{selectedLab.network?.lan || "Pending"}</dd></div>
                    <div><dt>Public</dt><dd>{selectedLab.network?.public || "Pending"}</dd></div>
                    <div><dt>TTL</dt><dd>{selectedLab.defaultTtlMinutes} min</dd></div>
                  </dl>
                  <section className="credentials-section">
                    <div className="section-heading">
                      <KeyRound size={18} />
                      <h4>Credentials</h4>
                    </div>
                    <div className="credential-grid">
                      {(selectedLab.credentials || []).map((credential) => (
                        <div key={`${credential.username}-${credential.password}`}>
                          <span>{credential.username}</span>
                          <code>{credential.password}</code>
                        </div>
                      ))}
                    </div>
                  </section>

                  <button onClick={() => deploy(selectedLab.id)} disabled={busyLabId === selectedLab.id}>
                    <Play size={18} />
                    {busyLabId === selectedLab.id ? "Deploying" : "Deploy machine"}
                  </button>

                  <section className="hardening-section">
                    <div className="section-heading">
                      <Shield size={18} />
                      <h4>{selectedLab.platform} pre-deployment hardening checklist</h4>
                    </div>
                    <ol className="hardening-list">
                      {(selectedLab.hardeningSteps || []).map((step) => (
                        <li key={step}>{step}</li>
                      ))}
                    </ol>
                  </section>

                  <section className="access-section">
                    <div className="section-heading">
                      <Network size={18} />
                      <h4>Access methods</h4>
                    </div>
                    <div className="access-methods">
                      {(selectedLab.accessMethods || []).map((method) => (
                        <span key={method}>{method}</span>
                      ))}
                    </div>
                  </section>
                </section>
              </>
            )}
          </div>
        )}

        {view === "topology" && (
          <div className="topology-layout">
            <CredentialsPreview />
            <LabTopologyPanel />
          </div>
        )}

        {view === "training1" && (
          <TrainingOne />
        )}

        {view === "resources" && (
          <ResourcesPanel
            resources={downloadResources}
            busyResourceId={busyResourceId}
            onDownload={downloadResource}
          />
        )}

        {view === "active" && (
          <div className="active-layout">
            <section className="deployment-list">
              {activeDeployments.length === 0 && <p className="muted">No active deployments.</p>}
              {activeDeployments.map((deployment) => (
                <button
                  key={deployment.id}
                  className={`deployment-row ${osThemeClass(deployment.lab?.category)} ${selectedDeployment?.id === deployment.id ? "selected" : ""}`}
                  onClick={() => selectDeployment(deployment)}
                >
                  <span>
                    <strong>{deployment.lab.name}</strong>
                    <small>{deploymentResourceName(deployment)}</small>
                    {deployment.lastError && <small className="row-error">{deployment.lastError}</small>}
                  </span>
                  <StatusPill status={deployment.status} />
                </button>
              ))}
            </section>

            {selectedDeployment && (
              <section className={`detail-panel ${osThemeClass(selectedDeployment.lab?.category)}`}>
                <div className="deployment-heading">
                  <div>
                    <p className="eyebrow">{selectedDeployment.provider}</p>
                    <h3>{selectedDeployment.lab.name}</h3>
                  </div>
                  <StatusPill status={selectedDeployment.status} />
                </div>
                <dl className="deployment-data">
                  <div><dt>Resource</dt><dd>{deploymentResourceName(selectedDeployment)}</dd></div>
                  <div><dt>Time left</dt><dd>{fmtTimeLeft(selectedDeployment.expiresAt, now)}</dd></div>
                  <div><dt>Updated</dt><dd>{fmtDate(selectedDeployment.updatedAt)}</dd></div>
                </dl>
                <AsciiDiagram
                  label="session"
                  lines={[
                    `${user.username}@portal`,
                    "  |",
                    `  +-- ${selectedDeployment.provider}`,
                    `      +-- ${deploymentResourceName(selectedDeployment)}`
                  ]}
                />
                {isStartingDeployment(selectedDeployment) && (
                  <VmStartupMarker deployment={selectedDeployment} now={now} />
                )}
                {selectedDeployment.lastError && (
                  <section className="error-panel">
                    <span>Last error</span>
                    <code>{selectedDeployment.lastError}</code>
                  </section>
                )}
                <div className="outputs">
                  {(selectedDeployment.outputs || []).filter((output) => !hiddenActiveOutputKeys.has(String(output.key).toLowerCase())).length === 0 && (
                    <p className="muted outputs-empty">No outputs yet. Refresh runs automatically while the machine is building.</p>
                  )}
                  {(selectedDeployment.outputs || [])
                    .filter((output) => !hiddenActiveOutputKeys.has(String(output.key).toLowerCase()))
                    .map((output) => (
                    <div key={output.key}>
                      <span>{output.key}</span>
                      <code>{output.sensitive ? "Hidden" : fmtValue(output.value)}</code>
                    </div>
                  ))}
                </div>
                <div className="actions">
                  <button onClick={() => openDeploymentGuide(selectedDeployment)}>
                    <Terminal size={18} />
                    Open guide
                  </button>
                  <button onClick={() => action(`/api/deployments/${selectedDeployment.id}/start`)}>
                    <Power size={18} />
                    Start
                  </button>
                  <button onClick={() => action(`/api/deployments/${selectedDeployment.id}/reset`)}>
                    <RotateCcw size={18} />
                    Reset
                  </button>
                  <button className="danger" onClick={() => action(`/api/deployments/${selectedDeployment.id}`, "DELETE")}>
                    <Trash2 size={18} />
                    Destroy
                  </button>
                </div>
              </section>
            )}
          </div>
        )}

        {view === "guide" && (
          <GuideWorkspace
            authToken={authToken}
            deployment={selectedDeployment}
            guideTabs={guideTabs}
            selectedGuideTab={selectedGuideTab}
            completedGuideSteps={completedGuideSteps}
            consoleMode={consoleMode}
            normalConsolePercent={normalConsolePercent}
            theaterConsolePercent={theaterConsolePercent}
            now={now}
            onSelectGuideTab={setGuideTabId}
            onSetConsoleMode={setConsoleMode}
            onSetNormalConsolePercent={setNormalConsolePercent}
            onSetTheaterConsolePercent={setTheaterConsolePercent}
            onToggleGuideStep={toggleGuideStep}
          />
        )}

        {view === "admin" && (
          <AdminPanel
            deployments={deployments}
            activeDeployments={activeDeployments}
            labs={labs}
            resourceSummary={resourceSummary}
          />
        )}
      </section>
    </main>
  );
}

function GuideMachineTabs({ deployments, selectedDeployment, onSelectDeployment }) {
  if (deployments.length === 0) {
    return (
      <div>
        <p className="eyebrow">Guide</p>
        <h2>No machines available</h2>
      </div>
    );
  }

  return (
    <div className="guide-machine-tabs" role="tablist" aria-label="Machines available for hardening">
      {deployments.map((deployment) => {
        const isSelected = selectedDeployment?.id === deployment.id;

        return (
          <button
            key={deployment.id}
            className={`${osThemeClass(deployment.lab?.category)} ${isSelected ? "active" : ""}`}
            onClick={() => onSelectDeployment(deployment)}
            role="tab"
            aria-selected={isSelected}
          >
            <DeploymentIcon deployment={deployment} size={16} />
            <span className="guide-machine-tab-copy">
              <strong>{deployment.lab?.name || deploymentResourceName(deployment)}</strong>
              <small>{statusLabels[deployment.status] || deployment.status}</small>
            </span>
          </button>
        );
      })}
    </div>
  );
}

function DeploymentIcon({ deployment, size = 16 }) {
  if (deployment.lab?.category === "windows") return <Monitor size={size} />;
  return <Terminal size={size} />;
}

function AsciiDiagram({ label, lines }) {
  return (
    <pre className="ascii-diagram" aria-label={label}>
      <span>{`# ${label}`}</span>
      {lines.join("\n")}
    </pre>
  );
}

function StatusPill({ status }) {
  return <span className={`status ${status}`}>{statusLabels[status] || status}</span>;
}

function InlineCodeText({ text }) {
  const parts = text.split(/(`[^`]+`)/g);
  return parts.map((part, index) => {
    if (part.startsWith("`") && part.endsWith("`")) {
      return <code key={`${part}-${index}`}>{part.slice(1, -1)}</code>;
    }
    return part;
  });
}

function TrainingOne() {
  return (
    <div className="training-layout">
      <section className="guide-content training-content os-linux">
        <div className="deployment-heading">
          <div>
            <p className="eyebrow">Redhavi Cleanup Task</p>
            <h3>Redhavi Cleanup Task</h3>
            <p>
              The system was prepared as a CCDC lab with multiple misconfigurations. Your task is to investigate the
              system, identify suspicious changes, and leave it in a clean state.
            </p>
            <p>
              Do not assume everything bad is in one place. Review users, services, scheduled tasks, web files,
              permissions, installed packages, and persistence.
            </p>
          </div>
        </div>

        <section className="training-section">
          <h4>Rules</h4>
          <ul className="training-list">
            <li>Document what you find before changing it.</li>
            <li>Do not delete evidence unless you understand what it does.</li>
            <li>Make small changes and verify again after each group of changes.</li>
            <li>Do not use automatic cleanup scripts if you cannot explain what they modify.</li>
            <li>At the end, run the lab verification and compare the result.</li>
          </ul>
        </section>

        <section className="training-section">
          <h4>Areas To Review</h4>
          <div className="training-section-grid">
            {trainingOneSections.map((section) => (
              <article key={section.title} className="training-checklist">
                <h5>{section.title}</h5>
                <ul className="training-list">
                  {section.items.map((item) => (
                    <li key={item}><InlineCodeText text={item} /></li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        </section>

        <section className="training-section">
          <h4>Deliverable</h4>
          <p>Prepare a short summary with:</p>
          <ul className="training-list compact">
            <li>Main findings.</li>
            <li>Risk of each finding.</li>
            <li>Changes performed.</li>
            <li>Final verification.</li>
            <li>Items that require follow-up.</li>
          </ul>
        </section>

        <section className="training-section final-hint">
          <h4>Final Hint</h4>
          <p>
            If a verification fails, do not treat it as a direct answer. Use it as a starting point to investigate why
            that state exists and what keeps it active.
          </p>
        </section>
      </section>
    </div>
  );
}

function ResourcesPanel({ resources, busyResourceId, onDownload }) {
  return (
    <div className="resources-layout">
      <section className="download-library">
        <div className="section-heading">
          <Download size={18} />
          <h4>Team Pack</h4>
        </div>
        <TeamPackDownloads />
      </section>

      <section className="download-library">
        <div className="section-heading">
          <BookOpen size={18} />
          <h4>Mastering Books</h4>
        </div>
        <div className="download-grid">
          {resources.length === 0 && <p className="muted">No resources available.</p>}
          {resources.map((resource) => {
            const themeClass = osThemeClass(String(resource.platform || "linux").toLowerCase());
            const isBusy = busyResourceId === resource.id;

            return (
              <article className={`download-card ${themeClass}`} key={resource.id}>
                <div className="download-card-heading">
                  <FileText size={22} />
                  <div>
                    <strong>{resource.title}</strong>
                    <small>{resource.platform}</small>
                  </div>
                </div>
                <dl className="download-meta">
                  <div><dt>Format</dt><dd>{resource.format}</dd></div>
                  <div><dt>Size</dt><dd>{fmtBytes(resource.sizeBytes)}</dd></div>
                </dl>
                <button
                  onClick={() => onDownload(resource)}
                  disabled={!resource.available || isBusy}
                >
                  <Download size={18} />
                  {isBusy ? "Downloading" : "Download"}
                </button>
              </article>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function AdminPanel({ deployments, activeDeployments, labs, resourceSummary }) {
  const resources = resourceSummary?.resources || [];

  return (
    <section className="admin-panel">
      <div className="admin-grid">
        <div>
          <span className="metric-value">{deployments.length}</span>
          <span>Total deployments</span>
        </div>
        <div>
          <span className="metric-value">{activeDeployments.length}</span>
          <span>Active deployments</span>
        </div>
        <div>
          <span className="metric-value">{labs.length}</span>
          <span>Enabled labs</span>
        </div>
      </div>

      <section className="resource-panel">
        <div className="deployment-heading">
          <div>
            <p className="eyebrow">Capacity</p>
            <h3>Server resources</h3>
          </div>
          <span className="platform-pill">{resourceSummary?.source || "loading"}</span>
        </div>
        <div className="resource-grid">
          {resources.length === 0 && <p className="muted">Resource usage is not available yet.</p>}
          {resources.map((metric) => {
            const percent = resourcePercent(metric);
            return (
              <div className="resource-card" key={metric.key}>
                <div className="resource-card-heading">
                  <strong>{metric.label}</strong>
                  <span>{percent === null ? "N/A" : `${percent}% used`}</span>
                </div>
                <AsciiDiagram
                  label={metric.key}
                  lines={[
                    `used  [${"#".repeat(Math.max(1, Math.round((percent ?? 0) / 12.5))).padEnd(8, ".")}]`,
                    `free  ${fmtResourceValue(metric.available, metric.unit)}`,
                    `total ${fmtResourceValue(metric.total, metric.unit)}`
                  ]}
                />
                <div className="resource-bar" aria-hidden="true">
                  <span style={{ width: `${percent ?? 0}%` }} />
                </div>
                <dl>
                  <div><dt>Used</dt><dd>{fmtResourceValue(metric.used, metric.unit)}</dd></div>
                  <div><dt>Available</dt><dd>{fmtResourceValue(metric.available, metric.unit)}</dd></div>
                  <div><dt>Total</dt><dd>{fmtResourceValue(metric.total, metric.unit)}</dd></div>
                </dl>
              </div>
            );
          })}
        </div>
        {(resourceSummary?.warnings || []).map((warning) => (
          <p className="resource-warning" key={warning}>{warning}</p>
        ))}
      </section>
    </section>
  );
}

function VmStartupMarker({ deployment, now }) {
  const startedAt = deployment.createdAt || deployment.updatedAt;
  const elapsed = fmtElapsed(startedAt, now);
  const label = deployment.status === "queued" ? "Request sent" : "Starting VM";

  return (
    <section className="startup-marker" aria-live="polite">
      <div className="startup-marker-main">
        <span className="startup-spinner" aria-hidden="true" />
        <div>
          <strong>{label}</strong>
          <span>{deploymentResourceName(deployment)}</span>
        </div>
      </div>
      <div className="startup-marker-meta">
        <span>{elapsed}</span>
        <span>{statusLabels[deployment.status] || deployment.status}</span>
      </div>
      <div className="startup-progress" aria-hidden="true">
        <span />
      </div>
    </section>
  );
}

function LabTopologyPreview() {
  return (
    <div className="topology-preview" aria-label="Mini CCDC lab topology preview">
      <img src={labTopologyImage} alt="Mini CCDC topology with VyOS, Palo Alto, Cisco FTD, Linux hosts, and Windows hosts" />
    </div>
  );
}

function LabTopologyPanel() {
  return (
    <section className="topology-guide topology-page-panel topology-image-panel">
      <div className="section-heading">
        <Network size={18} />
        <h4>Lab map</h4>
      </div>
      <LabTopologyPreview />
    </section>
  );
}

function CredentialsPreview() {
  return (
    <section className="topology-guide topology-page-panel topology-image-panel" aria-label="Credentials image">
      <div className="section-heading">
        <KeyRound size={18} />
        <h4>Credentials</h4>
      </div>
      <div className="topology-preview credentials-preview">
        <img src={credentialsImage} alt="CCDC credentials reference" />
      </div>
    </section>
  );
}

function TeamPackDownloads() {
  const downloads = [
    {
      href: teamPackPdf,
      label: "Team Pack",
      filename: "2025MWCCDCITeamPack_.pdf"
    },
    {
      href: regionalTeamPackPdf,
      label: "Regional Team Pack",
      filename: "2026MWCCDCRegionalTeamPack.pdf"
    }
  ];

  return (
    <div className="resource-downloads" aria-label="Team pack downloads">
      {downloads.map((download) => (
        <a
          key={download.filename}
          href={download.href}
          download={download.filename}
          className="resource-download-link"
          title={`Download ${download.filename}`}
        >
          <span>{download.label}</span>
          <Download size={18} aria-hidden="true" />
        </a>
      ))}
    </div>
  );
}

function GuideWorkspace({
  authToken,
  deployment,
  guideTabs,
  selectedGuideTab,
  completedGuideSteps,
  consoleMode,
  normalConsolePercent,
  theaterConsolePercent,
  now,
  onSelectGuideTab,
  onSetConsoleMode,
  onSetNormalConsolePercent,
  onSetTheaterConsolePercent,
  onToggleGuideStep
}) {
  if (!deployment) {
    return (
      <section className="guide-empty">
        <div className="detail-panel">
          <p className="eyebrow">Guide</p>
          <h3>No deployment selected</h3>
          <p className="muted">Deploy a machine or select an active deployment to open the guided hardening workspace.</p>
        </div>
      </section>
    );
  }

  const roleGuide = deployment.lab?.roleGuide;
  const activeConsoleUrl = consoleUrl(deployment);
  const proxmoxConsole = deployment.provider === "proxmox";
  const themeClass = osThemeClass(deployment.lab?.category);
  const theaterActive = consoleMode === "theater";
  const consolePercent = theaterActive ? theaterConsolePercent : normalConsolePercent;
  const layoutStyle = {
    "--console-pane": `${consolePercent}%`,
    "--guide-pane": `${100 - consolePercent}%`
  };

  function startConsoleResize(event) {
    event.preventDefault();
    const layout = event.currentTarget.closest(".guide-layout");
    const bounds = layout?.getBoundingClientRect();
    if (!bounds) return;
    const min = theaterActive ? 55 : 48;
    const max = theaterActive ? 92 : 78;
    const setter = theaterActive ? onSetTheaterConsolePercent : onSetNormalConsolePercent;

    function resize(moveEvent) {
      const nextPercent = ((moveEvent.clientX - bounds.left) / bounds.width) * 100;
      setter(Math.min(max, Math.max(min, Math.round(nextPercent))));
    }

    function stopResize() {
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", stopResize);
      document.body.classList.remove("resizing-console");
    }

    document.body.classList.add("resizing-console");
    window.addEventListener("pointermove", resize);
    window.addEventListener("pointerup", stopResize, { once: true });
  }

  return (
    <div className={`guide-layout ${themeClass} ${theaterActive ? "theater" : "normal"}`} style={layoutStyle}>
      <section className="terminal-panel">
        <div className="terminal-toolbar">
          <div>
            <p className="eyebrow">Console</p>
            <h3>{deployment.lab.name}</h3>
          </div>
          <div className="console-toolbar-actions">
            <StatusPill status={deployment.status} />
            <button
              className={consoleMode === "normal" ? "active" : ""}
              onClick={() => onSetConsoleMode("normal")}
              title="Normal mode"
            >
              <Columns2 size={16} />
              Normal
            </button>
            <button
              className={consoleMode === "theater" ? "active" : ""}
              onClick={() => onSetConsoleMode("theater")}
              title="Theater mode"
            >
              <RectangleHorizontal size={16} />
              Theater
            </button>
            <button
              onClick={() => {
                const element = document.querySelector(".console-frame");
                element?.requestFullscreen?.();
              }}
              title="Fullscreen console"
            >
              <Monitor size={16} />
              Fullscreen
            </button>
          </div>
        </div>

        <div className="terminal-screen" aria-label="Machine noVNC console">
          {isStartingDeployment(deployment) && (
            <VmStartupMarker deployment={deployment} now={now} />
          )}
          <div className="console-frame">
            {proxmoxConsole && activeConsoleUrl ? (
              <ProxmoxConsole deployment={deployment} authToken={authToken} />
            ) : activeConsoleUrl ? (
              <iframe
                title={`${deployment.lab.name} console`}
                src={activeConsoleUrl}
                sandbox="allow-same-origin allow-scripts allow-forms allow-popups"
                allow="clipboard-read; clipboard-write; fullscreen"
              />
            ) : (
              <div className="console-placeholder">
                <Terminal size={28} />
                <span>
                  {deployment.lastError
                    ? "Deployment error"
                    : "noVNC console"}
                </span>
                <code>
                  {deployment.lastError
                    || "Console URL unavailable"}
                </code>
              </div>
            )}
          </div>
        </div>
      </section>

      <button
        className="console-resizer"
        onPointerDown={startConsoleResize}
        title="Drag to resize console"
        aria-label="Resize console"
      />

      <section className="guide-content">
        <div className="deployment-heading">
          <div>
            <p className="eyebrow">{deployment.lab.platform} hardening</p>
            <h3>{deployment.lab.name}</h3>
            <p>{deployment.lab.description}</p>
          </div>
          <span className="platform-pill">{deployment.lab.difficulty}</span>
        </div>

        <dl className="guide-meta">
          <div><dt>Resource</dt><dd>{deploymentResourceName(deployment)}</dd></div>
          <div><dt>Time left</dt><dd>{fmtTimeLeft(deployment.expiresAt, now)}</dd></div>
          <div><dt>Updated</dt><dd>{fmtDate(deployment.updatedAt)}</dd></div>
        </dl>

        <div className="guide-tabs" role="tablist" aria-label="Hardening guide sections">
          {guideTabs.map((tab) => (
            <button
              key={tab.id}
              className={selectedGuideTab?.id === tab.id ? "active" : ""}
              onClick={() => onSelectGuideTab(tab.id)}
              role="tab"
              aria-selected={selectedGuideTab?.id === tab.id}
            >
              {tab.title}
            </button>
          ))}
        </div>

        {selectedGuideTab && (
          <div className="guide-tab-panel">
            <p>{selectedGuideTab.summary}</p>
            <div className="guide-step-list">
              {selectedGuideTab.steps.map((step, index) => {
                const stepKey = guideStepKey(deployment.id, selectedGuideTab.id, index);
                const isComplete = Boolean(completedGuideSteps[stepKey]);

                return (
                  <article key={`${selectedGuideTab.id}-${step.title}`} className={`guide-step ${isComplete ? "complete" : ""}`}>
                    <label>
                      <input
                        type="checkbox"
                        checked={isComplete}
                        onChange={() => onToggleGuideStep(deployment.id, selectedGuideTab.id, index)}
                      />
                      <span>
                        <strong>{step.title}</strong>
                        <small>{step.detail}</small>
                      </span>
                    </label>
                    {step.commands?.length > 0 && (
                      <div className="guide-command-list">
                        {step.commands.map((command) => (
                          <code key={command}>{command}</code>
                        ))}
                      </div>
                    )}
                  </article>
                );
              })}
            </div>
          </div>
        )}

        {roleGuide && (
          <aside className="role-focus">
            <div className="section-heading">
              <Shield size={18} />
              <h4>{roleGuide.title}</h4>
            </div>
            <ul>
              {roleGuide.steps.map((step) => (
                <li key={step}>{step}</li>
              ))}
            </ul>
            {roleGuide.commands?.length > 0 && (
              <div className="guide-command-list">
                {roleGuide.commands.map((command) => (
                  <code key={command}>{command}</code>
                ))}
              </div>
            )}
          </aside>
        )}
      </section>
    </div>
  );
}

function ProxmoxConsole({ deployment, authToken }) {
  const targetRef = useRef(null);
  const [status, setStatus] = useState("Connecting to Proxmox console...");
  const [error, setError] = useState("");

  useAutoDismissMessage(error, setError);

  useEffect(() => {
    if (!deployment?.id || !authToken) return undefined;

    let cancelled = false;
    let rfb = null;
    setStatus("Requesting console ticket...");
    setError("");

    async function connect() {
      const payload = await api.request(`/api/deployments/${deployment.id}/console-session`, {
        method: "POST"
      }, authToken);

      if (cancelled || !targetRef.current) return;

      const websocketUrl = api.websocketUrl(payload.session.websocketPath);
      targetRef.current.replaceChildren();
      rfb = new RFB(targetRef.current, websocketUrl, {
        credentials: {
          password: payload.session.password
        },
        shared: true
      });
      rfb.scaleViewport = true;
      rfb.resizeSession = true;
      rfb.focusOnClick = true;

      rfb.addEventListener("connect", () => setStatus("Connected"));
      rfb.addEventListener("disconnect", (event) => {
        setStatus(event.detail?.clean ? "Disconnected" : "Console disconnected");
      });
      rfb.addEventListener("securityfailure", (event) => {
        setError(event.detail?.reason || "Proxmox console authentication failed");
      });
      rfb.addEventListener("credentialsrequired", () => {
        setError("The console requested credentials unexpectedly");
      });
    }

    connect().catch((err) => {
      if (!cancelled) {
        setError(err.message);
        setStatus("Console unavailable");
      }
    });

    return () => {
      cancelled = true;
      if (rfb) rfb.disconnect();
    };
  }, [deployment?.id, authToken]);

  return (
    <div className="proxmox-console">
      <div ref={targetRef} className="proxmox-console-target" />
      {(status || error) && (
        <div className={`proxmox-console-status ${error ? "error" : ""}`}>
          <span>{error || status}</span>
        </div>
      )}
    </div>
  );
}
