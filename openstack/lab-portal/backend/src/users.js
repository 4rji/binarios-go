import { randomUUID } from "node:crypto";

const users = [
  { id: "user-havi", username: "havi", password: "metro123", role: "student" },
  { id: "user-connor", username: "connor", password: "metro123", role: "student" }
];

const sessions = new Map();

export function publicUser(user) {
  const { password: _password, ...safeUser } = user;
  return safeUser;
}

export function getPublicUsers() {
  return users.map(publicUser);
}

export function authenticateUser(username, password) {
  const normalizedUsername = String(username || "").trim().toLowerCase();
  const candidate = users.find((user) => user.username === normalizedUsername);

  if (!candidate || candidate.password !== String(password || "")) {
    return null;
  }

  return candidate;
}

export function createSession(user) {
  const token = randomUUID();
  sessions.set(token, user);
  return token;
}

export function getSessionUser(token) {
  return sessions.get(token) || null;
}

export function deleteSession(token) {
  sessions.delete(token);
}

export function readAuthToken(req) {
  const authorization = req.header("authorization") || "";
  if (authorization.toLowerCase().startsWith("bearer ")) {
    return authorization.slice(7).trim();
  }

  return req.header("x-lab-token") || "";
}
