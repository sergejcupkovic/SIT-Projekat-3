// Client-side input checks. The server repeats every one of them; these only
// give the user quick feedback. Configurable limits come from the server
// (see limits.js); a check whose limit is not known yet is left to the server.
const Validation = (() => {
  const NAME_REGEX = /^\p{L}[\p{L} '\-]*$/u;
  const FOLDER_NAME_REGEX = /^[\p{L}\p{N} ._()\-]+$/u;
  const OTP_REGEX = /^[0-9]{6}$/;

  const MAX_EMAIL_LEN = 254; // RFC 5321
  const MAX_LOGIN_PASSWORD_LEN = 72; // bcrypt cannot hash more

  let limits = {};
  const configure = (value) => {
    limits = value || {};
  };

  const length = (value) => [...value].length;
  const tooLong = (value, max) => Boolean(max) && length(value) > max;

  function name(value, label) {
    if (!value) return `${label} is required`;
    if (tooLong(value, limits.maxNameLength))
      return `${label} must not exceed ${limits.maxNameLength} characters`;
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

  // An existing password, as typed to sign in.
  function password(value) {
    if (!value) return "Password is required";
    if (value.length > MAX_LOGIN_PASSWORD_LEN)
      return `Password must not exceed ${MAX_LOGIN_PASSWORD_LEN} characters`;
    return "";
  }

  // The configured requirements a NEW password has to meet.
  function passwordRules(value) {
    const min = limits.passwordMinLength || 1;
    const bytes = new TextEncoder().encode(value).length; // as the server counts
    return [
      {
        id: "length",
        label: limits.passwordMinLength
          ? `At least ${min} characters`
          : "Password entered",
        met: bytes >= min,
        enabled: true,
      },
      {
        id: "uppercase",
        label: "At least one uppercase letter (A-Z)",
        met: /\p{Lu}/u.test(value),
        enabled: limits.passwordRequireUppercase,
      },
      {
        id: "lowercase",
        label: "At least one lowercase letter (a-z)",
        met: /\p{Ll}/u.test(value),
        enabled: limits.passwordRequireLowercase,
      },
      {
        id: "number",
        label: "At least one number (0-9)",
        met: /[0-9]/.test(value),
        enabled: limits.passwordRequireDigit,
      },
      {
        id: "special",
        label: "At least one special symbol (e.g. !@#$%^&*)",
        met: /[\p{P}\p{S}]/u.test(value),
        enabled: limits.passwordRequireSpecial,
      },
    ].filter((rule) => rule.enabled);
  }

  function newPassword(value) {
    if (!value) return "Password is required";
    const max = limits.passwordMaxLength;
    if (max && new TextEncoder().encode(value).length > max)
      return `Password must not exceed ${max} characters`;
    if (!passwordRules(value).every((rule) => rule.met))
      return "Password does not meet all security requirements listed below";
    return "";
  }

  function otp(value) {
    if (!OTP_REGEX.test(value)) return "Please enter a 6-digit numeric code";
    return "";
  }

  function folderName(value) {
    if (!value) return "Folder name is required";
    if (tooLong(value, limits.maxFolderNameLength))
      return `Folder name must not exceed ${limits.maxFolderNameLength} characters`;
    if (!FOLDER_NAME_REGEX.test(value) || value.replace(/[. ]/g, "") === "")
      return "Folder name may only contain letters, digits, spaces and . _ - ( )";
    return "";
  }

  // Same whitelist as storage-service/filecheck (the server also sniffs the content).
  function fileName(value, size) {
    if (!value) return "Please select a file";
    if (size === 0) return "Cannot upload an empty file (0 bytes)";
    if (tooLong(value, limits.maxFileNameLength))
      return `File name must not exceed ${limits.maxFileNameLength} characters`;
    if (!FOLDER_NAME_REGEX.test(value) || value.replace(/[. ]/g, "") === "")
      return "File name may only contain letters, digits, spaces and . _ - ( )";
    const allowed = limits.allowedFileExtensions;
    const ext = value.slice(value.lastIndexOf(".")).toLowerCase();
    if (allowed?.length && (!value.includes(".") || !allowed.includes(ext)))
      return `This file type is not allowed. Allowed: ${allowed.join(", ")}`;
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
    configure,
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
