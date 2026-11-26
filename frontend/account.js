// Password handling: the strength meter shared by every "new password" field,
// "Forgot password?", the reset link from the email, and changing the password
// while signed in.

// #region shared password widgets

const strengthMeters = [];

// Fills container with a strength bar and the configured requirement checklist.
function createStrengthMeter(container) {
  container.innerHTML = `
    <div class="strength-meter-bar"><div class="strength-meter-fill"></div></div>
    <div class="strength-label">Password strength: <strong></strong></div>
    <ul class="strength-checklist"></ul>
  `;
  const fill = container.querySelector(".strength-meter-fill");
  const label = container.querySelector(".strength-label strong");
  const list = container.querySelector(".strength-checklist");
  let lastValue = "";

  // Returns true when value meets every requirement.
  function update(value) {
    lastValue = value;
    const rules = Validation.passwordRules(value);

    list.innerHTML = "";
    rules.forEach((rule) => {
      const item = document.createElement("li");
      item.className = `rule-item${value && rule.met ? " valid" : ""}`;
      const dot = document.createElement("span");
      dot.className = "rule-dot";
      item.append(dot, ` ${rule.label}`);
      list.appendChild(item);
    });

    const met = rules.filter((rule) => rule.met).length;
    const ratio = met / rules.length;
    let [level, text, color] = ["strength-weak", "Weak", "var(--danger)"];
    if (!value) {
      [level, text, color] = ["", "None", "var(--text-subtle)"];
    } else if (met === rules.length) {
      [level, text, color] = ["strength-strong", "Strong (Ready)", "var(--success)"];
    } else if (ratio >= 0.7) {
      [level, text, color] = ["strength-good", "Good", "#3b82f6"];
    } else if (ratio >= 0.5) {
      [level, text, color] = ["strength-fair", "Fair", "var(--warning)"];
    }
    fill.className = `strength-meter-fill ${level}`;
    label.textContent = text;
    label.style.color = color;

    return Boolean(value) && met === rules.length;
  }

  const meter = {
    update,
    reset: () => update(""),
    // after the configured rules arrived
    refresh: () => update(lastValue),
  };
  meter.reset();
  strengthMeters.push(meter);
  return meter;
}

// Gives a password input the same show/hide button as the login form.
function addPasswordToggle(input) {
  const button = toggleLoginPasswordBtn.cloneNode(true);
  button.removeAttribute("id");
  button.classList.remove("active");
  button.setAttribute("aria-label", "Show password");
  button.addEventListener("click", () => togglePasswordVisibility(input, button));
  input.insertAdjacentElement("afterend", button);
}

function setFieldError(input, errorEl, message) {
  input.classList.toggle("is-invalid", Boolean(message));
  errorEl.textContent = message;
}

// Empties a form and returns its password fields to the hidden state.
function resetPasswordForm(form) {
  form.reset();
  form.querySelectorAll("input").forEach((input) => {
    input.classList.remove("is-valid", "is-invalid");
    if (input.hasAttribute("data-password-toggle")) input.type = "password";
  });
  form.querySelectorAll(".toggle-password").forEach((button) => {
    button.classList.remove("active");
  });
  form.querySelectorAll(".field-error").forEach((el) => {
    el.textContent = "";
  });
  form.querySelectorAll(".password-match-status").forEach((el) => {
    el.textContent = "";
    el.className = "password-match-status";
  });
}

function setButtonBusy(button, busy) {
  button.disabled = busy;
  button.querySelector(".btn-spinner")?.classList.toggle("hidden", !busy);
}

// Shows the RFC3339 timestamps in server messages in the user's local time.
function localizeTimestamps(message) {
  return message.replace(
    /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})/g,
    (timestamp) => new Date(timestamp).toLocaleString(),
  );
}

