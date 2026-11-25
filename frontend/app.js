const MAILPIT_URL = "http://localhost:8025";

const tabLogin = document.getElementById("tabLogin");
const tabRegister = document.getElementById("tabRegister");
const loginSection = document.getElementById("loginSection");
const otpSection = document.getElementById("otpSection");
const dashboardSection = document.getElementById("dashboardSection");
const adminSection = document.getElementById("adminSection");
const registerSection = document.getElementById("registerSection");
const forceChangePasswordSection = document.getElementById(
  "forceChangePasswordSection",
);

const forceChangePasswordForm = document.getElementById(
  "forceChangePasswordForm",
);
const forceNewPasswordInput = document.getElementById("forceNewPassword");
const forceConfirmPasswordInput = document.getElementById(
  "forceConfirmPassword",
);
const forceChangeSubmitBtn = document.getElementById("forceChangeSubmitBtn");
const forceChangeSpinner = document.getElementById("forceChangeSpinner");
const forceNewPasswordError = document.getElementById("forceNewPasswordError");
const forceConfirmPasswordError = document.getElementById(
  "forceConfirmPasswordError",
);
const toggleForceNewPasswordBtn = document.getElementById(
  "toggleForceNewPasswordBtn",
);
const toggleForceConfirmPasswordBtn = document.getElementById(
  "toggleForceConfirmPasswordBtn",
);
const forceStrengthBarFill = document.getElementById("forceStrengthBarFill");
const forceStrengthLabelText = document.getElementById(
  "forceStrengthLabelText",
);
const forceRuleLength = document.getElementById("force-rule-length");
const forceRuleUppercase = document.getElementById("force-rule-uppercase");
const forceRuleLowercase = document.getElementById("force-rule-lowercase");
const forceRuleNumber = document.getElementById("force-rule-number");
const forceRuleSpecial = document.getElementById("force-rule-special");
const forcePasswordMatchStatus = document.getElementById(
  "forcePasswordMatchStatus",
);

let restrictedSessionToken = null;

const loginForm = document.getElementById("loginForm");
const loginEmailInput = document.getElementById("loginEmail");
const loginPasswordInput = document.getElementById("loginPassword");
const loginSubmitBtn = document.getElementById("loginSubmitBtn");
const loginSpinner = document.getElementById("loginSpinner");
const loginEmailError = document.getElementById("loginEmailError");
const loginPasswordError = document.getElementById("loginPasswordError");
const toggleLoginPasswordBtn = document.getElementById(
  "toggleLoginPasswordBtn",
);

const otpForm = document.getElementById("otpForm");
const otpCodeInput = document.getElementById("otpCode");
const otpSubmitBtn = document.getElementById("otpSubmitBtn");
const otpSpinner = document.getElementById("otpSpinner");
const otpCodeError = document.getElementById("otpCodeError");
const otpEmailTarget = document.getElementById("otpEmailTarget");
const resendOtpBtn = document.getElementById("resendOtpBtn");
const backToLoginBtn = document.getElementById("backToLoginBtn");

const userEmailDisplay = document.getElementById("userEmailDisplay");
const jwtTokenDisplay = document.getElementById("jwtTokenDisplay");
const logoutBtn = document.getElementById("logoutBtn");
const logoutSpinner = document.getElementById("logoutSpinner");

const authCard = document.querySelector(".auth-card");
const authTabs = document.querySelector(".auth-tabs");
const toggleTokenDrawerBtn = document.getElementById("toggleTokenDrawerBtn");
const jwtDrawer = document.getElementById("jwtDrawer");
const navigateUpBtn = document.getElementById("navigateUpBtn");
const breadcrumbsContainer = document.getElementById("breadcrumbsContainer");
const currentFolderTitle = document.getElementById("currentFolderTitle");
const openCreateFolderBtn = document.getElementById("openCreateFolderBtn");
const createFolderForm = document.getElementById("createFolderForm");
const newFolderNameInput = document.getElementById("newFolderNameInput");
const createFolderSubmitBtn = document.getElementById("createFolderSubmitBtn");
const createFolderSpinner = document.getElementById("createFolderSpinner");
const cancelCreateFolderBtn = document.getElementById("cancelCreateFolderBtn");
const newFolderNameError = document.getElementById("newFolderNameError");
const folderLoadingState = document.getElementById("folderLoadingState");
const folderEmptyState = document.getElementById("folderEmptyState");
const folderItemsList = document.getElementById("folderItemsList");
const quotaUsageText = document.getElementById("quotaUsageText");
const quotaBar = document.getElementById("quotaBar");
const quotaBarFill = document.getElementById("quotaBarFill");
const fileUploadInput = document.getElementById("fileUploadInput");
const uploadFileBtn = document.getElementById("uploadFileBtn");
const uploadFileSpinner = document.getElementById("uploadFileSpinner");
const fileVersionUploadInput = document.getElementById("fileVersionUploadInput");

let versionTargetFile = null;
let versionTargetBtn = null;

let currentFolderId = null;
let currentBreadcrumbs = [];

const renameFolderForm = document.getElementById("renameFolderForm");
const renameFolderLabel = document.getElementById("renameFolderLabel");
const renameFolderNameInput = document.getElementById("renameFolderNameInput");
const renameFolderSubmitBtn = document.getElementById("renameFolderSubmitBtn");
const renameFolderSpinner = document.getElementById("renameFolderSpinner");
const cancelRenameFolderBtn = document.getElementById("cancelRenameFolderBtn");
const renameFolderNameError = document.getElementById("renameFolderNameError");

let renameFolderTarget = null; // { id: string, currentName: string }

let activeLoginEmail = "";
let activeLoginPassword = "";

const form = document.getElementById("registrationForm");
const firstNameInput = document.getElementById("firstName");
const lastNameInput = document.getElementById("lastName");
const emailInput = document.getElementById("email");
const passwordInput = document.getElementById("password");
const confirmPasswordInput = document.getElementById("confirmPassword");
const submitBtn = document.getElementById("submitBtn");
const btnSpinner = document.getElementById("btnSpinner");

const firstNameError = document.getElementById("firstNameError");
const lastNameError = document.getElementById("lastNameError");
const emailError = document.getElementById("emailError");
const passwordError = document.getElementById("passwordError");
const confirmPasswordError = document.getElementById("confirmPasswordError");
const passwordMatchStatus = document.getElementById("passwordMatchStatus");

const alertContainer = document.getElementById("alertContainer");
const alertIcon = document.getElementById("alertIcon");
const alertTitle = document.getElementById("alertTitle");
const alertMessage = document.getElementById("alertMessage");
const alertAction = document.getElementById("alertAction");
const alertCloseBtn = document.getElementById("alertCloseBtn");

const strengthBarFill = document.getElementById("strengthBarFill");
const strengthLabelText = document.getElementById("strengthLabelText");
const ruleLength = document.getElementById("rule-length");
const ruleUppercase = document.getElementById("rule-uppercase");
const ruleLowercase = document.getElementById("rule-lowercase");
const ruleNumber = document.getElementById("rule-number");
const ruleSpecial = document.getElementById("rule-special");

const togglePasswordBtn = document.getElementById("togglePasswordBtn");
const toggleConfirmPasswordBtn = document.getElementById(
  "toggleConfirmPasswordBtn",
);

