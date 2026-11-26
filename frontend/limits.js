// The limits and UI settings of config.yaml, read from GET /api/config/limits, so
// labels and client-side checks always match what the server enforces. Until
// they arrive (or if the request fails) the dependent checks are skipped and the
// server remains the judge.
const AppLimits = (() => {
  let current = null;
  let pending = null;
  const listeners = [];

  // protojson sends int64 values as strings.
  function normalize(data) {
    const limits = { ...data };
    [
      "minQuotaBytes",
      "maxQuotaBytes",
      "defaultQuotaBytes",
      "maxUploadBytes",
    ].forEach((key) => {
      limits[key] = Number(data[key]);
    });
    limits.allowedFileExtensions = data.allowedFileExtensions || [];
    limits.adminQuotaAdjustStepsBytes = (data.adminQuotaAdjustStepsBytes || []).map(Number);
    return limits;
  }

  async function fetchLimits() {
    try {
      const { response } = await fetchWithFallback("/api/config/limits", {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) return null;
      current = normalize(await response.json());
      Validation.configure(current);
      listeners.forEach((listener) => listener(current));
      return current;
    } catch (err) {
      console.warn("Could not load the configured limits:", err);
      return null;
    }
  }

  // Resolves to the limits, or to null. A failed attempt is retried next time.
  function load() {
    if (current) return Promise.resolve(current);
    if (!pending) {
      pending = fetchLimits().finally(() => {
        pending = null;
      });
    }
    return pending;
  }

  return {
    load,
    onLoad(listener) {
      listeners.push(listener);
    },
    get current() {
      return current;
    },
  };
})();

const SIZE_UNITS = { MB: 1024 ** 2, GB: 1024 ** 3, TB: 1024 ** 4 };

// The largest unit that shows the size without a long fraction.
function bestSizeUnit(bytes) {
  return (
    ["TB", "GB"].find((unit) => {
      const value = bytes / SIZE_UNITS[unit];
      return value >= 1 && Number.isInteger(value * 100);
    }) || "MB"
  );
}

function sizeInUnit(bytes, unit) {
  return Number((bytes / SIZE_UNITS[unit]).toFixed(2));
}

// Reads a number input and its unit <select>. Returns null if there is no number.
function readSizeInput(input, unitSelect) {
  const text = input.value.trim();
  const value = Number(text);
  if (!text || !Number.isFinite(value)) return null;
  return Math.round(value * SIZE_UNITS[unitSelect.value]);
}

function quotaRangeMessage(limits) {
  return `Storage quota must be between ${formatBytes(limits.minQuotaBytes)} and ${formatBytes(limits.maxQuotaBytes)}`;
}

// storage_warning_percent: from here a storage bar turns red.
function isNearlyFull(percent) {
  const threshold = AppLimits.current?.storageWarningPercent;
  return Boolean(threshold) && percent >= threshold;
}

// The "open the inbox" button shown after an email was sent, or null when
// mail_ui_url is empty.
function mailInboxAction() {
  const url = AppLimits.current?.mailUiUrl;
  return url
    ? `<a href="${Validation.escapeHtml(url)}" target="_blank" rel="noopener noreferrer" class="btn-mailpit">
        Open Mailpit Inbox ↗
      </a>`
    : null;
}

// One JSON call to the API; an authenticated one when a token is given.
async function apiJson(path, { method = "GET", body, token } = {}) {
  const headers = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (token) headers.Authorization = `Bearer ${token}`;

  const { response } = await fetchWithFallback(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  const text = await response.text();
  let data = {};
  try {
    data = JSON.parse(text);
  } catch {
    data = { message: text.trim() };
  }
  return { ok: response.ok, status: response.status, data };
}