// A "new password + confirmation" pair with its meter and live feedback.
function createNewPasswordFields(prefix) {
  const password = document.getElementById(`${prefix}NewPassword`);
  const confirm = document.getElementById(`${prefix}ConfirmPassword`);
  const passwordErrorEl = document.getElementById(`${prefix}NewPasswordError`);
  const confirmErrorEl = document.getElementById(`${prefix}ConfirmPasswordError`);
  const matchStatus = document.getElementById(`${prefix}PasswordMatchStatus`);
  const meter = createStrengthMeter(
    document.getElementById(`${prefix}StrengthContainer`),
  );

  password.addEventListener("input", () =>
    evaluatePasswordInput(password, passwordErrorEl, confirm, {
      meter,
      confirmError: confirmErrorEl,
      matchStatus,
    }),
  );
  confirm.addEventListener("input", () =>
    evaluateConfirmPasswordInput(password, confirm, confirmErrorEl, matchStatus),
  );

  return {
    password,
    confirm,
    passwordErrorEl,
    meter,
    // Marks the invalid fields; true when both can be submitted.
    validate() {
      const problem = Validation.newPassword(password.value);
      setFieldError(password, passwordErrorEl, problem);
      let confirmProblem = "";
      if (!confirm.value) confirmProblem = "Please confirm your password";
      else if (confirm.value !== password.value)
        confirmProblem = "Passwords do not match";
      setFieldError(confirm, confirmErrorEl, confirmProblem);
      return !problem && !confirmProblem;
    },
  };
}

// #endregion

// #region forgot password (request the link)

const forgotPasswordForm = document.getElementById("forgotPasswordForm");
const forgotEmailInput = document.getElementById("forgotEmail");
const forgotEmailError = document.getElementById("forgotEmailError");
const forgotSubmitBtn = document.getElementById("forgotSubmitBtn");

function openForgotPassword() {
  hideAlert();
  setFieldError(forgotEmailInput, forgotEmailError, "");
  forgotEmailInput.value = loginEmailInput.value.trim();
  switchSection(forgotPasswordSection);
  forgotEmailInput.focus();
}

function backToLogin() {
  hideAlert();
  switchTab("login");
}

