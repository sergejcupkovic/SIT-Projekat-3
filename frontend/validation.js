// Client-side input checks. The server repeats every one of them; these only
// give the user quick feedback.
const Validation = (() => {
  const NAME_REGEX = /^\p{L}[\p{L} '\-]*$/u;
  const FOLDER_NAME_REGEX = /^[\p{L}\p{N} ._()\-]+$/u;
  const OTP_REGEX = /^[0-9]{6}$/;

  const MAX_NAME_LEN = 100;
  const MAX_EMAIL_LEN = 254;
  const MAX_PASSWORD_LEN = 72;
  const MAX_FOLDER_NAME_LEN = 100;

  const length = (value) => [...value].length;

  function name(value, label) {
    if (!value) return `${label} is required`;
    if (length(value) > MAX_NAME_LEN)
      return `${label} must not exceed ${MAX_NAME_LEN} characters`;
    if (!NAME_REGEX.test(value))
      return `${label} may only contain letters, spaces, apostrophes and hyphens`;
    return "";
  }

  function email(value, emailRegex) {
    if (!value) return "Email address is required";
    if (value.length > MAX_EMAIL_LEN) return "Email address is too long";
    if (!emailRegex.test(value))
      return "Please enter a valid email address (e.g., user@example.com)";
    return "";
  }

  function password(value) {
    if (!value) return "Password is required";
    if (value.length > MAX_PASSWORD_LEN)
      return `Password must not exceed ${MAX_PASSWORD_LEN} characters`;
    return "";
  }

  function otp(value) {
    if (!OTP_REGEX.test(value)) return "Please enter a 6-digit numeric code";
    return "";
  }

  function folderName(value) {
    if (!value) return "Folder name is required";
    if (length(value) > MAX_FOLDER_NAME_LEN)
      return `Folder name must not exceed ${MAX_FOLDER_NAME_LEN} characters`;
    if (!FOLDER_NAME_REGEX.test(value) || value.replace(/[. ]/g, "") === "")
      return "Folder name may only contain letters, digits, spaces and . _ - ( )";
    return "";
  }

  // for the few places where a value has to go through innerHTML
  function escapeHtml(value) {
    return String(value).replace(
      /[&<>"']/g,
      (c) =>
        ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
          c
        ],
    );
  }

  return { name, email, password, otp, folderName, escapeHtml };
})();
