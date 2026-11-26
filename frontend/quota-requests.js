// Storage quota increase requests: a regular user submits one from the storage
// view, an administrator approves or rejects it in the "Quota Requests" tab.

const QUOTA_STATUS_LABELS = {
  pending: "Pending",
  approved: "Approved",
  rejected: "Rejected",
};

const formatDateTime = (value) => (value ? new Date(value).toLocaleString() : "");

function quotaStatusPill(status) {
  const pill = document.createElement("span");
  pill.className = `status-pill request-${QUOTA_STATUS_LABELS[status] ? status : "pending"}`;
  pill.textContent = QUOTA_STATUS_LABELS[status] || status;
  return pill;
}

function textElement(tag, className, text) {
  const el = document.createElement(tag);
  el.className = className;
  el.textContent = text;
  return el;
}

// #region regular user

const quotaRequestOpenBtn = document.getElementById("quotaRequestOpenBtn");
const quotaRequestPanel = document.getElementById("quotaRequestPanel");
const quotaRequestSummary = document.getElementById("quotaRequestSummary");
const quotaRequestNotice = document.getElementById("quotaRequestNotice");
const quotaRequestForm = document.getElementById("quotaRequestForm");
const quotaRequestInput = document.getElementById("quotaRequestInput");
const quotaRequestUnit = document.getElementById("quotaRequestUnit");
const quotaRequestHint = document.getElementById("quotaRequestHint");
const quotaRequestError = document.getElementById("quotaRequestError");
const quotaRequestSubmitBtn = document.getElementById("quotaRequestSubmitBtn");
const quotaRequestCloseBtn = document.getElementById("quotaRequestCloseBtn");
const quotaRequestHistory = document.getElementById("quotaRequestHistory");

let ownQuotaBytes = 0;
// Bumped whenever the panel is closed, so a late answer cannot repaint it.
let quotaPanelGeneration = 0;

function closeQuotaRequestPanel() {
  quotaPanelGeneration++;
  quotaRequestPanel.classList.add("hidden");
  quotaRequestForm.classList.add("hidden");
  quotaRequestForm.reset();
  setFieldError(quotaRequestInput, quotaRequestError, "");
  quotaRequestNotice.textContent = "";
  quotaRequestHistory.innerHTML = "";
}

async function openQuotaRequestPanel() {
  hideAlert();
  closeQuotaRequestPanel();
  const generation = quotaPanelGeneration;
  quotaRequestPanel.classList.remove("hidden");
  quotaRequestSummary.textContent = "Loading…";

  try {
    const token = session.token;
    const [limits, quota, mine] = await Promise.all([
      AppLimits.load(),
      apiJson("/api/files/quota", { token }),
      apiJson("/api/users/quota-requests", { token }),
    ]);
    if (generation !== quotaPanelGeneration) return;

    if (quota.status === 401 || mine.status === 401) {
      handleSessionExpired();
      return;
    }
    if (!quota.ok || !mine.ok) {
      quotaRequestSummary.textContent =
        (!mine.ok ? mine.data.message : quota.data.message) ||
        "Could not load your storage details. Try again.";
      return;
    }

    updateQuotaDisplay(quota.data.usedStorageBytes, quota.data.storageQuotaBytes);
    ownQuotaBytes = Number(quota.data.storageQuotaBytes) || 0;
    renderOwnQuotaRequests(mine.data.requests || [], limits);
  } catch {
    if (generation !== quotaPanelGeneration) return;
    quotaRequestSummary.textContent = "Could not reach the server. Try again.";
  }
}

function renderOwnQuotaRequests(requests, limits) {
  quotaRequestSummary.textContent = `Current quota: ${formatBytes(ownQuotaBytes)}${
    limits ? ` • Maximum: ${formatBytes(limits.maxQuotaBytes)}` : ""
  }`;

  const pending = requests.find((request) => request.status === "pending");
  const atMaximum = limits && ownQuotaBytes >= limits.maxQuotaBytes;

  if (pending) {
    quotaRequestNotice.textContent =
      `Your request for ${formatBytes(pending.requestedQuotaBytes)}, sent ` +
      `${formatDateTime(pending.createdAt)}, is waiting for an administrator. ` +
      "You can send another one after it is decided.";
  } else if (atMaximum) {
    quotaRequestNotice.textContent = `You already have the maximum quota of ${formatBytes(limits.maxQuotaBytes)}.`;
  } else {
    quotaRequestNotice.textContent = "";
    quotaRequestForm.classList.remove("hidden");
    // Suggest quota_request_suggestion_factor times the current size, within the
    // configured maximum.
    if (limits) {
      const suggestion = Math.min(
        Math.round(ownQuotaBytes * limits.quotaRequestSuggestionFactor),
        limits.maxQuotaBytes,
      );
      quotaRequestUnit.value = bestSizeUnit(suggestion);
      quotaRequestInput.value = sizeInUnit(suggestion, quotaRequestUnit.value);
    }
    quotaRequestHint.textContent = limits
      ? `Choose more than ${formatBytes(ownQuotaBytes)}, up to ${formatBytes(limits.maxQuotaBytes)}.`
      : `Choose more than ${formatBytes(ownQuotaBytes)}.`;
    quotaRequestInput.focus();
  }

  quotaRequestHistory.innerHTML = "";
  // quota_request_history_items
  const history = limits ? requests.slice(0, limits.quotaRequestHistoryItems) : requests;
  history.forEach((request) => {
    const item = document.createElement("li");
    item.className = "quota-history-item";
    item.append(
      textElement(
        "span",
        "quota-history-text",
        `${formatBytes(request.requestedQuotaBytes)} • ${formatDateTime(request.createdAt)}`,
      ),
      quotaStatusPill(request.status),
    );
    quotaRequestHistory.appendChild(item);
  });
}

