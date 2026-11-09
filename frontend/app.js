const MAILPIT_URL = "http://localhost:8025";

const tabLogin = document.getElementById("tabLogin");
const tabRegister = document.getElementById("tabRegister");
const loginSection = document.getElementById("loginSection");
const otpSection = document.getElementById("otpSection");
const dashboardSection = document.getElementById("dashboardSection");
const registerSection = document.getElementById("registerSection");

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

let activeLoginEmail = "";

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

function getCandidateEndpoints(path) {
  const endpoints = [];

  if (
    window.location.protocol === "http:" ||
    window.location.protocol === "https:"
  ) {
    endpoints.push(path);
  }

  const hostname = window.location.hostname || "localhost";
  if (hostname === "127.0.0.1") {
    endpoints.push(`http://127.0.0.1:8080${path}`);
    endpoints.push(`http://localhost:8080${path}`);
  } else {
    endpoints.push(`http://localhost:8080${path}`);
    endpoints.push(`http://127.0.0.1:8080${path}`);
  }

  return [...new Set(endpoints)];
}

async function fetchWithFallback(path, options) {
  const endpoints = getCandidateEndpoints(path);
  let lastError = null;

  for (const url of endpoints) {
    try {
      const response = await fetch(url, options);
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

function init() {
  firstNameInput.addEventListener("input", () =>
    validateField(firstNameInput, firstNameError, "First name is required"),
  );
  lastNameInput.addEventListener("input", () =>
    validateField(lastNameInput, lastNameError, "Last name is required"),
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
  tabLogin.addEventListener("click", () => switchTab("login"));
  tabRegister.addEventListener("click", () => switchTab("register"));

  // Login, OTPS & Logout Events
  toggleLoginPasswordBtn.addEventListener("click", () =>
    togglePasswordVisibility(loginPasswordInput, toggleLoginPasswordBtn),
  );
  loginForm.addEventListener("submit", handleLoginSubmit);
  otpForm.addEventListener("submit", handleOtpSubmit);
  resendOtpBtn.addEventListener("click", handleResendOtp);
  backToLoginBtn.addEventListener("click", () => switchSection(loginSection));
  logoutBtn.addEventListener("click", handleLogout);

  checkExistingSession();

  tabLogin.addEventListener("click", () => switchTab("login"));
  tabRegister.addEventListener("click", () => switchTab("register"));

  // Login, OTPS & Logout Events
  toggleLoginPasswordBtn.addEventListener("click", () =>
    togglePasswordVisibility(loginPasswordInput, toggleLoginPasswordBtn),
  );
  loginForm.addEventListener("submit", handleLoginSubmit);
  otpForm.addEventListener("submit", handleOtpSubmit);
  resendOtpBtn.addEventListener("click", handleResendOtp);
  backToLoginBtn.addEventListener("click", () => switchSection(loginSection));
  logoutBtn.addEventListener("click", handleLogout);

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

function handleEmailInput() {
  const email = emailInput.value.trim();
  if (!email) {
    emailInput.classList.add("is-invalid");
    emailInput.classList.remove("is-valid");
    emailError.textContent = "Email address is required";
    return false;
  }

  if (!EMAIL_REGEX.test(email)) {
    emailInput.classList.add("is-invalid");
    emailInput.classList.remove("is-valid");
    emailError.textContent =
      "Please enter a valid email address (e.g., user@example.com)";
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

function handlePasswordInput() {
  const password = passwordInput.value;
  const rules = checkPasswordRules(password);

  updateRuleItem(ruleLength, rules.length);
  updateRuleItem(ruleUppercase, rules.uppercase);
  updateRuleItem(ruleLowercase, rules.lowercase);
  updateRuleItem(ruleNumber, rules.number);
  updateRuleItem(ruleSpecial, rules.special);

  const score = Object.values(rules).filter(Boolean).length;
  updateStrengthMeter(score, password.length === 0);

  if (confirmPasswordInput.value.length > 0) {
    handleConfirmPasswordInput();
  }

  if (password.length === 0) {
    passwordInput.classList.remove("is-valid", "is-invalid");
    passwordError.textContent = "";
    return false;
  }

  if (score === 5) {
    passwordInput.classList.remove("is-invalid");
    passwordInput.classList.add("is-valid");
    passwordError.textContent = "";
    return true;
  } else {
    passwordInput.classList.remove("is-valid");
    return false;
  }
}

function updateRuleItem(element, isValid) {
  if (isValid) {
    element.classList.add("valid");
    element.classList.remove("invalid-touched");
  } else {
    element.classList.remove("valid");
  }
}

function updateStrengthMeter(score, isEmpty) {
  strengthBarFill.className = "strength-meter-fill";

  if (isEmpty) {
    strengthBarFill.style.width = "0%";
    strengthLabelText.textContent = "None";
    strengthLabelText.style.color = "var(--text-subtle)";
    return;
  }

  switch (score) {
    case 1:
    case 2:
      strengthBarFill.classList.add("strength-weak");
      strengthLabelText.textContent = "Weak";
      strengthLabelText.style.color = "var(--danger)";
      break;
    case 3:
      strengthBarFill.classList.add("strength-fair");
      strengthLabelText.textContent = "Fair";
      strengthLabelText.style.color = "var(--warning)";
      break;
    case 4:
      strengthBarFill.classList.add("strength-good");
      strengthLabelText.textContent = "Good";
      strengthLabelText.style.color = "#3b82f6";
      break;
    case 5:
      strengthBarFill.classList.add("strength-strong");
      strengthLabelText.textContent = "Strong (Ready)";
      strengthLabelText.style.color = "var(--success)";
      break;
    default:
      strengthBarFill.classList.add("strength-weak");
      strengthLabelText.textContent = "Very Weak";
      strengthLabelText.style.color = "var(--danger)";
  }
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

  if (
    !validateField(firstNameInput, firstNameError, "First name is required")
  ) {
    isValid = false;
    if (!firstInvalidEl) firstInvalidEl = firstNameInput;
  }

  if (!validateField(lastNameInput, lastNameError, "Last name is required")) {
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

  if (!password) {
    passwordInput.classList.add("is-invalid");
    passwordError.textContent = "Password is required";
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
    if (localStorage.getItem("jwt_token")) {
      switchSection(dashboardSection);
    } else {
      switchSection(loginSection);
    }
  } else {
    tabRegister.classList.add("active");
    tabLogin.classList.remove("active");
    switchSection(registerSection);
  }
}

function switchSection(sectionToDisplay) {
  [loginSection, otpSection, dashboardSection, registerSection].forEach((sec) =>
    sec.classList.add("hidden"),
  );
  sectionToDisplay.classList.remove("hidden");
}

async function handleLoginSubmit(event) {
  event.preventDefault();
  hideAlert();

  const email = loginEmailInput.value.trim();
  const password = loginPasswordInput.value;

  if (!email || !EMAIL_REGEX.test(email)) {
    loginEmailInput.classList.add("is-invalid");
    loginEmailError.textContent = "Please enter a valid email address";
    return;
  }
  if (!password) {
    loginPasswordInput.classList.add("is-invalid");
    loginPasswordError.textContent = "Password is required";
    return;
  }

  activeLoginEmail = email;
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
      otpEmailTarget.textContent = activeLoginEmail;
      switchSection(otpSection);
      showAlert(
        "success",
        "OTPS Code Sent!",
        "A 6-digit code has been sent to your email.",
      );
    } else {
      showAlert(
        "danger",
        "Login Failed",
        data.message || "Invalid credentials or unverified account.",
      );
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
  if (!otp || otp.length !== 6) {
    otpCodeInput.classList.add("is-invalid");
    otpCodeError.textContent = "Please enter a 6-digit code";
    return;
  }

  otpSubmitBtn.disabled = true;
  otpSpinner.classList.remove("hidden");

  try {
    const { response } = await fetchWithFallback("/api/users/verify-otp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: activeLoginEmail, otp }),
    });

    const text = await response.text();
    let data = {};
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text.trim() };
    }

    if (response.ok && data.token) {
      localStorage.setItem("jwt_token", data.token);
      localStorage.setItem("user_email", activeLoginEmail);
      showDashboard(activeLoginEmail, data.token);
      showAlert("success", "Authenticated!", "2FA verification successful.");
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
      body: JSON.stringify({ email: activeLoginEmail }),
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

async function handleLogout() {
  logoutBtn.disabled = true;
  logoutSpinner.classList.remove("hidden");
  try {
    await fetchWithFallback("/api/users/logout", { method: "POST" });
  } catch (err) {
    console.warn("Logout API warning:", err);
  } finally {
    localStorage.removeItem("jwt_token");
    localStorage.removeItem("user_email");
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

function showDashboard(email, token) {
  userEmailDisplay.textContent = email;
  jwtTokenDisplay.value = token;
  switchSection(dashboardSection);
}

function checkExistingSession() {
  const token = localStorage.getItem("jwt_token");
  const email = localStorage.getItem("user_email");
  if (token && email) {
    showDashboard(email, token);
  }
}
