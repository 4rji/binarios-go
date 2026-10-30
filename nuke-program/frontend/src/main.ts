import {Events} from "@wailsio/runtime";
import {VaultService} from "../bindings/github.com/4rji/nuke-program";

type VaultInfo = {
    id: string;
    name: string;
    createdAt: string;
    size: number;
};

type MountResult = {
    url: string;
    mountPoint: string;
    automatic: boolean;
    message: string;
};

type UnlockState = {
    vaultId: string;
    status: string;
    message: string;
    attempts: number;
    deadline: number;
    mount: MountResult;
};

type VaultEvent = {
    vaultId: string;
    status: string;
    message: string;
    attempts: number;
    deadline: number;
    phase: string;
    percent: number;
};

const app = document.querySelector<HTMLElement>("#app")!;
let selectedVault: VaultInfo | null = null;
let countdownTimer: number | undefined;
let destructionState: "none" | "nuking" | "wiping" | "destroyed" = "none";

function shell(content: string, tone = ""): string {
    return `
      <section class="shell ${tone}">
        <header class="brand-row">
          <div class="brand-mark" aria-hidden="true">N</div>
          <div>
            <p class="eyebrow">FORENSIC LAB / CONTROLLED DESTRUCTION</p>
            <h1>NUKE<span>VAULT</span></h1>
          </div>
          <div class="platform-pill">v0.1.3 · NO FUSE</div>
        </header>
        ${content}
        <footer>
          <span>AES-256-GCM</span><span>ARGON2ID</span><span>LOCAL WEBDAV</span><span>LAB LIMIT 256 MiB</span>
        </footer>
      </section>`;
}

function stopCountdown() {
    if (countdownTimer !== undefined) {
        window.clearInterval(countdownTimer);
        countdownTimer = undefined;
    }
}

function renderSetup(error = "") {
    stopCountdown();
    selectedVault = null;
    app.innerHTML = shell(`
      <div class="panel setup-panel">
        <div class="status-line"><span class="status-dot safe"></span>NO LAB CONTAINER PRESENT</div>
        <h2>Create a disposable encrypted vault</h2>
        <p class="lede">This container is intentionally hostile to missed and incorrect authentication. Use test evidence only.</p>
        ${error ? `<p class="alert">${escapeHTML(error)}</p>` : ""}
        <form id="create-form" class="form-stack" autocomplete="off">
          <label>Vault name<input id="vault-name" maxlength="64" value="Evidence Lab" required /></label>
          <label>Unlock password<input id="unlock-password" type="password" minlength="8" required autocomplete="new-password" /></label>
          <label>Nuke password<input id="nuke-password" type="password" minlength="8" required autocomplete="new-password" /></label>
          <button class="primary" type="submit">CREATE &amp; ARM VAULT</button>
        </form>
        <div class="rules">
          <div><strong>10 s</strong><span>no input</span></div>
          <div><strong>NUKE</strong><span>duress credential</span></div>
          <div><strong>3×</strong><span>incorrect attempts</span></div>
        </div>
      </div>`);
    document.querySelector<HTMLFormElement>("#create-form")!.addEventListener("submit", createVault);
}

async function createVault(event: SubmitEvent) {
    event.preventDefault();
    const button = document.querySelector<HTMLButtonElement>("#create-form button")!;
    const nameInput = document.querySelector<HTMLInputElement>("#vault-name")!;
    const unlockInput = document.querySelector<HTMLInputElement>("#unlock-password")!;
    const nukeInput = document.querySelector<HTMLInputElement>("#nuke-password")!;
    button.disabled = true;
    button.textContent = "DERIVING KEYS…";
    const unlockPassword = unlockInput.value;
    const nukePassword = nukeInput.value;
    unlockInput.value = "";
    nukeInput.value = "";
    try {
        await VaultService.CreateVault(nameInput.value, unlockPassword, nukePassword);
        await boot();
    } catch (error) {
        renderSetup(errorMessage(error));
    }
}