const EMAIL_REGEX = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
const SPECIAL_CHAR_REGEX = /[!@#~$%^&*()+|_.,<>?/{}\-]/;

const renameFileForm = document.getElementById("renameFileForm");
const renameFileLabel = document.getElementById("renameFileLabel");
const renameFileNameInput = document.getElementById("renameFileNameInput");
const renameFileSubmitBtn = document.getElementById("renameFileSubmitBtn");
const renameFileSpinner = document.getElementById("renameFileSpinner");
const cancelRenameFileBtn = document.getElementById("cancelRenameFileBtn");
const renameFileNameError = document.getElementById("renameFileNameError");

let renameFileTarget = null; // { id: string, currentName: string }

function getCandidateEndpoints(path) {
  const endpoints = [];

  if (
    window.location.protocol === "http:" ||
    window.location.protocol === "https:"
  ) {
    endpoints.push(path);
  }

  const hostname = window.location.hostname || "localhost";
  // Fallback: call the API gateway directly over HTTPS.
  if (hostname === "127.0.0.1") {
    endpoints.push(`https://127.0.0.1:8443${path}`);
    endpoints.push(`https://localhost:8443${path}`);
  } else {
    endpoints.push(`https://localhost:8443${path}`);
    endpoints.push(`https://127.0.0.1:8443${path}`);
  }

  return [...new Set(endpoints)];
}

async function fetchWithFallback(path, options) {
  const endpoints = getCandidateEndpoints(path);
  let lastError = null;

  for (const url of endpoints) {
    try {
      const response = await fetch(url, options);
      if (response.status === 429) {
        showAlert("warning", "Too Many Requests", "You are sending requests too fast. Please slow down and try again.");
        return { response, url };
      }
      if (response.status === 404 && url === path && endpoints.length > 1) {
        continue;
      }
      return { response, url };
    } catch (err) {
      lastError = err;
    }
  }

  throw lastError;
}

async function init() {
  await session.load();

  firstNameInput.addEventListener("input", () =>
    validateNameField(firstNameInput, firstNameError, "First name"),
  );
  lastNameInput.addEventListener("input", () =>
    validateNameField(lastNameInput, lastNameError, "Last name"),
  );
  emailInput.addEventListener("input", handleEmailInput);
  passwordInput.addEventListener("input", handlePasswordInput);
  confirmPasswordInput.addEventListener("input", handleConfirmPasswordInput);

  togglePasswordBtn.addEventListener("click", () =>
    togglePasswordVisibility(passwordInput, togglePasswordBtn),
  );
  toggleConfirmPasswordBtn.addEventListener("click", () =>
    togglePasswordVisibility(confirmPasswordInput, toggleConfirmPasswordBtn),
  );

  form.addEventListener("submit", handleFormSubmit);

  alertCloseBtn.addEventListener("click", hideAlert);

  checkVerificationTokenInUrl();
  checkMagicLinkTokenInUrl();

  const requestMagicLinkBtn = document.getElementById("requestMagicLinkBtn");
  if (requestMagicLinkBtn) {
    requestMagicLinkBtn.addEventListener("click", handleMagicLinkRequest);
  }
  tabLogin.addEventListener("click", () => switchTab("login"));
  tabRegister.addEventListener("click", () => switchTab("register"));

  // Login, OTPS & Logout Events
  toggleLoginPasswordBtn.addEventListener("click", () =>
    togglePasswordVisibility(loginPasswordInput, toggleLoginPasswordBtn),
  );
  loginForm.addEventListener("submit", handleLoginSubmit);
  if (forceChangePasswordForm) {
    forceChangePasswordForm.addEventListener(
      "submit",
      handleForceChangePasswordSubmit,
    );
  }
  if (toggleForceNewPasswordBtn && forceNewPasswordInput) {
    toggleForceNewPasswordBtn.addEventListener("click", () =>
      togglePasswordVisibility(forceNewPasswordInput, toggleForceNewPasswordBtn),
    );
  }
  if (toggleForceConfirmPasswordBtn && forceConfirmPasswordInput) {
    toggleForceConfirmPasswordBtn.addEventListener("click", () =>
      togglePasswordVisibility(
        forceConfirmPasswordInput,
        toggleForceConfirmPasswordBtn,
      ),
    );
  }
  if (forceNewPasswordInput && forceConfirmPasswordInput) {
    forceNewPasswordInput.addEventListener("input", handleForcePasswordInput);
    forceConfirmPasswordInput.addEventListener(
      "input",
      handleForceConfirmPasswordInput,
    );
  }
  otpCodeInput.addEventListener("input", () => {
    otpCodeInput.value = otpCodeInput.value.replace(/\D/g, "");
  });
  otpForm.addEventListener("submit", handleOtpSubmit);
  resendOtpBtn.addEventListener("click", handleResendOtp);
  backToLoginBtn.addEventListener("click", () => {
    resetOtpForm();
    switchSection(loginSection);
  });
  logoutBtn.addEventListener("click", handleLogout);

  if (cancelRenameFolderBtn) {
    cancelRenameFolderBtn.addEventListener("click", closeRenameForm);
  }
  if (renameFolderForm) {
    renameFolderForm.addEventListener("submit", handleRenameFolderSubmit);
  }

  if (cancelRenameFileBtn) {
    cancelRenameFileBtn.addEventListener("click", closeRenameFileForm);
  }
  if (renameFileForm) {
    renameFileForm.addEventListener("submit", handleRenameFileSubmit);
  }

  checkExistingSession();
}

function togglePasswordVisibility(inputEl, btnEl) {
  const isPassword = inputEl.type === "password";
  inputEl.type = isPassword ? "text" : "password";
  btnEl.classList.toggle("active", isPassword);
  btnEl.setAttribute(
    "aria-label",
    isPassword ? "Hide password" : "Show password",
  );
}

function validateField(inputEl, errorEl, message) {
  if (!inputEl.value.trim()) {
    inputEl.classList.add("is-invalid");
    inputEl.classList.remove("is-valid");
    errorEl.textContent = message;
    return false;
  }
  inputEl.classList.remove("is-invalid");
  inputEl.classList.add("is-valid");
  errorEl.textContent = "";
  return true;
}

function validateNameField(inputEl, errorEl, label) {
  const problem = Validation.name(inputEl.value.trim(), label);
  inputEl.classList.toggle("is-invalid", Boolean(problem));
  inputEl.classList.toggle("is-valid", !problem);
  errorEl.textContent = problem;
  return !problem;
}

function handleEmailInput() {
  const email = emailInput.value.trim();
  const problem = Validation.email(email, EMAIL_REGEX);
  if (problem) {
    emailInput.classList.add("is-invalid");
    emailInput.classList.remove("is-valid");
    emailError.textContent = problem;
    return false;
  }

  emailInput.classList.remove("is-invalid");
  emailInput.classList.add("is-valid");
  emailError.textContent = "";
  return true;
}

function checkPasswordRules(password) {
  return {
    length: password.length >= 8,
    uppercase: /[A-Z]/.test(password),
    lowercase: /[a-z]/.test(password),
    number: /[0-9]/.test(password),
    special: SPECIAL_CHAR_REGEX.test(password),
  };
}

function evaluatePasswordInput(pwInput, pwError, confirmInput, ui) {
  const password = pwInput.value;
  const rules = checkPasswordRules(password);

  updateRuleItem(ui.length, rules.length);
  updateRuleItem(ui.uppercase, rules.uppercase);
  updateRuleItem(ui.lowercase, rules.lowercase);
  updateRuleItem(ui.number, rules.number);
  updateRuleItem(ui.special, rules.special);

  const score = Object.values(rules).filter(Boolean).length;
  updateStrengthMeterUI(ui.barFill, ui.labelText, score, password.length === 0);

  if (confirmInput.value.length > 0) {
    evaluateConfirmPasswordInput(
      pwInput,
      confirmInput,
      ui.confirmError,
      ui.matchStatus,
    );
  }

  if (password.length === 0) {
    pwInput.classList.remove("is-valid", "is-invalid");
    pwError.textContent = "";
    return false;
  }

  if (score === 5) {
    pwInput.classList.remove("is-invalid");
    pwInput.classList.add("is-valid");
    pwError.textContent = "";
    return true;
  }

  pwInput.classList.remove("is-valid");
  return false;
}

function evaluateConfirmPasswordInput(
  pwInput,
  confirmInput,
  confirmErrorEl,
  matchStatusEl,
) {
  const password = pwInput.value;
  const confirmPassword = confirmInput.value;

  if (!confirmPassword) {
    confirmInput.classList.remove("is-valid", "is-invalid");
    confirmErrorEl.textContent = "";
    matchStatusEl.textContent = "";
    matchStatusEl.className = "password-match-status";
    return false;
  }

  if (password !== confirmPassword) {
    confirmInput.classList.add("is-invalid");
    confirmInput.classList.remove("is-valid");
    confirmErrorEl.textContent = "Passwords do not match";
    matchStatusEl.textContent = "✖ Passwords do not match";
    matchStatusEl.className = "password-match-status mismatch";
    return false;
  }

  confirmInput.classList.remove("is-invalid");
  confirmInput.classList.add("is-valid");
  confirmErrorEl.textContent = "";
  matchStatusEl.textContent = "✔ Passwords match";
  matchStatusEl.className = "password-match-status match";
  return true;
}

function handlePasswordInput() {
  return evaluatePasswordInput(passwordInput, passwordError, confirmPasswordInput, {
    barFill: strengthBarFill,
    labelText: strengthLabelText,
    length: ruleLength,
    uppercase: ruleUppercase,
    lowercase: ruleLowercase,
    number: ruleNumber,
    special: ruleSpecial,
    confirmError: confirmPasswordError,
    matchStatus: passwordMatchStatus,
  });
}

function handleForcePasswordInput() {
  return evaluatePasswordInput(
    forceNewPasswordInput,
    forceNewPasswordError,
    forceConfirmPasswordInput,
    {
      barFill: forceStrengthBarFill,
      labelText: forceStrengthLabelText,
      length: forceRuleLength,
      uppercase: forceRuleUppercase,
      lowercase: forceRuleLowercase,
      number: forceRuleNumber,
      special: forceRuleSpecial,
      confirmError: forceConfirmPasswordError,
      matchStatus: forcePasswordMatchStatus,
    },
  );
}

function handleForceConfirmPasswordInput() {
  return evaluateConfirmPasswordInput(
    forceNewPasswordInput,
    forceConfirmPasswordInput,
    forceConfirmPasswordError,
    forcePasswordMatchStatus,
  );
}

function updateRuleItem(element, isValid) {
  if (isValid) {
    element.classList.add("valid");
    element.classList.remove("invalid-touched");
  } else {
    element.classList.remove("valid");
  }
}

function updateStrengthMeterUI(barFill, labelText, score, isEmpty) {
  barFill.className = "strength-meter-fill";

  if (isEmpty) {
    barFill.style.width = "0%";
    labelText.textContent = "None";
    labelText.style.color = "var(--text-subtle)";
    return;
  }

  switch (score) {
    case 1:
    case 2:
      barFill.classList.add("strength-weak");
      labelText.textContent = "Weak";
      labelText.style.color = "var(--danger)";
      break;
    case 3:
      barFill.classList.add("strength-fair");
      labelText.textContent = "Fair";
      labelText.style.color = "var(--warning)";
      break;
    case 4:
      barFill.classList.add("strength-good");
      labelText.textContent = "Good";
      labelText.style.color = "#3b82f6";
      break;
    case 5:
      barFill.classList.add("strength-strong");
      labelText.textContent = "Strong (Ready)";
      labelText.style.color = "var(--success)";
      break;
    default:
      barFill.classList.add("strength-weak");
      labelText.textContent = "Very Weak";
      labelText.style.color = "var(--danger)";
  }
}

function updateStrengthMeter(score, isEmpty) {
  updateStrengthMeterUI(strengthBarFill, strengthLabelText, score, isEmpty);
}

function handleConfirmPasswordInput() {
  const password = passwordInput.value;
  const confirmPassword = confirmPasswordInput.value;

  if (!confirmPassword) {
    confirmPasswordInput.classList.remove("is-valid", "is-invalid");
    confirmPasswordError.textContent = "";
    passwordMatchStatus.textContent = "";
    passwordMatchStatus.className = "password-match-status";
    return false;
  }

  if (password !== confirmPassword) {
    confirmPasswordInput.classList.add("is-invalid");
    confirmPasswordInput.classList.remove("is-valid");
    confirmPasswordError.textContent = "Passwords do not match";
    passwordMatchStatus.textContent = "✖ Passwords do not match";
    passwordMatchStatus.className = "password-match-status mismatch";
    return false;
  }

  confirmPasswordInput.classList.remove("is-invalid");
  confirmPasswordInput.classList.add("is-valid");
  confirmPasswordError.textContent = "";
  passwordMatchStatus.textContent = "✓ Passwords match";
  passwordMatchStatus.className = "password-match-status match";
  return true;
}

function validateFullForm() {
  let isValid = true;
  let firstInvalidEl = null;

  if (!validateNameField(firstNameInput, firstNameError, "First name")) {
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = firstNameInput;
  }

  if (!validateNameField(lastNameInput, lastNameError, "Last name")) {
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = lastNameInput;
  }

  if (!handleEmailInput()) {
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = emailInput;
  }

  const password = passwordInput.value;
  const rules = checkPasswordRules(password);
  const isPasswordValid = Object.values(rules).every(Boolean);

  if (Validation.password(password)) {
    passwordInput.classList.add("is-invalid");
    passwordError.textContent = Validation.password(password);
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = passwordInput;
  } else if (!isPasswordValid) {
    passwordInput.classList.add("is-invalid");
    passwordError.textContent =
      "Password does not meet all security requirements listed below";
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = passwordInput;
  }

  const confirmPassword = confirmPasswordInput.value;
  if (!confirmPassword) {
    confirmPasswordInput.classList.add("is-invalid");
    confirmPasswordError.textContent = "Please confirm your password";
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = confirmPasswordInput;
  } else if (password !== confirmPassword) {
    confirmPasswordInput.classList.add("is-invalid");
    confirmPasswordError.textContent = "Passwords do not match";
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = confirmPasswordInput;
  }

  if (!isValid && firstInvalidEl) {
    firstInvalidEl.focus();
  }

  return isValid;
}

async function handleFormSubmit(event) {
  event.preventDefault();
  hideAlert();

  if (!validateFullForm()) {
    showAlert(
      "danger",
      "Validation Error",
      "Please correct the highlighted errors before submitting the registration form.",
    );
    return;
  }

  const payload = {
    firstName: firstNameInput.value.trim(),
    lastName: lastNameInput.value.trim(),
    email: emailInput.value.trim(),
    password: passwordInput.value,
    confirmPassword: confirmPasswordInput.value,
  };

  setSubmitting(true);

  try {
    const { response } = await fetchWithFallback("/api/users/register", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify(payload),
    });

    let data = {};
    const textData = await response.text();
    try {
      data = JSON.parse(textData);
    } catch {
      data = { message: textData.trim() };
    }

    if (response.status === 201) {
      const successMsg =
        data.message ||
        "Registration successful. Check your email to verify your account.";
      showAlert(
        "success",
        "Registration Successful!",
        successMsg,
        `<a href="${MAILPIT_URL}" target="_blank" rel="noopener noreferrer" class="btn-mailpit">
          Open Mailpit Inbox (:8025) ↗
        </a>`,
      );

      form.reset();
      resetFormValidationVisuals();
    } else if (response.status === 409) {
      const conflictMsg =
        data.message || "A user with this email address already exists.";
      emailInput.classList.add("is-invalid");
      emailError.textContent = conflictMsg;
      emailInput.focus();

      showAlert("warning", "Account Already Exists", conflictMsg);
    } else if (response.status === 400) {
      const errorMsg = data.message || "Invalid registration request.";
      showAlert("danger", "Validation Failed", errorMsg);
    } else {
      const errorMsg =
        data.message || `Server responded with status code ${response.status}.`;
      showAlert("danger", "Registration Failed", errorMsg);
    }
  } catch (err) {
    console.error("Registration network error:", err);
    showAlert(
      "danger",
      "Connection Failed",
      "Unable to connect to API Gateway. Please ensure Docker containers (docker compose up) are running and accessible.",
    );
  } finally {
    setSubmitting(false);
  }
}

