// One place that knows how to talk to the backend: base URL, auth header,
// and error shape. Components never touch fetch directly.

const BASE = (import.meta.env.VITE_API_BASE || "").replace(/\/$/, "");
const TOKEN_KEY = "lp_token";

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
};

export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

async function request(path, { method = "GET", body, auth = true } = {}) {
  const headers = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const token = tokenStore.get();
  if (auth && token) headers.Authorization = `Bearer ${token}`;

  let res;
  try {
    res = await fetch(`${BASE}/api${path}`, {
      method,
      headers,
      credentials: "include", // carries the anonymous voter cookie
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError("Can't reach the server. Check your connection.", 0);
  }

  if (res.status === 204) return null;

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401 && auth) tokenStore.clear();
    throw new ApiError(data.error || "Something went wrong.", res.status);
  }
  return data;
}

export const api = {
  signup: (payload) => request("/auth/signup", { method: "POST", body: payload, auth: false }),
  login: (payload) => request("/auth/login", { method: "POST", body: payload, auth: false }),
  me: () => request("/auth/me"),

  createPoll: (payload) => request("/polls", { method: "POST", body: payload }),
  myPolls: () => request("/polls"),
  getPoll: (slug) => request(`/polls/${encodeURIComponent(slug)}`),
  vote: (slug, optionIds) =>
    request(`/polls/${encodeURIComponent(slug)}/vote`, { method: "POST", body: { optionIds } }),
  setStatus: (slug, status) =>
    request(`/polls/${encodeURIComponent(slug)}/status`, { method: "PATCH", body: { status } }),
  deletePoll: (slug) => request(`/polls/${encodeURIComponent(slug)}`, { method: "DELETE" }),
};

export const streamUrl = (slug) => `${BASE}/api/polls/${encodeURIComponent(slug)}/stream`;