function renderChallenge(state: UnlockState) {
    stopCountdown();
    const vault = selectedVault!;
    app.innerHTML = shell(`
      <div class="panel challenge-panel">
        <div class="status-line danger"><span class="status-dot pulse"></span>DESTRUCTIVE CHALLENGE ARMED</div>
        <div class="challenge-grid">
          <div>
            <p class="label">TARGET</p>
            <h2>${escapeHTML(vault.name)}</h2>
            <p class="mono">${escapeHTML(vault.id)}</p>
          </div>
          <div class="clock" id="clock"><span id="seconds">10.0</span><small>SECONDS</small></div>
        </div>
        <div class="attempt-track" aria-label="Failed attempts">
          ${[0, 1, 2].map((index) => `<i class="${index < state.attempts ? "spent" : ""}"></i>`).join("")}
          <span>${state.attempts}/3 FAILED</span>
        </div>
        <p id="message" class="message">${escapeHTML(state.message)}</p>
        <form id="unlock-form" class="unlock-row" autocomplete="off">
          <input id="password" type="password" placeholder="Enter unlock or Nuke password" autofocus required autocomplete="off" />
          <button class="primary" type="submit">SUBMIT</button>
        </form>
        <p class="warning">Timeout, the Nuke credential, or a third failure destroys key material and starts a random + zero overwrite.</p>
      </div>`, "armed");
    document.querySelector<HTMLFormElement>("#unlock-form")!.addEventListener("submit", submitPassword);
    const input = document.querySelector<HTMLInputElement>("#password")!;
    window.setTimeout(() => input.focus(), 50);
    startCountdown(state.deadline);
}

function startCountdown(deadline: number) {
    const update = () => {
        const remaining = Math.max(0, deadline - Date.now());
        const seconds = document.querySelector<HTMLElement>("#seconds");
        if (seconds) seconds.textContent = (remaining / 1000).toFixed(1);
        if (remaining === 0) {
            stopCountdown();
            const form = document.querySelector<HTMLFormElement>("#unlock-form");
            if (form) form.classList.add("disabled");
        }
    };
    update();
    countdownTimer = window.setInterval(update, 50);
}

async function submitPassword(event: SubmitEvent) {
    event.preventDefault();
    stopCountdown();
    const input = document.querySelector<HTMLInputElement>("#password")!;
    const button = document.querySelector<HTMLButtonElement>("#unlock-form button")!;
    const password = input.value;
    input.value = "";
    input.disabled = true;
    button.disabled = true;
    button.textContent = "VERIFYING…";
    try {
        const state = await VaultService.SubmitPassword(selectedVault!.id, password) as UnlockState;
        handleState(state);
    } catch (error) {
        renderFatal(errorMessage(error));
    }
}

function handleState(state: UnlockState) {
    if (state.status === "armed") {
        renderChallenge(state);
    } else if (state.status === "mounted") {
        renderMounted(state);
    } else if (state.status === "nuking" || state.status === "wiping") {
		if (destructionState === "wiping" || destructionState === "destroyed") return;
		destructionState = "nuking";
        renderWipe({vaultId: state.vaultId, status: state.status, message: state.message, attempts: state.attempts, deadline: 0, phase: "destroying-keys", percent: 0});
    }
}

function renderMounted(state: UnlockState) {
    stopCountdown();
    const mount = state.mount || {url: "", mountPoint: "", automatic: false, message: ""};
    app.innerHTML = shell(`
      <div class="panel mounted-panel">
        <div class="status-line"><span class="status-dot safe pulse-soft"></span>VAULT UNLOCKED</div>
        <h2>${escapeHTML(selectedVault!.name)}</h2>
        <p class="lede">The decrypted view exists only while this session and local WebDAV server are running.</p>
        <dl class="mount-details">
          <div><dt>Automatic mount</dt><dd>${mount.automatic ? "YES" : "BLOCKED / FALLBACK"}</dd></div>
          <div><dt>Mount point</dt><dd class="mono">${escapeHTML(mount.mountPoint || "Local WebDAV URL only")}</dd></div>
          <div><dt>Session URL</dt><dd class="mono private">${escapeHTML(mount.url)}</dd></div>
        </dl>
        <p class="message">${escapeHTML(state.message || mount.message)}</p>
        <div class="button-row">
          <button id="reveal" class="primary">REVEAL FILES</button>
          <button id="lock" class="secondary">LOCK VAULT</button>
          <button id="destroy" class="destructive">DESTROY VAULT</button>
        </div>
      </div>`, "unlocked");
    document.querySelector<HTMLButtonElement>("#reveal")!.addEventListener("click", async () => {
        try { await VaultService.RevealVault(selectedVault!.id); } catch (error) { showInline(errorMessage(error)); }
    });
    document.querySelector<HTMLButtonElement>("#lock")!.addEventListener("click", async () => {
        try { await VaultService.LockVault(selectedVault!.id); await boot(); } catch (error) { showInline(errorMessage(error)); }
    });
    document.querySelector<HTMLButtonElement>("#destroy")!.addEventListener("click", async () => {
        const vault = selectedVault!;
        if (!window.confirm(`Permanently destroy ${vault.name}? This cannot be undone.`)) return;
        const phrase = `DELETE ${vault.name}`;
        if (window.prompt(`Type ${phrase} to confirm:`) !== phrase) {
            showInline("Destruction cancelled: confirmation phrase did not match.");
            return;
        }
        document.querySelectorAll<HTMLButtonElement>(".button-row button").forEach((button) => button.disabled = true);
        try {
            const next = await VaultService.DestroyVault(vault.id) as UnlockState;
            handleState(next);
        } catch (error) {
            showInline(errorMessage(error));
            document.querySelectorAll<HTMLButtonElement>(".button-row button").forEach((button) => button.disabled = false);
        }
    });
}

