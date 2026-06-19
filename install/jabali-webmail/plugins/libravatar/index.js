var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/index.js
var index_exports = {};
__export(index_exports, {
  activate: () => activate,
  hooks: () => hooks
});
module.exports = __toCommonJS(index_exports);
var CACHE_KEY = "cache.v1";
var CACHE_TTL_HIT_MS = 7 * 24 * 60 * 60 * 1e3;
var CACHE_TTL_MISS_MS = 24 * 60 * 60 * 1e3;
var HEAD_TIMEOUT_MS = 3e3;
var MAX_CACHE_ENTRIES = 500;
var VALID_DEFAULTS = /* @__PURE__ */ new Set([
  "404",
  "mm",
  "identicon",
  "monsterid",
  "wavatar",
  "retro",
  "robohash",
  "pagan"
]);
function normalizeEmail(value) {
  if (typeof value !== "string") return null;
  const trimmed = value.trim().toLowerCase();
  const at = trimmed.indexOf("@");
  if (at <= 0 || at === trimmed.length - 1) return null;
  return trimmed;
}
async function sha256Hex(input) {
  const bytes = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  const view = new Uint8Array(digest);
  let out = "";
  for (let i = 0; i < view.length; i++) {
    out += view[i].toString(16).padStart(2, "0");
  }
  return out;
}
function buildAvatarUrl(hash, { size, defaultStyle }) {
  const params = new URLSearchParams({
    s: String(size),
    d: defaultStyle
  });
  return `https://seccdn.libravatar.org/avatar/${hash}?${params.toString()}`;
}
function buildExistenceUrl(hash) {
  return `https://seccdn.libravatar.org/avatar/${hash}?s=1&d=404`;
}
async function libravatarExists(hash) {
  try {
    const res = await fetch(buildExistenceUrl(hash), {
      method: "HEAD",
      signal: AbortSignal.timeout(HEAD_TIMEOUT_MS)
    });
    return res.ok;
  } catch {
    return false;
  }
}
function readSettings(raw) {
  const rawSize = typeof raw.size === "number" ? raw.size : 160;
  const size = Math.min(512, Math.max(16, Math.round(rawSize)));
  const defaultStyle = VALID_DEFAULTS.has(raw.defaultStyle) ? raw.defaultStyle : "404";
  return { size, defaultStyle };
}
function loadCacheFromStored(stored) {
  const cache2 = /* @__PURE__ */ new Map();
  if (!stored || typeof stored !== "object") return cache2;
  const now = Date.now();
  for (const [email, entry] of Object.entries(stored)) {
    if (entry && typeof entry === "object" && typeof entry.expires === "number" && entry.expires > now && (entry.url === null || typeof entry.url === "string")) {
      cache2.set(email, entry);
    }
  }
  return cache2;
}
function trimCache(cache2) {
  if (cache2.size <= MAX_CACHE_ENTRIES) return;
  const sorted = [...cache2.entries()].sort((a, b) => a[1].expires - b[1].expires);
  const dropCount = sorted.length - MAX_CACHE_ENTRIES;
  for (let i = 0; i < dropCount; i++) cache2.delete(sorted[i][0]);
}
var pluginApi = null;
var settings = readSettings({});
var cache = /* @__PURE__ */ new Map();
var inFlight = /* @__PURE__ */ new Map();
function rememberAndPersist(email, url) {
  const ttl = url ? CACHE_TTL_HIT_MS : CACHE_TTL_MISS_MS;
  cache.set(email, { url, expires: Date.now() + ttl });
  trimCache(cache);
  void pluginApi.storage.set(CACHE_KEY, Object.fromEntries(cache));
}
async function resolveFor(email) {
  const cached = cache.get(email);
  if (cached && cached.expires > Date.now()) {
    return cached.url;
  }
  const existing = inFlight.get(email);
  if (existing) return existing;
  const promise = (async () => {
    try {
      const hash = await sha256Hex(email);
      const url = buildAvatarUrl(hash, settings);
      const resolved = settings.defaultStyle === "404" ? await libravatarExists(hash) ? url : null : url;
      rememberAndPersist(email, resolved);
      return resolved;
    } finally {
      inFlight.delete(email);
    }
  })();
  inFlight.set(email, promise);
  return promise;
}
async function activate(api) {
  pluginApi = api;
  settings = readSettings(api.plugin.settings || {});
  const stored = await api.storage.get(CACHE_KEY);
  const loaded = loadCacheFromStored(stored);
  cache.clear();
  for (const [k, v] of loaded) cache.set(k, v);
  api.log.info(
    `Libravatar activated (size=${settings.size}, default=${settings.defaultStyle}, cached=${cache.size})`
  );
}
var hooks = {
  async onAvatarResolve(currentUrl, ctx) {
    if (currentUrl) return void 0;
    const email = normalizeEmail(ctx?.email);
    if (!email) return void 0;
    const url = await resolveFor(email);
    return url ?? void 0;
  }
};
