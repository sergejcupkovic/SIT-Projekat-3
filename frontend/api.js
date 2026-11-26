// Every API call goes through here. Resolves to { ok, status, data }.
async function apiJson(path, { method = "GET", body, token } = {}) {
  const headers = { Accept: "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  // for a file upload the browser sets the multipart Content-Type itself
  const isUpload = body instanceof FormData;
  if (body !== undefined && !isUpload) headers["Content-Type"] = "application/json";

  const response = await fetch(path, {
    method,
    headers,
    body: body === undefined || isUpload ? body : JSON.stringify(body),
  });
  if (response.status === 429) {
    showAlert("warning", "Too Many Requests", "You are sending requests too fast. Please slow down and try again.");
  }

  const text = await response.text();
  let data = {};
  try {
    data = JSON.parse(text);
  } catch {
    data = { message: text.trim() };
  }
  return { ok: response.ok, status: response.status, data };
}

// Quota bounds and password length from config.yaml; null until loaded.
let appLimits = null;

async function loadLimits() {
  if (appLimits) return appLimits;
  try {
    const { ok, data } = await apiJson("/api/config/limits");
    if (ok) {
      // int64 values arrive as strings
      appLimits = {
        minQuotaBytes: Number(data.minQuotaBytes),
        maxQuotaBytes: Number(data.maxQuotaBytes),
        defaultQuotaBytes: Number(data.defaultQuotaBytes),
        passwordMinLength: data.passwordMinLength,
      };
    }
  } catch (err) {
    console.warn("Could not load the limits:", err);
  }
  return appLimits;
}

const BYTES_PER_MB = 1024 * 1024;
const toMb = (bytes) => Number((Number(bytes) / BYTES_PER_MB).toFixed(2));

function quotaRangeProblem(bytes, limits) {
  if (bytes >= limits.minQuotaBytes && bytes <= limits.maxQuotaBytes) return "";
  return `Storage quota must be between ${formatBytes(limits.minQuotaBytes)} and ${formatBytes(limits.maxQuotaBytes)}`;
}

function quotaRangeHint(limits) {
  return `Allowed: ${toMb(limits.minQuotaBytes)} MB to ${toMb(limits.maxQuotaBytes)} MB (${formatBytes(limits.maxQuotaBytes)}).`;
}