async function checkVerificationTokenInUrl() {
  const urlParams = new URLSearchParams(window.location.search);
  const token = urlParams.get("token");
  if (!token) return;

  // Ukloni token iz URL-a nakon čitanja
  window.history.replaceState({}, document.title, window.location.pathname);
  switchTab("login");

  showAlert(
    "warning",
    "Verifying Token...",
    "Please wait while we verify your email address...",
  );

  try {
    const { response } = await fetchWithFallback(
      `/api/users/verify?token=${encodeURIComponent(token)}`,
      {
        method: "GET",
        headers: {
          Accept: "application/json",
        },
      },
    );

    let data = {};
    const text = await response.text();
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      showAlert(
        "success",
        "Account Activated!",
        data.message || "Account successfully activated. You can now log in.",
      );
    } else {
      showAlert(
        "danger",
        "Activation Failed",
        data.message || "Invalid or expired verification token.",
      );
    }
  } catch (err) {
    showAlert(
      "danger",
      "Verification Error",
      "Could not connect to the API Gateway to verify your token.",
    );
  }
}

async function handleMagicLinkRequest() {
  hideAlert();
  const email = loginEmailInput.value.trim();
  if (Validation.email(email, EMAIL_REGEX)) {
    loginEmailInput.classList.add("is-invalid");
    loginEmailError.textContent = "Please enter your email above first";
    return;
  }
  loginEmailInput.classList.remove("is-invalid");
  loginEmailError.textContent = "";

  showAlert("warning", "Sending...", "Requesting magic login link...");

  try {
    const { response } = await fetchWithFallback(
      "/api/users/magic-link-request",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      showAlert(
        "success",
        "Magic Link Sent!",
        data.message || "Check Mailpit (:8025) for your direct login link.",
      );
    } else {
      showAlert(
        "danger",
        "Request Failed",
        data.message || "Could not send magic link.",
      );
    }
  } catch (err) {
    showAlert("danger", "Error", "Could not connect to send magic link.");
  }
}

async function checkMagicLinkTokenInUrl() {
  const urlParams = new URLSearchParams(window.location.search);
  const magicToken = urlParams.get("magicToken");
  if (!magicToken) return;

  // Clear query string from URL for clean appearance
  window.history.replaceState({}, document.title, window.location.pathname);

  showAlert(
    "warning",
    "Logging in...",
    "Verifying magic link token and authenticating...",
  );

  try {
    const { response } = await fetchWithFallback(
      "/api/users/magic-link-login",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: magicToken }),
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok && data.token) {
      if (data.restricted) {
        restrictedSessionToken = data.token;
        switchSection(forceChangePasswordSection);
        showAlert(
          "warning",
          "Password Update Required",
          data.message ||
          "Your ban period has expired. Please choose a new password.",
        );
      } else {
        await session.start({
          token: data.token,
          email: data.user?.email || "user",
          role: data.user?.role,
        });
        showAuthenticatedView(data.user?.email || "user", data.token);
        showAlert(
          "success",
          "Welcome!",
          "Logged in successfully via magic link.",
        );
      }
    } else {
      showAlert(
        "danger",
        "Magic Login Failed",
        data.message || "Invalid, expired, or already used magic link.",
      );
    }
  } catch (err) {
    showAlert("danger", "Error", "Could not complete magic login.");
  }
}

function setSubmitting(isSubmitting) {
  submitBtn.disabled = isSubmitting;
  const btnText = submitBtn.querySelector(".btn-text");

  if (isSubmitting) {
    btnText.textContent = "Registering Account...";
    btnSpinner.classList.remove("hidden");
  } else {
    btnText.textContent = "Create Account";
    btnSpinner.classList.add("hidden");
  }
}

