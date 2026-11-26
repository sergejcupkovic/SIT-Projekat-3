// Quota increase requests: users submit them, admins approve or reject them.

const QUOTA_STATUS_LABELS = {
  pending: "Pending",
  approved: "Approved",
  rejected: "Rejected",
};

const formatDateTime = (value) => (value ? new Date(value).toLocaleString() : "");

function textElement(tag, className, text) {
  const el = document.createElement(tag);
  el.className = className;
  el.textContent = text;
  return el;
}

function quotaStatusPill(status) {
  return textElement("span", `status-pill request-${status}`, QUOTA_STATUS_LABELS[status] || status);
}

// #region regular user

const quotaRequestOpenBtn = document.getElementById("quotaRequestOpenBtn");
const quotaRequestPanel = document.getElementById("quotaRequestPanel");
const quotaRequestSummary = document.getElementById("quotaRequestSummary");
const quotaRequestNotice = document.getElementById("quotaRequestNotice");
const quotaRequestForm = document.getElementById("quotaRequestForm");
const quotaRequestInput = document.getElementById("quotaRequestInput");
const quotaRequestHint = document.getElementById("quotaRequestHint");
const quotaRequestError = document.getElementById("quotaRequestError");
const quotaRequestSubmitBtn = document.getElementById("quotaRequestSubmitBtn");
const quotaRequestCloseBtn = document.getElementById("quotaRequestCloseBtn");
const quotaRequestHistory = document.getElementById("quotaRequestHistory");

let ownQuotaBytes = 0;

function closeQuotaRequestPanel() {
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
  quotaRequestPanel.classList.remove("hidden");
  quotaRequestSummary.textContent = "Loading…";

  try {
    const token = session.token;
    const limits = await loadLimits();
    const quota = await apiJson("/api/files/quota", { token });
    const mine = await apiJson("/api/users/quota-requests", { token });

    if (quota.status === 401 || mine.status === 401) {
      handleSessionExpired();
      return;
    }
    if (!limits || !quota.ok || !mine.ok) {
      quotaRequestSummary.textContent = "Could not load your storage details. Try again.";
      return;
    }

    updateQuotaDisplay(quota.data.usedStorageBytes, quota.data.storageQuotaBytes);
    ownQuotaBytes = Number(quota.data.storageQuotaBytes);
    renderOwnQuotaRequests(mine.data.requests || [], limits);
  } catch {
    quotaRequestSummary.textContent = "Could not reach the server. Try again.";
  }
}