async function handleForgotPasswordSubmit(event) {
  event.preventDefault();
  hideAlert();

  const email = forgotEmailInput.value.trim();
  const problem = Validation.email(email, EMAIL_REGEX);
  setFieldError(forgotEmailInput, forgotEmailError, problem);
  if (problem) return;

  setButtonBusy(forgotSubmitBtn, true);
  try {
    const { ok, status, data } = await apiJson("/api/users/forgot-password", {
      method: "POST",
      body: { email },
    });
    if (ok) {
      // The server answers the same whether or not the account exists.
      showAlert(
        "success",
        "Check Your Email",
        data.message ||
          "If an account with that email exists, a password reset link has been sent.",
        mailInboxAction(),
      );
    } else if (status !== 429) {
      showAlert("danger", "Request Failed", data.message || "Could not request a reset link.");
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not connect to API Gateway.");
  } finally {
    setButtonBusy(forgotSubmitBtn, false);
  }
}

// #endregion

// #region reset password (the link in the email: /?resetToken=...)

const resetLinkForm = document.getElementById("resetPasswordForm");
const resetSubmitBtn = document.getElementById("resetSubmitBtn");
let resetFields = null;
let passwordResetToken = null;

function readResetTokenFromUrl() {
  return new URLSearchParams(window.location.search).get("resetToken");
}

async function checkResetTokenInUrl() {
  const token = readResetTokenFromUrl();
  if (!token) return;

  // Keep the one-time token out of the address bar and the history.
  window.history.replaceState({}, document.title, window.location.pathname);

  switchSection(resetPasswordSection);
  resetLinkForm.classList.add("hidden");
  showAlert("warning", "Checking Link...", "Please wait while we check your reset link...");

  try {
    const { ok, status, data } = await apiJson(
      `/api/users/reset-password?token=${encodeURIComponent(token)}`,
    );
    if (ok) {
      passwordResetToken = token;
      hideAlert();
      resetPasswordForm(resetLinkForm);
      resetFields.meter.reset();
      resetLinkForm.classList.remove("hidden");
      resetFields.password.focus();
      return;
    }
    if (status === 429) return;
    // 403: the password is too new to be changed again; a new link would not help.
    switchSection(status === 403 ? loginSection : forgotPasswordSection);
    showAlert(
      "danger",
      "Cannot Reset Password",
      localizeTimestamps(data.message || "This password reset link is invalid or has expired."),
    );
  } catch {
    switchSection(loginSection);
    showAlert(
      "danger",
      "Connection Error",
      "Could not check your reset link. Open the link from the email again.",
    );
  }
}

async function handleResetPasswordSubmit(event) {
  event.preventDefault();
  hideAlert();
  if (!passwordResetToken || !resetFields.validate()) return;

  setButtonBusy(resetSubmitBtn, true);
  try {
    const { ok, status, data } = await apiJson("/api/users/reset-password", {
      method: "POST",
      body: {
        token: passwordResetToken,
        password: resetFields.password.value,
        confirmPassword: resetFields.confirm.value,
      },
    });
    if (ok) {
      passwordResetToken = null;
      resetPasswordForm(resetLinkForm);
      session.end();
      switchTab("login");
      showAlert("success", "Password Updated", data.message || "Password updated. You can now log in.");
    } else if (status !== 429) {
      showAlert(
        "danger",
        "Reset Failed",
        localizeTimestamps(data.message || "Could not reset your password."),
      );
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not connect to API Gateway.");
  } finally {
    setButtonBusy(resetSubmitBtn, false);
  }
}

// #endregion

// #region change password (signed in)

const changePasswordForm = document.getElementById("changePasswordForm");
const currentPasswordInput = document.getElementById("changeCurrentPassword");
const currentPasswordError = document.getElementById("changeCurrentPasswordError");
const changePasswordSubmitBtn = document.getElementById("changePasswordSubmitBtn");
const changePasswordAccount = document.getElementById("changePasswordAccount");
let changeFields = null;

function clearChangePasswordForm() {
  resetPasswordForm(changePasswordForm);
  changeFields?.meter.reset();
}

function openChangePassword() {
  hideAlert();
  clearChangePasswordForm();
  changePasswordAccount.textContent = session.email || "";
  switchSection(changePasswordSection);
  currentPasswordInput.focus();
}

function leaveChangePassword() {
  clearChangePasswordForm();
  switchSection(canOpenAdminPanel() ? adminSection : dashboardSection);
}

async function handleChangePasswordSubmit(event) {
  event.preventDefault();
  hideAlert();

  const currentPassword = currentPasswordInput.value;
  const currentProblem = Validation.password(currentPassword);
  setFieldError(currentPasswordInput, currentPasswordError, currentProblem);
  const newIsValid = changeFields.validate();
  if (currentProblem || !newIsValid) return;
  if (changeFields.password.value === currentPassword) {
    setFieldError(
      changeFields.password,
      changeFields.passwordErrorEl,
      "New password must be different from your current password",
    );
    return;
  }

  setButtonBusy(changePasswordSubmitBtn, true);
  try {
    const { ok, status, data } = await apiJson("/api/users/change-password", {
      method: "POST",
      token: session.token,
      body: {
        currentPassword,
        newPassword: changeFields.password.value,
        confirmPassword: changeFields.confirm.value,
      },
    });
    if (ok) {
      leaveChangePassword();
      showAlert("success", "Password Changed", data.message || "Password changed successfully.");
    } else if (status === 401) {
      handleSessionExpired();
    } else if (status !== 429) {
      const message = localizeTimestamps(data.message || "Could not change your password.");
      if (/current password is incorrect/i.test(message)) {
        setFieldError(currentPasswordInput, currentPasswordError, message);
        currentPasswordInput.focus();
      }
      showAlert("danger", "Password Not Changed", message);
    }
  } catch {
    showAlert("danger", "Connection Error", "Could not connect to API Gateway.");
  } finally {
    setButtonBusy(changePasswordSubmitBtn, false);
  }
}

// #endregion

function initAccountFeatures() {
  document.querySelectorAll("input[data-password-toggle]").forEach(addPasswordToggle);
  resetFields = createNewPasswordFields("reset");
  changeFields = createNewPasswordFields("change");

  document.getElementById("forgotPasswordLink").addEventListener("click", openForgotPassword);
  document.getElementById("forgotBackBtn").addEventListener("click", backToLogin);
  document.getElementById("resetBackBtn").addEventListener("click", () => {
    passwordResetToken = null;
    backToLogin();
  });
  forgotPasswordForm.addEventListener("submit", handleForgotPasswordSubmit);
  resetLinkForm.addEventListener("submit", handleResetPasswordSubmit);

  document.querySelectorAll(".open-change-password-btn").forEach((button) => {
    button.addEventListener("click", openChangePassword);
  });
  document.getElementById("changePasswordBackBtn").addEventListener("click", () => {
    hideAlert();
    leaveChangePassword();
  });
  changePasswordForm.addEventListener("submit", handleChangePasswordSubmit);
}