function showAlert(type, title, message, actionHtml = null) {
  alertContainer.className = `alert-container alert-${type}`;
  alertTitle.textContent = title;
  alertMessage.textContent = message;

  if (type === "success") {
    alertIcon.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
        <polyline points="22 4 12 14.01 9 11.01"/>
      </svg>
    `;
  } else if (type === "warning") {
    alertIcon.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/>
        <line x1="12" y1="9" x2="12" y2="13"/>
        <line x1="12" y1="17" x2="12.01" y2="17"/>
      </svg>
    `;
  } else {
    alertIcon.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
    `;
  }

  if (actionHtml) {
    alertAction.innerHTML = actionHtml;
    alertAction.classList.remove("hidden");
  } else {
    alertAction.innerHTML = "";
    alertAction.classList.add("hidden");
  }

  alertContainer.classList.remove("hidden");
  alertContainer.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

function hideAlert() {
  alertContainer.classList.add("hidden");
}

function resetFormValidationVisuals() {
  const inputs = [
    firstNameInput,
    lastNameInput,
    emailInput,
    passwordInput,
    confirmPasswordInput,
  ];
  inputs.forEach((input) => {
    input.classList.remove("is-valid", "is-invalid");
  });

  const errors = [
    firstNameError,
    lastNameError,
    emailError,
    passwordError,
    confirmPasswordError,
  ];
  errors.forEach((err) => {
    err.textContent = "";
  });

  passwordMatchStatus.textContent = "";
  passwordMatchStatus.className = "password-match-status";

  updateStrengthMeter(0, true);

  [ruleLength, ruleUppercase, ruleLowercase, ruleNumber, ruleSpecial].forEach(
    (rule) => {
      rule.classList.remove("valid", "invalid-touched");
    },
  );
}

document.addEventListener("DOMContentLoaded", init);

function switchTab(tab) {
  hideAlert();
  if (tab === "login") {
    tabLogin.classList.add("active");
    tabRegister.classList.remove("active");
    if (session.token) {
      switchSection(canOpenAdminPanel() ? adminSection : dashboardSection);
    } else {
      switchSection(loginSection);
    }
  } else {
    tabRegister.classList.add("active");
    tabLogin.classList.remove("active");
    switchSection(registerSection);
  }
}

function canOpenDashboard() {
  return Boolean(session.token) && session.role === "RK";
}

function canOpenAdminPanel() {
  return Boolean(session.token) && session.role === "A";
}

function switchSection(sectionToDisplay) {
  if (sectionToDisplay === dashboardSection && !canOpenDashboard()) {
    sectionToDisplay = loginSection;
  }
  if (sectionToDisplay === adminSection && !canOpenAdminPanel()) {
    sectionToDisplay = loginSection;
  }
  [
    loginSection,
    otpSection,
    dashboardSection,
    adminSection,
    registerSection,
    forceChangePasswordSection,
  ].forEach((sec) => {
    if (sec) sec.classList.add("hidden");
  });
  if (sectionToDisplay) sectionToDisplay.classList.remove("hidden");

  const isDashboard = sectionToDisplay === dashboardSection;
  const isAdmin = sectionToDisplay === adminSection;
  if (authCard) {
    authCard.classList.toggle("dashboard-wide", isDashboard);
    authCard.classList.toggle("admin-wide", isAdmin);
  }
  if (authTabs) authTabs.classList.toggle("hidden", isDashboard || isAdmin);
}

async function handleLoginSubmit(event) {
  event.preventDefault();
  hideAlert();

  const email = loginEmailInput.value.trim();
  const password = loginPasswordInput.value;

  const emailProblem = Validation.email(email, EMAIL_REGEX);
  if (emailProblem) {
    loginEmailInput.classList.add("is-invalid");
    loginEmailError.textContent = emailProblem;
    return;
  }
  const passwordProblem = Validation.password(password);
  if (passwordProblem) {
    loginPasswordInput.classList.add("is-invalid");
    loginPasswordError.textContent = passwordProblem;
    return;
  }

  activeLoginEmail = email;
  activeLoginPassword = password;
  loginSubmitBtn.disabled = true;
  loginSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback("/api/users/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      if (data.restricted) {
        restrictedSessionToken = data.token;
        switchSection(forceChangePasswordSection);
        showAlert(
          "warning",
          "Password Expired",
          data.message ||
          "Your ban period has expired. Please choose a new password.",
        );
        return;
      }

      otpEmailTarget.textContent = activeLoginEmail;
      resetOtpForm();
      switchSection(otpSection);
      showAlert(
        "success",
        "OTPS Code Sent!",
        "A 6-digit code has been sent to your email.",
      );
    } else {
      let displayMessage =
        data.message || "Invalid credentials or unverified account.";

      // Convert any ISO/RFC3339 timestamp in the message to the user's local timezone
      const isoRegex =
        /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})/;
      const match = displayMessage.match(isoRegex);
      if (match) {
        const localTimeStr = new Date(match[0]).toLocaleString();
        displayMessage = displayMessage.replace(match[0], localTimeStr);
      }

      showAlert("danger", "Login Failed", displayMessage);
    }
  } catch (err) {
    showAlert(
      "danger",
      "Connection Error",
      "Could not connect to API Gateway.",
    );
  } finally {
    loginSubmitBtn.disabled = false;
    loginSpinner.classList.add("hidden");
  }
}

async function handleOtpSubmit(event) {
  event.preventDefault();
  hideAlert();

  const otp = otpCodeInput.value.trim();
  const otpProblem = Validation.otp(otp);
  if (otpProblem) {
    otpCodeInput.classList.add("is-invalid");
    otpCodeError.textContent = otpProblem;
    return;
  }

  otpSubmitBtn.disabled = true;
  otpSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback("/api/users/verify-otp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: activeLoginEmail,
        password: activeLoginPassword,
        otp,
      }),
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok && data.token) {
      activeLoginPassword = "";
      if (data.restricted) {
        restrictedSessionToken = data.token;
        switchSection(forceChangePasswordSection);
        showAlert(
          "warning",
          "Password Update Required",
          data.message ||
          "Your ban period has expired. Please choose a new password.",
        );
      } else {
        resetOtpForm();
        await session.start({
          token: data.token,
          email: activeLoginEmail,
          role: data.user?.role,
        });
        showAuthenticatedView(activeLoginEmail, data.token);
        showAlert("success", "Authenticated!", "2FA verification successful.");
      }
    } else {
      showAlert(
        "danger",
        "Verification Failed",
        data.message || "Invalid or expired OTPS code.",
      );
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not verify OTPS code.");
  } finally {
    otpSubmitBtn.disabled = false;
    otpSpinner.classList.add("hidden");
  }
}

async function handleResendOtp() {
  hideAlert();
  try {
    const { response } = await fetchWithFallback("/api/users/resend-otp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: activeLoginEmail,
        password: activeLoginPassword,
      }),
    });

    if (response.ok) {
      showAlert(
        "success",
        "New Code Sent!",
        "A new OTPS code has been sent to your inbox.",
      );
    } else {
      showAlert("danger", "Resend Failed", "Unable to send a new code.");
    }
  } catch (err) {
    showAlert("danger", "Error", "Could not connect to resend OTPS.");
  }
}

function resetOtpForm() {
  if (otpForm) otpForm.reset();
  if (otpCodeInput) {
    otpCodeInput.value = "";
    otpCodeInput.classList.remove("is-invalid", "is-valid");
  }
  if (otpCodeError) otpCodeError.textContent = "";
}

function resetForceChangePasswordUI() {
  if (forceChangePasswordForm) forceChangePasswordForm.reset();
  [forceNewPasswordInput, forceConfirmPasswordInput].forEach((input) => {
    if (input) {
      input.type = "password";
      input.classList.remove("is-valid", "is-invalid");
    }
  });
  [toggleForceNewPasswordBtn, toggleForceConfirmPasswordBtn].forEach((btn) => {
    if (btn) {
      btn.classList.remove("active");
      btn.setAttribute("aria-label", "Show password");
    }
  });
  if (forceNewPasswordError) forceNewPasswordError.textContent = "";
  if (forceConfirmPasswordError) forceConfirmPasswordError.textContent = "";
  if (forcePasswordMatchStatus) {
    forcePasswordMatchStatus.textContent = "";
    forcePasswordMatchStatus.className = "password-match-status";
  }
  if (forceStrengthBarFill && forceStrengthLabelText) {
    updateStrengthMeterUI(forceStrengthBarFill, forceStrengthLabelText, 0, true);
  }
  [
    forceRuleLength,
    forceRuleUppercase,
    forceRuleLowercase,
    forceRuleNumber,
    forceRuleSpecial,
  ].forEach((rule) => {
    if (rule) rule.classList.remove("valid", "invalid-touched");
  });
}

async function handleLogout() {
  logoutBtn.disabled = true;
  logoutSpinner.classList.remove("hidden");
  try {
    await fetchWithFallback("/api/users/logout", { method: "POST" });
  } catch (err) {
    console.warn("Logout API warning:", err);
  } finally {
    session.end();
    resetOtpForm();
    currentFolderId = null;
    currentBreadcrumbs = [];
    activeLoginPassword = "";
    logoutBtn.disabled = false;
    logoutSpinner.classList.add("hidden");
    switchSection(loginSection);
    showAlert(
      "success",
      "Logged Out",
      "You have been successfully logged out.",
    );
  }
}

async function handleForceChangePasswordSubmit(event) {
  event.preventDefault();
  hideAlert();

  const newPassword = forceNewPasswordInput.value;
  const confirmPassword = forceConfirmPasswordInput.value;

  const newPasswordProblem = Validation.password(newPassword);
  if (newPasswordProblem) {
    forceNewPasswordInput.classList.add("is-invalid");
    forceNewPasswordError.textContent = newPasswordProblem;
    return;
  }
  if (!handleForcePasswordInput()) {
    forceNewPasswordInput.classList.add("is-invalid");
    forceNewPasswordError.textContent =
      "Password does not meet all security requirements";
    return;
  }
  forceNewPasswordInput.classList.remove("is-invalid");
  forceNewPasswordError.textContent = "";

  if (newPassword !== confirmPassword) {
    forceConfirmPasswordInput.classList.add("is-invalid");
    forceConfirmPasswordError.textContent = "Passwords do not match";
    return;
  }
  forceConfirmPasswordInput.classList.remove("is-invalid");
  forceConfirmPasswordError.textContent = "";

  forceChangeSubmitBtn.disabled = true;
  forceChangeSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback(
      "/api/users/change-expired-password",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${restrictedSessionToken}`,
        },
        body: JSON.stringify({
          password: newPassword,
          confirmPassword: confirmPassword,
        }),
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      restrictedSessionToken = null;
      resetForceChangePasswordUI();
      switchSection(loginSection);
      showAlert(
        "success",
        "Password Updated",
        "Your password has been reset and account lock cleared. Please sign in.",
      );
    } else {
      showAlert(
        "danger",
        "Update Failed",
        data.message || "Failed to update password.",
      );
    }
  } catch (err) {
    showAlert(
      "danger",
      "Connection Error",
      "Could not connect to update password.",
    );
  } finally {
    forceChangeSubmitBtn.disabled = false;
    forceChangeSpinner.classList.add("hidden");
  }
}