function renderOwnQuotaRequests(requests, limits) {
  quotaRequestSummary.textContent =
    `Current quota: ${formatBytes(ownQuotaBytes)} • Maximum: ${formatBytes(limits.maxQuotaBytes)}`;

  const pending = requests.find((request) => request.status === "pending");
  if (pending) {
    quotaRequestNotice.textContent =
      `Your request for ${formatBytes(pending.requestedQuotaBytes)}, sent ` +
      `${formatDateTime(pending.createdAt)}, is waiting for an administrator.`;
  } else if (ownQuotaBytes >= limits.maxQuotaBytes) {
    quotaRequestNotice.textContent = "You already have the maximum quota.";
  } else {
    quotaRequestForm.classList.remove("hidden");
    quotaRequestInput.value = toMb(Math.min(ownQuotaBytes * 2, limits.maxQuotaBytes));
    quotaRequestHint.textContent =
      `Choose more than ${toMb(ownQuotaBytes)} MB, up to ${toMb(limits.maxQuotaBytes)} MB.`;
    quotaRequestInput.focus();
  }

  requests.slice(0, 5).forEach((request) => {
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

async function handleQuotaRequestSubmit(event) {
  event.preventDefault();
  hideAlert();

  const limits = await loadLimits();
  if (!limits) return;
  const bytes = enteredQuotaBytes(quotaRequestInput);
  let problem = quotaRangeProblem(bytes, limits);
  if (!problem && bytes <= ownQuotaBytes) {
    problem = `Requested quota must be greater than your current quota of ${formatBytes(ownQuotaBytes)}`;
  }
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
      showAlert("success", "Request Sent", `Your request for ${formatBytes(bytes)} was sent to an administrator.`);
    } else if (status === 401) {
      handleSessionExpired();
    } else {
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

// the number on the "Quota Requests" tab
async function refreshPendingQuotaRequestCount() {
  try {
    const { ok, data } = await apiJson("/api/admin/quota-requests?status=pending", {
      token: session.token,
    });
    if (!ok) return;
    const count = (data.requests || []).length;
    adminQuotaRequestsCount.textContent = String(count);
    adminQuotaRequestsCount.classList.toggle("hidden", count === 0);
  } catch (err) {
    console.warn("Could not count pending quota requests:", err);
  }
}

function clearAdminQuotaRequests() {
  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestFilter.value = "pending";
  adminQuotaRequestsCount.classList.add("hidden");
}

async function loadAdminQuotaRequests() {
  const filter = adminQuotaRequestFilter.value;
  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestsEmpty.classList.add("hidden");
  adminQuotaRequestsLoading.classList.remove("hidden");

  try {
    const query = filter ? `?status=${encodeURIComponent(filter)}` : "";
    const { ok, status, data } = await apiJson(`/api/admin/quota-requests${query}`, {
      token: session.token,
    });
    if (status === 401) {
      handleSessionExpired();
    } else if (!ok) {
      showAlert("danger", "Error Loading Requests", data.message || "Could not load quota requests.");
    } else {
      renderAdminQuotaRequests(data.requests || []);
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not load quota requests.");
  } finally {
    adminQuotaRequestsLoading.classList.add("hidden");
  }
}

function renderAdminQuotaRequests(requests) {
  adminQuotaRequestsList.innerHTML = "";
  adminQuotaRequestsEmpty.classList.toggle("hidden", requests.length > 0);

  requests.forEach((request) => {
    const fullName = `${request.firstName || ""} ${request.lastName || ""}`.trim() || "Unknown User";
    const isPending = request.status === "pending";

    const nameLine = document.createElement("div");
    nameLine.className = "admin-user-name-line";
    nameLine.append(textElement("span", "admin-user-name", fullName), quotaStatusPill(request.status));

    const info = document.createElement("div");
    info.className = "admin-user-info";
    info.append(
      nameLine,
      textElement("div", "admin-user-email", request.email),
      textElement("div", "quota-request-time", `Requested ${formatDateTime(request.createdAt)}`),
    );

    const sizes = document.createElement("div");
    sizes.className = "admin-user-storage";
    sizes.append(
      textElement(
        "div",
        "admin-storage-text",
        `Current: ${formatBytes(request.currentQuotaBytes)} (${formatBytes(request.usedStorageBytes)} used)`,
      ),
      textElement("div", "admin-storage-text quota-request-size", `Requested: ${formatBytes(request.requestedQuotaBytes)}`),
    );

    const row = document.createElement("div");
    row.className = "admin-user-row";
    row.append(info, sizes);

    if (isPending) {
      const approve = textElement("button", "btn-primary-sm", "Approve");
      const reject = textElement("button", "btn-danger-sm", "Reject");
      approve.type = "button";
      reject.type = "button";
      approve.addEventListener("click", () => decideQuotaRequest(request, "approve"));
      reject.addEventListener("click", () => decideQuotaRequest(request, "reject"));

      const actions = document.createElement("div");
      actions.className = "admin-user-actions";
      actions.append(approve, reject);
      row.append(actions);
    }

    adminQuotaRequestsList.appendChild(row);
  });
}

// action is "approve" or "reject"
async function decideQuotaRequest(request, action) {
  hideAlert();
  try {
    const { ok, status, data } = await apiJson(
      `/api/admin/quota-requests/${encodeURIComponent(request.id)}/${action}`,
      { method: "POST", token: session.token, body: {} },
    );
    if (status === 401) {
      handleSessionExpired();
      return;
    }

    // reload even after a refusal: the request may have been decided meanwhile
    await loadAdminQuotaRequests();
    await refreshPendingQuotaRequestCount();
    if (!ok) {
      showAlert("danger", "Could Not Resolve Request", data.message || "Please try again.");
    } else if (action === "approve") {
      await loadAdminUsers();
      showAlert("success", "Request Approved", `Storage quota for ${request.email} is now ${formatBytes(request.requestedQuotaBytes)}.`);
    } else {
      showAlert("success", "Request Rejected", `The quota of ${request.email} is unchanged.`);
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not reach the server. Refresh to see the request's state.");
  }
}

// #endregion

function initQuotaRequestFeatures() {
  quotaRequestOpenBtn.addEventListener("click", openQuotaRequestPanel);
  quotaRequestCloseBtn.addEventListener("click", closeQuotaRequestPanel);
  quotaRequestForm.addEventListener("submit", handleQuotaRequestSubmit);
  quotaRequestInput.addEventListener("input", () => setFieldError(quotaRequestInput, quotaRequestError, ""));

  adminQuotaRequestFilter.addEventListener("change", loadAdminQuotaRequests);
  adminRefreshQuotaRequestsBtn.addEventListener("click", loadAdminQuotaRequests);
}
