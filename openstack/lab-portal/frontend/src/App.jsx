import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Columns2,
  KeyRound,
  LogIn,
  Monitor,
  Network,
  Play,
  Power,
  RotateCcw,
  Server,
  Shield,
  Terminal,
  Trash2,
  RectangleHorizontal
} from "lucide-react";

const labTopologyImage = new URL("../../topology.png", import.meta.url).href;

const api = {
  async request(path, options = {}, token = "") {
    const authHeaders = token ? { Authorization: `Bearer ${token}` } : {};
    const apiBaseUrl = window.labPortal?.apiBaseUrl || import.meta.env.VITE_API_BASE_URL || "";
    const response = await fetch(`${apiBaseUrl}${path}`, {
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

const catalogTabs = [
  { id: "test", label: "Prueba" },
  { id: "linux", label: "Linux" },
  { id: "windows", label: "Windows" }
];

function osThemeClass(category = "linux") {
  return `os-${category}`;
}

const viewLabels = {
  catalog: { eyebrow: "catalog", title: "CCDC catalog" },
  active: { eyebrow: "active", title: "Deployments" },
  guide: { eyebrow: "guide", title: "Hardening guide" },
  admin: { eyebrow: "admin", title: "Operations" }
};

function fmtDate(value) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    day: "numeric"
  }).format(new Date(value));
}

function fmtValue(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

function consoleUrl(deployment) {
  const consoleOutput = deployment?.outputs?.find((output) => (
    ["console_url", "novnc_url", "vnc_url"].includes(output.key)
  ));
  return consoleOutput?.value || deployment?.consoleUrl || "";
}

function guideStepKey(deploymentId, tabId, stepIndex) {
  return `${deploymentId}:${tabId}:${stepIndex}`;
}

export default function App() {
  const [user, setUser] = useState(null);
  const [username, setUsername] = useState("havi");
  const [password, setPassword] = useState("metro123");
  const [authToken, setAuthToken] = useState("");
  const [labs, setLabs] = useState([]);
  const [deployments, setDeployments] = useState([]);
  const [selectedLabId, setSelectedLabId] = useState(null);
  const [selectedDeploymentId, setSelectedDeploymentId] = useState(null);
  const [catalogTab, setCatalogTab] = useState("linux");
  const [view, setView] = useState("catalog");
  const [error, setError] = useState("");
  const [busyLabId, setBusyLabId] = useState("");
  const [guideTabId, setGuideTabId] = useState("");
  const [completedGuideSteps, setCompletedGuideSteps] = useState({});
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [sidebarHovered, setSidebarHovered] = useState(false);

  const activeCatalogLabs = useMemo(
    () => labs.filter((lab) => (lab.category || "linux") === catalogTab),
    [catalogTab, labs]
  );
  const selectedLab = useMemo(
    () => activeCatalogLabs.find((lab) => lab.id === selectedLabId) || activeCatalogLabs[0] || labs[0],
    [activeCatalogLabs, labs, selectedLabId]
  );
  const activeDeployments = deployments.filter((deployment) => deployment.status !== "deleted");
  const selectedDeployment = activeDeployments.find((deployment) => deployment.id === selectedDeploymentId)
    || activeDeployments[0];
  const guideTabs = selectedDeployment?.lab?.hardeningGuide || [];
  const selectedGuideTab = guideTabs.find((tab) => tab.id === guideTabId) || guideTabs[0];
  const viewLabel = viewLabels[view] || viewLabels.active;

  async function loadData(currentToken = authToken) {
    if (!currentToken) return;
    const [labsPayload, deploymentsPayload] = await Promise.all([
      api.request("/api/labs", {}, currentToken),
      api.request("/api/deployments", {}, currentToken)
    ]);
    setLabs(labsPayload.labs);
    setDeployments(deploymentsPayload.deployments);
    setSelectedLabId((current) => {
      if (labsPayload.labs.some((lab) => lab.id === current)) return current;
      return labsPayload.labs.find((lab) => (lab.category || "linux") === catalogTab)?.id
        || labsPayload.labs[0]?.id;
    });
  }

  function selectCatalogTab(tabId) {
    setCatalogTab(tabId);
    const nextLab = labs.find((lab) => (lab.category || "linux") === tabId);
    if (nextLab) setSelectedLabId(nextLab.id);
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

  async function deploy(labId) {
    setBusyLabId(labId);
    setError("");
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
    if (!user || sidebarHovered) return undefined;
    const timer = setTimeout(() => {
      setSidebarCollapsed(true);
    }, 10000);
    return () => clearTimeout(timer);
  }, [user, sidebarHovered, view, catalogTab, selectedDeploymentId]);

  if (!user) {
    return (
      <main className="login-shell">
        <section className="login-panel">
          <div>
            <p className="eyebrow">OpenStack</p>
            <h1>Lab Portal</h1>
          </div>
          <form onSubmit={login} className="login-form">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
            />
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
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
    <main className={`app-shell ${sidebarCollapsed ? "sidebar-collapsed" : ""}`}>
      <aside
        className={`sidebar ${sidebarCollapsed ? "collapsed" : ""}`}
        onMouseEnter={() => {
          setSidebarHovered(true);
          setSidebarCollapsed(false);
        }}
        onMouseLeave={() => setSidebarHovered(false)}
      >
        <div className="brand">
          <div className="brand-copy">
            <strong>OpenStack</strong>
            <span>Lab Portal</span>
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
          <span className="sidebar-label">{user.username}</span>
        </footer>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">{viewLabel.eyebrow}</p>
            <h2>{viewLabel.title}</h2>
          </div>
          {error && <p className="error">{error}</p>}
        </header>

        {view === "catalog" && (
          <div className={`catalog-layout ${osThemeClass(catalogTab)}`}>
            <section className="machine-browser">
              <section className="topology-guide">
                <div className="section-heading">
                  <Network size={18} />
                  <h4>Lab topology guide</h4>
                </div>
                <LabTopologyPreview />
              </section>

              <div className="catalog-tabs" role="tablist" aria-label="Machine operating system">
                {catalogTabs.map((tab) => (
                  <button
                    key={tab.id}
                    className={`${catalogTab === tab.id ? "active" : ""} ${osThemeClass(tab.id)}`}
                    onClick={() => selectCatalogTab(tab.id)}
                    role="tab"
                    aria-selected={catalogTab === tab.id}
                  >
                    {tab.id === "linux" && <Terminal size={16} />}
                    {tab.id === "windows" && <Monitor size={16} />}
                    {tab.id === "test" && <Server size={16} />}
                    {tab.label}
                  </button>
                ))}
              </div>

              <div className="machine-card-grid">
                {activeCatalogLabs.map((lab) => (
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
                  </button>
                ))}
              </div>
            </section>

            {selectedLab && (
              <section className={`detail-panel machine-detail ${osThemeClass(selectedLab.category)}`}>
                <div className="deployment-heading">
                  <div>
                    <p className="eyebrow">{selectedLab.platform} machine</p>
                    <h3>{selectedLab.name}</h3>
                    <p>{selectedLab.description}</p>
                  </div>
                  <span className="platform-pill">{selectedLab.difficulty}</span>
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

                <button onClick={() => deploy(selectedLab.id)} disabled={busyLabId === selectedLab.id}>
                  <Play size={18} />
                  {busyLabId === selectedLab.id ? "Deploying" : "Deploy machine"}
                </button>
              </section>
            )}
          </div>
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
                    <small>{deployment.heatStackName}</small>
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
                <dl className="stack-data">
                  <div><dt>Resource</dt><dd>{selectedDeployment.heatStackName}</dd></div>
                  <div><dt>Expires</dt><dd>{fmtDate(selectedDeployment.expiresAt)}</dd></div>
                  <div><dt>Updated</dt><dd>{fmtDate(selectedDeployment.updatedAt)}</dd></div>
                </dl>
                {selectedDeployment.lastError && (
                  <section className="error-panel">
                    <span>Last error</span>
                    <code>{selectedDeployment.lastError}</code>
                  </section>
                )}
                <div className="outputs">
                  {(selectedDeployment.outputs || []).length === 0 && (
                    <p className="muted outputs-empty">No outputs yet. Refresh runs automatically while the machine is building.</p>
                  )}
                  {(selectedDeployment.outputs || []).map((output) => (
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
            deployment={selectedDeployment}
            guideTabs={guideTabs}
            selectedGuideTab={selectedGuideTab}
            completedGuideSteps={completedGuideSteps}
            onSelectGuideTab={setGuideTabId}
            onToggleGuideStep={toggleGuideStep}
          />
        )}

        {view === "admin" && (
          <section className="admin-grid">
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
          </section>
        )}
      </section>
    </main>
  );
}

function StatusPill({ status }) {
  return <span className={`status ${status}`}>{statusLabels[status] || status}</span>;
}

function LabTopologyPreview() {
  return (
    <div className="topology-preview" aria-label="Mini CCDC lab topology preview">
      <img src={labTopologyImage} alt="Mini CCDC topology with VyOS, Palo Alto, Cisco FTD, Linux hosts, and Windows hosts" />
    </div>
  );
}

function GuideWorkspace({
  deployment,
  guideTabs,
  selectedGuideTab,
  completedGuideSteps,
  onSelectGuideTab,
  onToggleGuideStep
}) {
  const [consoleMode, setConsoleMode] = useState("normal");

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
  const themeClass = osThemeClass(deployment.lab?.category);

  return (
    <div className={`guide-layout ${themeClass} ${consoleMode === "theater" ? "theater" : "normal"}`}>
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
              onClick={() => setConsoleMode("normal")}
              title="Normal mode"
            >
              <Columns2 size={16} />
              Normal
            </button>
            <button
              className={consoleMode === "theater" ? "active" : ""}
              onClick={() => setConsoleMode("theater")}
              title="Theater mode"
            >
              <RectangleHorizontal size={16} />
              Theater
            </button>
          </div>
        </div>

        <div className="terminal-screen" aria-label="Machine noVNC console">
          <div className="console-frame">
            {activeConsoleUrl ? (
              <iframe
                title={`${deployment.lab.name} console`}
                src={activeConsoleUrl}
                sandbox="allow-same-origin allow-scripts allow-forms allow-popups"
                allow="clipboard-read; clipboard-write; fullscreen"
              />
            ) : (
              <div className="console-placeholder">
                <Terminal size={28} />
                <span>{deployment.lastError ? "Deployment error" : "noVNC console"}</span>
                <code>{deployment.lastError || `openstack console url show --novnc ${deployment.heatStackName}`}</code>
              </div>
            )}
          </div>
        </div>
      </section>

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
          <div><dt>Resource</dt><dd>{deployment.heatStackName}</dd></div>
          <div><dt>Expires</dt><dd>{fmtDate(deployment.expiresAt)}</dd></div>
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