function renderWipe(event: VaultEvent) {
	if (destructionState === "destroyed") return;
	destructionState = "wiping";
    stopCountdown();
    const percent = Math.max(0, Math.min(100, event.percent || 0));
    app.innerHTML = shell(`
      <div class="panel wipe-panel">
        <div class="status-line danger"><span class="status-dot pulse"></span>NUKE SEQUENCE ACTIVE</div>
        <div class="nuke-symbol">×</div>
        <h2>${escapeHTML((event.phase || "destroying-keys").replaceAll("-", " "))}</h2>
        <p class="lede">${escapeHTML(event.message || "Destructive operation in progress.")}</p>
        <div class="progress"><i style="width:${percent}%"></i></div>
        <p class="mono progress-label">${percent.toFixed(1)}%</p>
        <p class="warning">Once key destruction begins, this operation cannot be reversed.</p>
      </div>`, "nuking");
}

function renderDestroyed(message: string) {
	destructionState = "destroyed";
    stopCountdown();
    app.innerHTML = shell(`
      <div class="panel destroyed-panel">
        <div class="status-line danger"><span class="status-dot"></span>CONTAINER DESTROYED</div>
        <div class="nuke-symbol done">✓</div>
        <h2>Lab wipe complete</h2>
        <p class="lede">${escapeHTML(message)}</p>
        <button id="reset" class="primary">CREATE ANOTHER LAB VAULT</button>
      </div>`, "destroyed");
    document.querySelector<HTMLButtonElement>("#reset")!.addEventListener("click", () => boot());
}

function renderFatal(message: string) {
    stopCountdown();
    app.innerHTML = shell(`
      <div class="panel"><div class="status-line danger"><span class="status-dot"></span>ERROR</div>
      <h2>Operation failed</h2><p class="alert">${escapeHTML(message)}</p>
      <button id="retry" class="secondary">RELOAD STATE</button></div>`);
    document.querySelector<HTMLButtonElement>("#retry")!.addEventListener("click", () => boot());
}

function showInline(message: string) {
    const target = document.querySelector<HTMLElement>(".message");
    if (target) target.textContent = message;
}

async function boot() {
    stopCountdown();
	destructionState = "none";
    app.innerHTML = shell(`<div class="panel loading"><div class="spinner"></div><p>Inspecting managed vault directory…</p></div>`);
    try {
        const vaults = await VaultService.ListVaults() as VaultInfo[];
        if (!vaults.length) {
            renderSetup();
            return;
        }
        selectedVault = vaults[0];
        const state = await VaultService.ArmVault(selectedVault.id) as UnlockState;
        handleState(state);
    } catch (error) {
        renderFatal(errorMessage(error));
    }
}

function errorMessage(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
}

function escapeHTML(value: unknown): string {
    return String(value ?? "").replace(/[&<>'"]/g, (character) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;"
    })[character]!);
}

Events.On("vault:state", (incoming) => {
    const event = incoming.data as VaultEvent;
    if (selectedVault && event.vaultId !== selectedVault.id) return;
    if (event.status === "wiping") renderWipe(event);
    else if (event.status === "destroyed") renderDestroyed(event.message);
    else if (event.status === "error") renderFatal(event.message);
});

void boot();