function showAuthenticatedView(email, token) {
  if (canOpenAdminPanel()) {
    showAdminPanel(email, token);
    return;
  }
  showDashboard(email, token);
}

function showDashboard(email, token) {
  activeLoginPassword = "";
  if (!canOpenDashboard()) {
    session.end();
    switchSection(loginSection);
    showAlert(
      "danger",
      "Access Denied",
      "This account does not have access to file storage.",
    );
    return;
  }
  userEmailDisplay.textContent = email;
  jwtTokenDisplay.value = token;
  switchSection(dashboardSection);
  loadUserQuota();
  loadFolderContents(null);
}

async function checkExistingSession() {
  const token = session.token;
  const email = session.email;
  if (!token || !email) return;

  if (canOpenAdminPanel()) {
    showAdminPanel(email, token);
    return;
  }

  try {
    const { response } = await fetchWithFallback("/api/folders", {
      method: "GET",
      headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
    });

    if (response.status === 401 || response.status === 403) {
      session.end();
      return; //stay on login if the token is expired
    }

    showDashboard(email, token);
  } catch {
    showDashboard(email, token);
  }
}

if (toggleTokenDrawerBtn) {
  toggleTokenDrawerBtn.addEventListener("click", () => {
    jwtDrawer.classList.toggle("hidden");
  });
}
if (openCreateFolderBtn) {
  openCreateFolderBtn.addEventListener("click", () => {
    createFolderForm.classList.remove("hidden");
    newFolderNameInput.value = "";
    newFolderNameInput.classList.remove("is-invalid");
    newFolderNameError.textContent = "";
    newFolderNameInput.focus();
  });
}
if (cancelCreateFolderBtn) {
  cancelCreateFolderBtn.addEventListener("click", () => {
    createFolderForm.classList.add("hidden");
  });
}
if (createFolderForm) {
  createFolderForm.addEventListener("submit", handleCreateFolderSubmit);
}
if (navigateUpBtn) {
  navigateUpBtn.addEventListener("click", () => {
    if (currentBreadcrumbs.length <= 1) {
      loadFolderContents(null);
    } else {
      const parentCrumb = currentBreadcrumbs[currentBreadcrumbs.length - 2];
      loadFolderContents(parentCrumb.id);
    }
  });
}

if (uploadFileBtn && fileUploadInput) {
  uploadFileBtn.addEventListener("click", () => {
    hideAlert();
    fileUploadInput.value = "";
    fileUploadInput.click();
  });
  fileUploadInput.addEventListener("change", handleFileUploadChange);
}

if (fileVersionUploadInput) {
  fileVersionUploadInput.addEventListener("change", handleFileVersionUploadChange);
}

async function loadFolderContents(folderId) {
  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  currentFolderId = folderId || null;
  folderLoadingState.classList.remove("hidden");
  folderEmptyState.classList.add("hidden");
  folderItemsList.innerHTML = "";

  const endpoint = currentFolderId
    ? `/api/folders/${encodeURIComponent(currentFolderId)}`
    : "/api/folders";

  try {
    const { response } = await fetchWithFallback(endpoint, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
      },
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
      return;
    }

    if (response.status === 403) {
      showAlert("danger", "Access Denied", "You do not have permission to access this resource.");
      return;
    }

    if (!response.ok) {
      showAlert(
        "danger",
        "Error",
        data.message || "Failed to load folder contents.",
      );
      return;
    }

    currentBreadcrumbs = Array.isArray(data.breadcrumbs)
      ? data.breadcrumbs
      : [];
    renderBreadcrumbs(currentBreadcrumbs);
    renderFolderItems(data.folders || [], data.files || []);
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    folderLoadingState.classList.add("hidden");
  }
}

function renderBreadcrumbs(breadcrumbs) {
  breadcrumbsContainer.innerHTML = "";

  const rootBtn = document.createElement("button");
  rootBtn.type = "button";
  rootBtn.className = `breadcrumb-item ${breadcrumbs.length === 0 ? "active" : ""}`;
  rootBtn.textContent = "My Drive";
  if (breadcrumbs.length > 0) {
    rootBtn.addEventListener("click", () => loadFolderContents(null));
  }
  breadcrumbsContainer.appendChild(rootBtn);

  breadcrumbs.forEach((crumb, idx) => {
    const sep = document.createElement("span");
    sep.className = "breadcrumb-sep";
    sep.textContent = "/";
    breadcrumbsContainer.appendChild(sep);

    const isLast = idx === breadcrumbs.length - 1;
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = `breadcrumb-item ${isLast ? "active" : ""}`;
    btn.textContent = crumb.name;
    if (!isLast) {
      btn.addEventListener("click", () => loadFolderContents(crumb.id));
    }
    breadcrumbsContainer.appendChild(btn);
  });

  if (breadcrumbs.length > 0) {
    navigateUpBtn.classList.remove("hidden");
    currentFolderTitle.textContent = breadcrumbs[breadcrumbs.length - 1].name;
  } else {
    navigateUpBtn.classList.add("hidden");
    currentFolderTitle.textContent = "My Drive";
  }
}

function openRenameForm(folderId, currentName) {
  hideAlert();
  if (createFolderForm) createFolderForm.classList.add("hidden");
  closeRenameFileForm();

  renameFolderTarget = { id: folderId, currentName };
  renameFolderLabel.textContent = `Rename folder "${currentName}"`;
  renameFolderNameInput.value = currentName;
  renameFolderNameInput.classList.remove("is-invalid");
  renameFolderNameError.textContent = "";
  renameFolderForm.classList.remove("hidden");
  renameFolderNameInput.focus();
  renameFolderNameInput.select();
}

function closeRenameForm() {
  renameFolderTarget = null;
  if (renameFolderForm) renameFolderForm.classList.add("hidden");
  if (renameFolderNameInput) {
    renameFolderNameInput.value = "";
    renameFolderNameInput.classList.remove("is-invalid");
  }
  if (renameFolderNameError) renameFolderNameError.textContent = "";
}

function openRenameFileForm(fileId, currentName) {
  hideAlert();
  if (createFolderForm) createFolderForm.classList.add("hidden");
  closeRenameForm();

  renameFileTarget = { id: fileId, currentName };
  renameFileLabel.textContent = `Rename file "${currentName}"`;
  renameFileNameInput.value = currentName;
  renameFileNameInput.classList.remove("is-invalid");
  renameFileNameError.textContent = "";
  renameFileForm.classList.remove("hidden");
  renameFileNameInput.focus();

  const dotIdx = currentName.lastIndexOf(".");
  if (dotIdx > 0) {
    renameFileNameInput.setSelectionRange(0, dotIdx);
  } else {
    renameFileNameInput.select();
  }
}

function closeRenameFileForm() {
  renameFileTarget = null;
  if (renameFileForm) renameFileForm.classList.add("hidden");
  if (renameFileNameInput) {
    renameFileNameInput.value = "";
    renameFileNameInput.classList.remove("is-invalid");
  }
  if (renameFileNameError) renameFileNameError.textContent = "";
}

