const defaultAllowedOrigins = [
  "http://localhost",
  "http://127.0.0.1",
  "https://localhost",
  "https://127.0.0.1",
  "http://localhost:5173",
  "http://127.0.0.1:5173"
];

function isPrivateIpv4(hostname) {
  const parts = hostname.split(".").map((part) => Number(part));
  if (parts.length !== 4 || parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) {
    return false;
  }

  const [first, second] = parts;
  return first === 10
    || (first === 172 && second >= 16 && second <= 31)
    || (first === 192 && second === 168);
}

export function parseAllowedOrigins(value) {
  return (value || defaultAllowedOrigins.join(","))
    .split(",")
    .map((origin) => origin.trim())
    .filter(Boolean);
}

export function isAllowedCorsOrigin(origin, {
  allowedOrigins = defaultAllowedOrigins,
  nodeEnv = process.env.NODE_ENV
} = {}) {
  if (!origin || allowedOrigins.includes("*") || allowedOrigins.includes(origin)) {
    return true;
  }

  if (nodeEnv === "production") {
    return false;
  }

  try {
    const parsedOrigin = new URL(origin);
    const isLocalHost = ["localhost", "127.0.0.1"].includes(parsedOrigin.hostname);
    const isDevHost = isLocalHost || isPrivateIpv4(parsedOrigin.hostname);
    if (!isDevHost) return false;

    if (parsedOrigin.protocol === "http:") {
      return ["", "80", "5173"].includes(parsedOrigin.port);
    }

    if (parsedOrigin.protocol === "https:") {
      return ["", "443"].includes(parsedOrigin.port);
    }

    return false;
  } catch {
    return false;
  }
}
