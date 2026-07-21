const DEFAULT_MAX_ATTEMPTS = 5;
const DEFAULT_LOCKOUT_MS = 5 * 60 * 1000;
const DEFAULT_WINDOW_MS = 10 * 60 * 1000;

const loginAttempts = new Map();

function readPositiveInt(value, fallback) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export function readLoginRateLimitConfig(env = process.env) {
  return {
    maxAttempts: readPositiveInt(env.LOGIN_MAX_FAILED_ATTEMPTS, DEFAULT_MAX_ATTEMPTS),
    lockoutMs: readPositiveInt(env.LOGIN_LOCKOUT_MS, DEFAULT_LOCKOUT_MS),
    windowMs: readPositiveInt(env.LOGIN_ATTEMPT_WINDOW_MS, DEFAULT_WINDOW_MS)
  };
}

export function normalizeLoginSubject(username) {
  return String(username || "").trim().toLowerCase() || "unknown";
}

export function loginRateLimitKey({ ip, username }) {
  return `${String(ip || "unknown")}:${normalizeLoginSubject(username)}`;
}

export function getLoginRateLimitStatus(key, options = {}) {
  const now = options.now ?? Date.now();
  const entry = loginAttempts.get(key);

  if (!entry) {
    return { limited: false };
  }

  if (entry.lockedUntil && entry.lockedUntil > now) {
    return {
      limited: true,
      retryAfterSeconds: Math.ceil((entry.lockedUntil - now) / 1000)
    };
  }

  if (entry.lockedUntil || entry.firstAttemptAt + options.windowMs <= now) {
    loginAttempts.delete(key);
  }

  return { limited: false };
}

export function recordFailedLogin(key, options = {}) {
  const now = options.now ?? Date.now();
  const config = {
    ...readLoginRateLimitConfig(),
    ...options
  };
  const current = loginAttempts.get(key);
  const entry = current && current.firstAttemptAt + config.windowMs > now
    ? current
    : { count: 0, firstAttemptAt: now, lockedUntil: 0 };

  entry.count += 1;

  if (entry.count >= config.maxAttempts) {
    entry.lockedUntil = now + config.lockoutMs;
  }

  loginAttempts.set(key, entry);

  return getLoginRateLimitStatus(key, { ...config, now });
}

export function clearLoginFailures(key) {
  loginAttempts.delete(key);
}

export function resetLoginRateLimits() {
  loginAttempts.clear();
}