function renderFolderItems(folders, files) {
  folderItemsList.innerHTML = "";

  if (folders.length === 0 && files.length === 0) {
    folderEmptyState.classList.remove("hidden");
    return;
  }
  folderEmptyState.classList.add("hidden");

  folders.forEach((folder) => {
    const row = document.createElement("div");
    row.className = "storage-item-row folder-row";
    row.title = `Open folder "${folder.name}"`;

    const createdDate = folder.createdAt
      ? new Date(folder.createdAt).toLocaleDateString()
      : "";

    row.innerHTML = `
      <div class="item-left">
        <svg class="item-icon-folder" viewBox="0 0 24 24" fill="currentColor">
          <path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
        </svg>
        <span class="item-name"></span>
      </div>
      <div class="item-right">
        <span class="item-meta">Folder • ${Validation.escapeHtml(createdDate)}</span>
        <button type="button" class="btn-rename-sm" title="Rename folder">Rename</button>
      </div>
    `;
    row.querySelector(".item-name").textContent = folder.name;

    const renameBtn = row.querySelector(".btn-rename-sm");
    renameBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      openRenameForm(folder.id, folder.name);
    });

    row.addEventListener("click", () => {
      closeRenameForm();
      loadFolderContents(folder.id);
    });
    folderItemsList.appendChild(row);
  });

  files.forEach((file) => {
    const row = document.createElement("div");
    row.className = "storage-item-row";

    const hasUpdated =
      file.updatedAt &&
      file.createdAt &&
      new Date(file.updatedAt).getTime() > new Date(file.createdAt).getTime() + 1000;
    const dateLabel = hasUpdated
      ? `Updated ${new Date(file.updatedAt).toLocaleString()}`
      : file.createdAt
        ? new Date(file.createdAt).toLocaleDateString()
        : "";

    row.innerHTML = `
      <div class="item-left">
        <svg class="item-icon-file" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
          <polyline points="14 2 14 8 20 8"/>
        </svg>
        <span class="item-name"></span>
      </div>
      <div class="item-right">
        <span class="item-meta">${Validation.escapeHtml(formatBytes(file.sizeBytes))} • ${Validation.escapeHtml(dateLabel)}</span>
        <button type="button" class="btn-rename-sm btn-rename-file-sm" title="Rename file">Rename</button>
        <button type="button" class="btn-rename-sm btn-version-sm" title="Upload a new version of this file">↑ New Version</button>
      </div>
    `;
    row.querySelector(".item-name").textContent = file.name;

    const renameFileBtn = row.querySelector(".btn-rename-file-sm");
    renameFileBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      openRenameFileForm(file.id, file.name);
    });

    const versionBtn = row.querySelector(".btn-version-sm");
    versionBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      startFileVersionUpload(file, versionBtn);
    });

    folderItemsList.appendChild(row);
  });
}

async function handleRenameFolderSubmit(event) {
  event.preventDefault();
  hideAlert();

  if (!renameFolderTarget) return;

  const newName = renameFolderNameInput.value.trim();
  const renameProblem = Validation.folderName(newName);
  if (renameProblem) {
    renameFolderNameInput.classList.add("is-invalid");
    renameFolderNameError.textContent = renameProblem;
    return;
  }
  renameFolderNameInput.classList.remove("is-invalid");
  renameFolderNameError.textContent = "";

  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  renameFolderSubmitBtn.disabled = true;
  renameFolderSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback(
      `/api/folders/${encodeURIComponent(renameFolderTarget.id)}`,
      {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name: newName }),
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      closeRenameForm();
      showAlert("success", "Folder Renamed", `Folder renamed to "${newName}".`);
      await loadFolderContents(currentFolderId);
    } else if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
    } else if (response.status === 403) {
      showAlert("danger", "Access Denied", "You do not have permission to rename this folder.");
    } else {
      renameFolderNameInput.classList.add("is-invalid");
      renameFolderNameError.textContent =
        data.message || "Could not rename folder.";
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    renameFolderSubmitBtn.disabled = false;
    renameFolderSpinner.classList.add("hidden");
  }
}

async function handleCreateFolderSubmit(event) {
  event.preventDefault();
  hideAlert();

  const name = newFolderNameInput.value.trim();
  const nameProblem = Validation.folderName(name);
  if (nameProblem) {
    newFolderNameInput.classList.add("is-invalid");
    newFolderNameError.textContent = nameProblem;
    return;
  }
  newFolderNameInput.classList.remove("is-invalid");
  newFolderNameError.textContent = "";

  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  createFolderSubmitBtn.disabled = true;
  createFolderSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback("/api/folders", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        name,
        parentId: currentFolderId,
      }),
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.status === 201) {
      createFolderForm.classList.add("hidden");
      newFolderNameInput.value = "";
      showAlert("success", "Folder Created", `Folder "${name}" was created.`);
      await loadFolderContents(currentFolderId);
    } else if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
    } else if (response.status === 403) {
      showAlert("danger", "Access Denied", "You do not have permission to create folders here.");
    } else {
      newFolderNameInput.classList.add("is-invalid");
      newFolderNameError.textContent =
        data.message || "Could not create folder.";
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    createFolderSubmitBtn.disabled = false;
    createFolderSpinner.classList.add("hidden");
  }
}

// #region file upload and storage quota (US-2.2)

