const Validation = (() => {
  const NAME_REGEX = /^\p{L}[\p{L} '\-]*$/u;
  const FOLDER_NAME_REGEX = /^[\p{L}\p{N} ._()\-]+$/u;
  const OTP_REGEX = /^[0-9]{6}$/;

  const MAX_NAME_LEN = 100;
  const MAX_EMAIL_LEN = 254;
  const MAX_PASSWORD_LEN = 72;
  const MAX_FOLDER_NAME_LEN = 100;
  const MAX_FILE_NAME_LEN = 255;
  const ALLOWED_FILE_EXTENSIONS = [
    ".txt", ".md", ".csv", ".json", ".pdf", ".png", ".jpg", ".jpeg", ".gif",
  ];

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

  // The requirements of a new password (same as the server's ValidatePassword).
  function passwordRules(value) {
    const min = appLimits?.passwordMinLength || 8;
    return [
      { label: `At least ${min} characters`, met: value.length >= min },
      { label: "At least one uppercase letter (A-Z)", met: /\p{Lu}/u.test(value) },
      { label: "At least one lowercase letter (a-z)", met: /\p{Ll}/u.test(value) },
      { label: "At least one number (0-9)", met: /[0-9]/.test(value) },
      { label: "At least one special symbol (e.g. !@#$%^&*)", met: /[\p{P}\p{S}]/u.test(value) },
    ];
  }

  function newPassword(value) {
    const problem = password(value);
    if (problem) return problem;
    return passwordRules(value).every((rule) => rule.met)
      ? ""
      : "Password does not meet all security requirements listed below";
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

  // Same whitelist as storage-service/filecheck (the server also sniffs the content).
  function fileName(value, size) {
    if (!value) return "Please select a file";
    if (size === 0) return "Cannot upload an empty file (0 bytes)";
    if (length(value) > MAX_FILE_NAME_LEN)
      return `File name must not exceed ${MAX_FILE_NAME_LEN} characters`;
    if (!FOLDER_NAME_REGEX.test(value) || value.replace(/[. ]/g, "") === "")
      return "File name may only contain letters, digits, spaces and . _ - ( )";
    const ext = value.slice(value.lastIndexOf(".")).toLowerCase();
    if (!value.includes(".") || !ALLOWED_FILE_EXTENSIONS.includes(ext))
      return `This file type is not allowed. Allowed: ${ALLOWED_FILE_EXTENSIONS.join(", ")}`;
    return "";
  }

  function renameFileName(value, originalName) {
    const baseErr = fileName(value, 1);
    if (baseErr) return baseErr;

    if (originalName) {
      const origExt = originalName.slice(originalName.lastIndexOf(".")).toLowerCase();
      const nextExt = value.slice(value.lastIndexOf(".")).toLowerCase();
      if (originalName.includes(".") && origExt !== nextExt) {
        return `File name must keep the original extension (${origExt})`;
      }
    }
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

  return {
    name,
    email,
    password,
    passwordRules,
    newPassword,
    otp,
    folderName,
    fileName,
    renameFileName,
    escapeHtml,
  };
})();