// "" when bytes can be requested.
function quotaRequestProblem(bytes, limits) {
  if (quotaRequestInput.validity.badInput || bytes === null) return "Enter the quota you need";
  if (limits && (bytes > limits.maxQuotaBytes || bytes < limits.minQuotaBytes))
    return quotaRangeMessage(limits);
  if (bytes <= ownQuotaBytes)
    return `Requested quota must be greater than your current quota of ${formatBytes(ownQuotaBytes)}`;
  return "";
}

async function handleQuotaRequestSubmit(event) {
  event.preventDefault();
  hideAlert();

  const limits = await AppLimits.load();
  const bytes = readSizeInput(quotaRequestInput, quotaRequestUnit);
  const problem = quotaRequestProblem(bytes, limits);
  setFieldError(quotaRequestInput, quotaRequestError, problem);
  if (problem) return;

  setButtonBusy(quotaRequestSubmitBtn, true);
  try {
    const { status, data } = await apiJson("/api/users/quota-requests", {
      method: "POST",
      token: session.token,
      body: { requestedQuotaBytes: bytes },
    });
    if (status === 201) {
      await openQuotaRequestPanel();
      showAlert(
        "success",
        "Request Sent",
        `Your request for ${formatBytes(bytes)} was sent to an administrator.`,
      );
    } else if (status === 401) {
      handleSessionExpired();
    } else if (status === 409) {
      // Another request is already pending (e.g. sent from another tab).
      await openQuotaRequestPanel();
      showAlert("warning", "Request Already Pending", data.message);
    } else if (status !== 429) {
      setFieldError(quotaRequestInput, quotaRequestError, data.message || "Could not send the request.");
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not reach the server. Your request was not sent.");
  } finally {
    setButtonBusy(quotaRequestSubmitBtn, false);
  }
}

// #endregion

// #region administrator

const adminQuotaRequestsCount = document.getElementById("adminQuotaRequestsCount");
const adminQuotaRequestFilter = document.getElementById("adminQuotaRequestFilter");
const adminRefreshQuotaRequestsBtn = document.getElementById("adminRefreshQuotaRequestsBtn");
const adminQuotaRequestsLoading = document.getElementById("adminQuotaRequestsLoading");
const adminQuotaRequestsEmpty = document.getElementById("adminQuotaRequestsEmpty");
const adminQuotaRequestsList = document.getElementById("adminQuotaRequestsList");

// Only the answer to the newest load is shown (the filter can change quickly).
let adminQuotaRequestsLoad = 0;

function showPendingCount(count) {
  adminQuotaRequestsCount.textContent = String(count);
  adminQuotaRequestsCount.classList.toggle("hidden", count === 0);
}

async function refreshPendingQuotaRequestCount() {
  if (!canOpenAdminPanel()) return;
  try {
    const { ok, data } = await apiJson("/api/admin/quota-requests?status=pending", {
      token: session.token,
    });
    if (ok) showPendingCount((data.requests || []).length);
  } catch (err) {
    console.warn("Could not count pending quota requests:", err);
  }
}

function clearAdminQuotaRequests() {
  adminQuotaRequestsLoad++;
  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestFilter.value = "pending";
  showPendingCount(0);
}

async function loadAdminQuotaRequests() {
  if (!canOpenAdminPanel()) return;
  const load = ++adminQuotaRequestsLoad;
  const filter = adminQuotaRequestFilter.value;

  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestsEmpty.classList.add("hidden");
  adminQuotaRequestsLoading.classList.remove("hidden");

  try {
    const query = filter ? `?status=${encodeURIComponent(filter)}` : "";
    const { ok, status, data } = await apiJson(`/api/admin/quota-requests${query}`, {
      token: session.token,
    });
    if (load !== adminQuotaRequestsLoad) return;
    if (status === 401) {
      handleSessionExpired();
      return;
    }
    if (!ok) {
      if (status !== 429) {
        showAlert("danger", "Error Loading Requests", data.message || "Could not load quota requests.");
      }
      return;
    }

    const requests = data.requests || [];
    if (filter === "pending") showPendingCount(requests.length);
    else if (filter === "") showPendingCount(requests.filter((r) => r.status === "pending").length);
    renderAdminQuotaRequests(requests);
  } catch {
    if (load === adminQuotaRequestsLoad) {
      showAlert("danger", "Connection Error", "Could not load quota requests.");
    }
  } finally {
    if (load === adminQuotaRequestsLoad) adminQuotaRequestsLoading.classList.add("hidden");
  }
}

function renderAdminQuotaRequests(requests) {
  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestsEmpty.classList.toggle("hidden", requests.length > 0);

  requests.forEach((request) => {
    const fullName =
      `${request.firstName || ""} ${request.lastName || ""}`.trim() || "Unknown User";
    const isPending = request.status === "pending";

    const row = document.createElement("div");
    row.className = "admin-user-row";

    const info = document.createElement("div");
    info.className = "admin-user-info";
    const nameLine = document.createElement("div");
    nameLine.className = "admin-user-name-line";
    nameLine.append(textElement("span", "admin-user-name", fullName), quotaStatusPill(request.status));
    info.append(
      nameLine,
      textElement("div", "admin-user-email", request.email || "Account no longer exists"),
      textElement("div", "quota-request-time", `Requested ${formatDateTime(request.createdAt)}`),
    );
    if (!isPending && request.resolvedAt) {
      info.append(
        textElement(
          "div",
          "quota-request-time",
          `${QUOTA_STATUS_LABELS[request.status]} ${formatDateTime(request.resolvedAt)}`,
        ),
      );
    }

    const sizes = document.createElement("div");
    sizes.className = "admin-user-storage";
    sizes.append(
      textElement(
        "div",
        "admin-storage-text",
        `Current: ${formatBytes(request.currentQuotaBytes)} (${formatBytes(request.usedStorageBytes)} used)`,
      ),
      textElement(
        "div",
        "admin-storage-text quota-request-size",
        `Requested: ${formatBytes(request.requestedQuotaBytes)}`,
      ),
    );
    if (!isPending) {
      sizes.append(
        textElement("div", "quota-request-time", `Quota when requested: ${formatBytes(request.quotaAtRequestBytes)}`),
      );
    }

    row.append(info, sizes);

    if (isPending) {
      const actions = document.createElement("div");
      actions.className = "admin-user-actions";
      const approve = textElement("button", "btn-primary-sm approve-request-btn", "Approve");
      const reject = textElement("button", "btn-danger-sm reject-request-btn", "Reject");
      [approve, reject].forEach((button) => {
        button.type = "button";
      });
      approve.addEventListener("click", () => decideQuotaRequest(request, "approve", [approve, reject]));
      reject.addEventListener("click", () => decideQuotaRequest(request, "reject", [approve, reject]));
      actions.append(approve, reject);
      row.append(actions);
    }

    adminQuotaRequestsList.appendChild(row);
  });
}

// action is "approve" or "reject".
async function decideQuotaRequest(request, action, buttons) {
  hideAlert();
  buttons.forEach((button) => {
    button.disabled = true;
  });

  try {
    const { ok, status, data } = await apiJson(
      `/api/admin/quota-requests/${encodeURIComponent(request.id)}/${action}`,
      { method: "POST", token: session.token, body: {} },
    );
    if (status === 401) {
      handleSessionExpired();
      return;
    }
    if (status === 429) return;

    // Reload even after a refusal: someone else may have decided it meanwhile.
    await loadAdminQuotaRequests();
    await refreshPendingQuotaRequestCount();
    if (ok) {
      if (action === "approve") {
        await loadAdminUsers();
        showAlert(
          "success",
          "Request Approved",
          `Storage quota for ${request.email} is now ${formatBytes(request.requestedQuotaBytes)}.`,
        );
      } else {
        showAlert(
          "success",
          "Request Rejected",
          `The request of ${request.email} was rejected. The quota is unchanged.`,
        );
      }
    } else {
      showAlert("danger", "Could Not Resolve Request", data.message || "Please try again.");
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not reach the server. Refresh to see the request's state.");
  } finally {
    buttons.forEach((button) => {
      button.disabled = false;
    });
  }
}

// #endregion

function initQuotaRequestFeatures() {
  quotaRequestOpenBtn.addEventListener("click", openQuotaRequestPanel);
  quotaRequestCloseBtn.addEventListener("click", closeQuotaRequestPanel);
  quotaRequestForm.addEventListener("submit", handleQuotaRequestSubmit);
  quotaRequestInput.addEventListener("input", () => setFieldError(quotaRequestInput, quotaRequestError, ""));
  quotaRequestUnit.addEventListener("change", () => setFieldError(quotaRequestInput, quotaRequestError, ""));

  adminQuotaRequestFilter.addEventListener("change", loadAdminQuotaRequests);
  adminRefreshQuotaRequestsBtn.addEventListener("click", loadAdminQuotaRequests);
}