// The API returns int64 values as JSON strings ("1048576"), so always go
// through Number() before doing any arithmetic.
function formatBytes(bytes) {
  const n = Number(bytes);
  if (!n || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const idx = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
  const value = n / Math.pow(1024, idx);
  return `${idx === 0 ? value : value.toFixed(1)} ${units[idx]}`;
}

function updateQuotaDisplay(usedBytes, totalBytes) {
  if (!quotaUsageText) return;
  const used = Number(usedBytes) || 0;
  const total = Number(totalBytes) || 0;
  if (total <= 0) return;

  quotaUsageText.textContent = `${formatBytes(used)} of ${formatBytes(total)} used`;
  if (quotaBar && quotaBarFill) {
    const pct = Math.min(100, (used / total) * 100);
    quotaBarFill.style.width = `${pct}%`;
    quotaBarFill.classList.toggle("is-nearly-full", pct >= 90);
    quotaBar.classList.remove("hidden");
  }
}

async function loadUserQuota() {
  const token = session.token;
  if (!token) return;

  try {
    const { response } = await fetchWithFallback("/api/files/quota", {
      method: "GET",
      headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
    });
    if (response.ok) {
      const data = await response.json();
      updateQuotaDisplay(data.usedStorageBytes, data.storageQuotaBytes);
    }
  } catch (err) {
    console.warn("Could not load storage quota:", err);
  }
}

// 2.2.1: the user picked a file; it goes into the folder that is open right now.
async function handleFileUploadChange(event) {
  const file = event.target.files && event.target.files[0];
  if (!file) return;

  hideAlert();
  const problem = Validation.fileName(file.name, file.size);
  if (problem) {
    showAlert("danger", "Cannot Upload File", problem);
    fileUploadInput.value = "";
    return;
  }

  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  const formData = new FormData();
  formData.append("file", file);
  if (currentFolderId) {
    formData.append("folderId", currentFolderId);
  }

  uploadFileBtn.disabled = true;
  uploadFileSpinner.classList.remove("hidden");

  try {
    // No Content-Type header: the browser sets multipart/form-data with the boundary.
    const { response } = await fetchWithFallback("/api/files/upload", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
      body: formData,
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.status === 201) {
      if (data.quota) {
        updateQuotaDisplay(data.quota.usedStorageBytes, data.quota.storageQuotaBytes);
      }
      showAlert(
        "success",
        "File Uploaded",
        `"${file.name}" (${formatBytes(file.size)}) was uploaded successfully.`,
      );
      await loadFolderContents(currentFolderId);
    } else if (response.status === 413 && /quota/i.test(data.message || "")) {
      // 2.2.4: quota violation - the server aborted the upload; show its message.
      showAlert(
        "danger",
        "Storage Quota Exceeded",
        data.message ||
          "Upload aborted: you do not have enough free storage space.",
      );
      loadUserQuota();
    } else if (response.status === 413) {
      showAlert("danger", "File Too Large", data.message || "This file is too large to upload.");
    } else if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
    } else if (response.status === 429) {
      // fetchWithFallback already showed the "Too Many Requests" alert
    } else {
      showAlert("danger", "Upload Failed", data.message || "Could not upload the file.");
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    uploadFileBtn.disabled = false;
    uploadFileSpinner.classList.add("hidden");
    fileUploadInput.value = "";
  }
}

function getFileExtension(name) {
  const idx = name.lastIndexOf(".");
  return idx >= 0 ? name.slice(idx).toLowerCase() : "";
}

function startFileVersionUpload(file, btnEl) {
  if (!fileVersionUploadInput) return;
  hideAlert();
  versionTargetFile = file;
  versionTargetBtn = btnEl;

  const ext = getFileExtension(file.name);
  fileVersionUploadInput.accept = ext || ".txt,.md,.csv,.json,.pdf,.png,.jpg,.jpeg,.gif";
  fileVersionUploadInput.value = "";
  fileVersionUploadInput.click();
}

async function handleFileVersionUploadChange(event) {
  const selectedFile = event.target.files && event.target.files[0];
  const targetFile = versionTargetFile;
  const triggerBtn = versionTargetBtn;

  if (!selectedFile || !targetFile) return;

  hideAlert();
  const problem = Validation.fileName(selectedFile.name, selectedFile.size);
  if (problem) {
    showAlert("danger", "Cannot Upload New Version", problem);
    fileVersionUploadInput.value = "";
    return;
  }

  const expectedExt = getFileExtension(targetFile.name);
  const actualExt = getFileExtension(selectedFile.name);
  if (expectedExt && actualExt !== expectedExt) {
    showAlert(
      "danger",
      "File Type Mismatch",
      `The new version must have the same extension (${expectedExt}) as "${targetFile.name}".`,
    );
    fileVersionUploadInput.value = "";
    return;
  }

  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  const formData = new FormData();
  formData.append("file", selectedFile);

  if (triggerBtn) {
    triggerBtn.disabled = true;
    triggerBtn.textContent = "Uploading…";
  }

  try {
    const { response } = await fetchWithFallback(
      `/api/files/${encodeURIComponent(targetFile.id)}/version`,
      {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, Accept: "application/json" },
        body: formData,
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      if (data.quota && Number(data.quota.storageQuotaBytes) > 0) {
        updateQuotaDisplay(data.quota.usedStorageBytes, data.quota.storageQuotaBytes);
      } else {
        loadUserQuota();
      }
      showAlert(
        "success",
        "New Version Uploaded",
        `New version of "${targetFile.name}" (${formatBytes(selectedFile.size)}) was uploaded successfully.`,
      );
      await loadFolderContents(currentFolderId);
    } else if (response.status === 413 && /quota/i.test(data.message || "")) {
      showAlert(
        "danger",
        "Storage Quota Exceeded",
        data.message ||
          "Version upload aborted: you do not have enough free storage space for the larger file.",
      );
      loadUserQuota();
    } else if (response.status === 413) {
      showAlert("danger", "File Too Large", data.message || "This file is too large to upload.");
    } else if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
    } else if (response.status === 429) {
      // fetchWithFallback već prikazuje upozorenje za previše zahtjeva
    } else {
      showAlert(
        "danger",
        "Version Upload Failed",
        data.message || "Could not upload the new file version.",
      );
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    if (triggerBtn) {
      triggerBtn.disabled = false;
      triggerBtn.textContent = "New Version";
    }
    fileVersionUploadInput.value = "";
    versionTargetFile = null;
    versionTargetBtn = null;
  }
}

// #endregion

// #region Admin Panel

const adminEmailDisplay = document.getElementById("adminEmailDisplay");
const adminToggleTokenBtn = document.getElementById("adminToggleTokenBtn");
const adminJwtDrawer = document.getElementById("adminJwtDrawer");
const adminJwtTokenDisplay = document.getElementById("adminJwtTokenDisplay");
const adminLogoutBtn = document.getElementById("adminLogoutBtn");
const adminLogoutSpinner = document.getElementById("adminLogoutSpinner");

const adminTabUsers = document.getElementById("adminTabUsers");
const adminTabNotifications = document.getElementById("adminTabNotifications");
const adminUsersPanel = document.getElementById("adminUsersPanel");
const adminNotificationsPanel = document.getElementById("adminNotificationsPanel");

const adminPreviewBanner = document.getElementById("adminPreviewBanner");
const adminStatUsers = document.getElementById("adminStatUsers");
const adminStatUsed = document.getElementById("adminStatUsed");
const adminStatQuota = document.getElementById("adminStatQuota");
const adminUserSearchInput = document.getElementById("adminUserSearchInput");
const adminRefreshUsersBtn = document.getElementById("adminRefreshUsersBtn");

const adminQuotaForm = document.getElementById("adminQuotaForm");
const adminQuotaTargetTitle = document.getElementById("adminQuotaTargetTitle");
const adminQuotaTargetMeta = document.getElementById("adminQuotaTargetMeta");
const adminNewQuotaMbInput = document.getElementById("adminNewQuotaMbInput");
const adminQuotaSubmitBtn = document.getElementById("adminQuotaSubmitBtn");
const adminQuotaSpinner = document.getElementById("adminQuotaSpinner");
const adminQuotaCancelBtn = document.getElementById("adminQuotaCancelBtn");
const adminQuotaError = document.getElementById("adminQuotaError");
const adminPresetBtns = document.querySelectorAll(".admin-preset-btn");

const adminUsersLoading = document.getElementById("adminUsersLoading");
const adminUsersEmpty = document.getElementById("adminUsersEmpty");
const adminUsersList = document.getElementById("adminUsersList");

const MB_BYTES = 1024 * 1024;
let adminUsers = [];
let adminPreviewMode = false;
let adminQuotaTargetUser = null;

function createPreviewAdminUsers() {
  return [
    {
      id: "preview-1",
      firstName: "Petar",
      lastName: "Petrovic",
      email: "petar@example.com",
      role: "RK",
      isVerified: true,
      usedStorageBytes: 72 * MB_BYTES,
      storageQuotaBytes: 100 * MB_BYTES,
    },
    {
      id: "preview-2",
      firstName: "Milica",
      lastName: "Jovanovic",
      email: "milica@example.com",
      role: "RK",
      isVerified: true,
      usedStorageBytes: 94 * MB_BYTES,
      storageQuotaBytes: 100 * MB_BYTES,
    },
    {
      id: "preview-3",
      firstName: "Marko",
      lastName: "Nikolic",
      email: "marko@example.com",
      role: "RK",
      isVerified: false,
      usedStorageBytes: 0,
      storageQuotaBytes: 100 * MB_BYTES,
    },
  ];
}

function showAdminPanel(email, token) {
  activeLoginPassword = "";
  if (!canOpenAdminPanel()) {
    session.end();
    switchSection(loginSection);
    showAlert("danger", "Access Denied", "Administrator privileges required.");
    return;
  }
  if (adminEmailDisplay) adminEmailDisplay.textContent = email;
  if (adminJwtTokenDisplay) adminJwtTokenDisplay.value = token;
  switchSection(adminSection);
  switchAdminTab("users");
  loadAdminUsers();
}

function switchAdminTab(tab) {
  const showUsers = tab === "users";
  if (adminTabUsers) {
    adminTabUsers.classList.toggle("active", showUsers);
    adminTabUsers.setAttribute("aria-selected", String(showUsers));
  }
  if (adminTabNotifications) {
    adminTabNotifications.classList.toggle("active", !showUsers);
    adminTabNotifications.setAttribute("aria-selected", String(!showUsers));
  }
  if (adminUsersPanel) adminUsersPanel.classList.toggle("hidden", !showUsers);
  if (adminNotificationsPanel) {
    adminNotificationsPanel.classList.toggle("hidden", showUsers);
  }
}

async function loadAdminUsers() {
  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  closeAdminQuotaForm();
  if (adminUsersLoading) adminUsersLoading.classList.remove("hidden");
  if (adminUsersEmpty) adminUsersEmpty.classList.add("hidden");
  if (adminUsersList) adminUsersList.innerHTML = "";

  try {
    const { response } = await fetchWithFallback("/api/admin/users", {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
      },
    });

    if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
      return;
    }

    if (response.status === 404 || response.status === 501) {
      adminPreviewMode = true;
      if (adminUsers.length === 0) {
        adminUsers = createPreviewAdminUsers();
      }
      if (adminPreviewBanner) adminPreviewBanner.classList.remove("hidden");
      renderAdminUsers();
      return;
    }

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (!response.ok) {
      showAlert(
        "danger",
        "Error Loading Users",
        data.message || "Could not load users.",
      );
      return;
    }

    adminPreviewMode = false;
    if (adminPreviewBanner) adminPreviewBanner.classList.add("hidden");
    adminUsers = Array.isArray(data) ? data : data.users || [];
    renderAdminUsers();
  } catch (err) {
    adminPreviewMode = true;
    if (adminUsers.length === 0) {
      adminUsers = createPreviewAdminUsers();
    }
    if (adminPreviewBanner) adminPreviewBanner.classList.remove("hidden");
    renderAdminUsers();
  } finally {
    if (adminUsersLoading) adminUsersLoading.classList.add("hidden");
  }
}

function updateAdminStats(users) {
  const totalUsers = users.length;
  const totalUsed = users.reduce(
    (sum, u) => sum + (Number(u.usedStorageBytes) || 0),
    0,
  );
  const totalQuota = users.reduce(
    (sum, u) => sum + (Number(u.storageQuotaBytes) || 0),
    0,
  );

  if (adminStatUsers) adminStatUsers.textContent = String(totalUsers);
  if (adminStatUsed) adminStatUsed.textContent = formatBytes(totalUsed);
  if (adminStatQuota) adminStatQuota.textContent = formatBytes(totalQuota);
}

function renderAdminUsers() {
  if (!adminUsersList) return;
  adminUsersList.innerHTML = "";

  updateAdminStats(adminUsers);

  const query = (adminUserSearchInput?.value || "").trim().toLowerCase();
  const filtered = adminUsers.filter((u) => {
    if (!query) return true;
    const fullName = `${u.firstName || ""} ${u.lastName || ""}`.toLowerCase();
    const email = (u.email || "").toLowerCase();
    return fullName.includes(query) || email.includes(query);
  });

  if (filtered.length === 0) {
    if (adminUsersEmpty) adminUsersEmpty.classList.remove("hidden");
    return;
  }
  if (adminUsersEmpty) adminUsersEmpty.classList.add("hidden");

  filtered.forEach((user) => {
    const used = Number(user.usedStorageBytes) || 0;
    const quota = Number(user.storageQuotaBytes) || 0;
    const pct = quota > 0 ? Math.min(100, (used / quota) * 100) : 0;
    const fullName =
      `${user.firstName || ""} ${user.lastName || ""}`.trim() || "Unnamed User";
    const isAdminUser = user.role === "A";

    const row = document.createElement("div");
    row.className = "admin-user-row";

    row.innerHTML = `
      <div class="admin-user-info">
        <div class="admin-user-name-line">
          <span class="admin-user-name"></span>
          <span class="role-pill ${isAdminUser ? "role-admin" : "role-user"}">
            ${isAdminUser ? "Admin" : "Regular"}
          </span>
          <span class="status-pill ${user.isVerified ? "verified" : "unverified"}">
            ${user.isVerified ? "Verified" : "Unverified"}
          </span>
        </div>
        <div class="admin-user-email"></div>
      </div>
      <div class="admin-user-storage">
        <div class="admin-storage-text">
          ${Validation.escapeHtml(formatBytes(used))} / ${Validation.escapeHtml(formatBytes(quota))}
        </div>
        <div class="quota-bar" aria-hidden="true">
          <div class="quota-bar-fill ${pct >= 90 ? "is-nearly-full" : ""}" style="width: ${pct}%"></div>
        </div>
      </div>
      <div class="admin-user-actions">
        <button type="button" class="btn-outline-sm admin-increase-quota-btn">
          + Increase Storage
        </button>
      </div>
    `;

    row.querySelector(".admin-user-name").textContent = fullName;
    row.querySelector(".admin-user-email").textContent = user.email || "";

    const increaseBtn = row.querySelector(".admin-increase-quota-btn");
    increaseBtn.addEventListener("click", () => openAdminQuotaForm(user));

    adminUsersList.appendChild(row);
  });
}

function openAdminQuotaForm(user) {
  hideAlert();
  adminQuotaTargetUser = user;
  const currentMb = Math.round((Number(user.storageQuotaBytes) || 0) / MB_BYTES);
  const usedStr = formatBytes(user.usedStorageBytes);
  const quotaStr = formatBytes(user.storageQuotaBytes);
  const fullName =
    `${user.firstName || ""} ${user.lastName || ""}`.trim() || user.email;

  if (adminQuotaTargetTitle) {
    adminQuotaTargetTitle.textContent = `Increase Storage for ${fullName}`;
  }
  if (adminQuotaTargetMeta) {
    adminQuotaTargetMeta.textContent = `${user.email} • Current usage: ${usedStr} / ${quotaStr}`;
  }
  if (adminNewQuotaMbInput) {
    adminNewQuotaMbInput.min = String(currentMb + 1);
    adminNewQuotaMbInput.value = String(currentMb + 100);
    adminNewQuotaMbInput.classList.remove("is-invalid");
    adminNewQuotaMbInput.focus();
  }
  if (adminQuotaError) adminQuotaError.textContent = "";
  if (adminQuotaForm) adminQuotaForm.classList.remove("hidden");
}

function closeAdminQuotaForm() {
  adminQuotaTargetUser = null;
  if (adminQuotaForm) adminQuotaForm.classList.add("hidden");
  if (adminNewQuotaMbInput) {
    adminNewQuotaMbInput.value = "";
    adminNewQuotaMbInput.classList.remove("is-invalid");
  }
  if (adminQuotaError) adminQuotaError.textContent = "";
}

async function handleAdminQuotaSubmit(event) {
  event.preventDefault();
  hideAlert();
  if (!adminQuotaTargetUser) return;

  const currentQuotaBytes = Number(adminQuotaTargetUser.storageQuotaBytes) || 0;
  const currentMb = Math.round(currentQuotaBytes / MB_BYTES);
  const newMb = Number(adminNewQuotaMbInput.value);

  if (!Number.isFinite(newMb) || newMb <= currentMb) {
    adminNewQuotaMbInput.classList.add("is-invalid");
    adminQuotaError.textContent = `New quota must be greater than current ${currentMb} MB.`;
    return;
  }

  adminNewQuotaMbInput.classList.remove("is-invalid");
  adminQuotaError.textContent = "";

  const newQuotaBytes = Math.round(newMb * MB_BYTES);
  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  adminQuotaSubmitBtn.disabled = true;
  adminQuotaSpinner.classList.remove("hidden");

  try {
    if (!adminPreviewMode) {
      const { response } = await fetchWithFallback(
        `/api/admin/users/${encodeURIComponent(adminQuotaTargetUser.id)}/quota`,
        {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ storageQuotaBytes: newQuotaBytes }),
        },
      );

      if (response.status === 401) {
        session.end();
        switchSection(loginSection);
        showAlert("warning", "Session Expired", "Please sign in again.");
        return;
      }

      if (response.status !== 404 && response.status !== 501 && !response.ok) {
        const text = await response.text();
        let data = {};
        try {
          data = JSON.parse(text);
        } catch {
          data = { message: text.trim() };
        }
        adminNewQuotaMbInput.classList.add("is-invalid");
        adminQuotaError.textContent =
          data.message || "Failed to update storage quota.";
        return;
      }
    }

    const updatedEmail = adminQuotaTargetUser.email;
    adminUsers = adminUsers.map((u) =>
      u.id === adminQuotaTargetUser.id
        ? { ...u, storageQuotaBytes: newQuotaBytes }
        : u,
    );
    closeAdminQuotaForm();
    renderAdminUsers();
    showAlert(
      "success",
      "Storage Quota Updated",
      `Storage quota for ${updatedEmail} increased to ${formatBytes(newQuotaBytes)}.`,
    );
  } catch (err) {
    const updatedEmail = adminQuotaTargetUser.email;
    adminUsers = adminUsers.map((u) =>
      u.id === adminQuotaTargetUser.id
        ? { ...u, storageQuotaBytes: newQuotaBytes }
        : u,
    );
    closeAdminQuotaForm();
    renderAdminUsers();
    showAlert(
      "success",
      "Storage Quota Updated (Preview)",
      `Storage quota for ${updatedEmail} increased to ${formatBytes(newQuotaBytes)}.`,
    );
  } finally {
    adminQuotaSubmitBtn.disabled = false;
    adminQuotaSpinner.classList.add("hidden");
  }
}

if (adminToggleTokenBtn) {
  adminToggleTokenBtn.addEventListener("click", () => {
    if (adminJwtDrawer) adminJwtDrawer.classList.toggle("hidden");
  });
}
if (adminLogoutBtn) {
  adminLogoutBtn.addEventListener("click", async () => {
    adminLogoutBtn.disabled = true;
    if (adminLogoutSpinner) adminLogoutSpinner.classList.remove("hidden");
    await handleLogout();
    adminUsers = [];
    closeAdminQuotaForm();
    adminLogoutBtn.disabled = false;
    if (adminLogoutSpinner) adminLogoutSpinner.classList.add("hidden");
  });
}
if (adminTabUsers) {
  adminTabUsers.addEventListener("click", () => switchAdminTab("users"));
}
if (adminTabNotifications) {
  adminTabNotifications.addEventListener("click", () =>
    switchAdminTab("notifications"),
  );
}
if (adminUserSearchInput) {
  adminUserSearchInput.addEventListener("input", renderAdminUsers);
}
if (adminRefreshUsersBtn) {
  adminRefreshUsersBtn.addEventListener("click", loadAdminUsers);
}
if (adminQuotaCancelBtn) {
  adminQuotaCancelBtn.addEventListener("click", closeAdminQuotaForm);
}
if (adminQuotaForm) {
  adminQuotaForm.addEventListener("submit", handleAdminQuotaSubmit);
}
adminPresetBtns.forEach((btn) => {
  btn.addEventListener("click", () => {
    if (!adminQuotaTargetUser || !adminNewQuotaMbInput) return;
    const addMb = Number(btn.dataset.addMb) || 0;
    const currentMb = Math.round(
      (Number(adminQuotaTargetUser.storageQuotaBytes) || 0) / MB_BYTES,
    );
    const baseMb = Math.max(currentMb, Number(adminNewQuotaMbInput.value) || 0);
    adminNewQuotaMbInput.value = String(baseMb + addMb);
    adminNewQuotaMbInput.classList.remove("is-invalid");
    if (adminQuotaError) adminQuotaError.textContent = "";
  });
});

async function handleRenameFileSubmit(event) {
  event.preventDefault();
  hideAlert();

  if (!renameFileTarget) return;

  const newName = renameFileNameInput.value.trim();
  const renameProblem = Validation.renameFileName(newName, renameFileTarget.currentName);
  if (renameProblem) {
    renameFileNameInput.classList.add("is-invalid");
    renameFileNameError.textContent = renameProblem;
    return;
  }
  renameFileNameInput.classList.remove("is-invalid");
  renameFileNameError.textContent = "";

  const token = session.token;
  if (!token) {
    switchSection(loginSection);
    return;
  }

  renameFileSubmitBtn.disabled = true;
  renameFileSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback(
      `/api/files/${encodeURIComponent(renameFileTarget.id)}`,
      {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name: newName }),
      },
    );

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok) {
      closeRenameFileForm();
      showAlert("success", "File Renamed", `File renamed to "${newName}".`);
      await loadFolderContents(currentFolderId);
    } else if (response.status === 401) {
      session.end();
      switchSection(loginSection);
      showAlert("warning", "Session Expired", "Please sign in again.");
    } else if (response.status === 403) {
      showAlert("danger", "Access Denied", "You do not have permission to rename this file.");
    } else {
      renameFileNameInput.classList.add("is-invalid");
      renameFileNameError.textContent =
        data.message || "Could not rename file.";
    }
  } catch (err) {
    showAlert("danger", "Connection Error", "Could not reach Storage Service.");
  } finally {
    renameFileSubmitBtn.disabled = false;
    renameFileSpinner.classList.add("hidden");
  }
}

// #endregion
