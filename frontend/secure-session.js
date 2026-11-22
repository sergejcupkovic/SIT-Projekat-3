// The session (JWT, e-mail, role) is kept in localStorage only as an AES-GCM
// ciphertext. GCM also authenticates it, so a modified value fails to decrypt
// and is discarded. The key is generated as non-extractable and lives in
// IndexedDB, so its raw bytes are never available to page scripts.
const session = (() => {
  const DB_NAME = "stashr-secure";
  const STORE = "keys";
  const KEY_ID = "session-key";
  const BLOB_KEY = "session_blob";
  const AAD = new TextEncoder().encode("stashr-session-v1");

  let data = null;

  const toB64 = (bytes) => btoa(String.fromCharCode(...new Uint8Array(bytes)));
  const fromB64 = (text) => Uint8Array.from(atob(text), (c) => c.charCodeAt(0));

  function openDb() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, 1);
      req.onupgradeneeded = () => req.result.createObjectStore(STORE);
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  function idb(db, mode, run) {
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE, mode);
      const req = run(tx.objectStore(STORE));
      tx.oncomplete = () => resolve(req.result);
      tx.onerror = () => reject(tx.error);
    });
  }

  async function getKey(create) {
    const db = await openDb();
    let key = await idb(db, "readonly", (s) => s.get(KEY_ID));
    if (!key && create) {
      key = await crypto.subtle.generateKey(
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt", "decrypt"],
      );
      await idb(db, "readwrite", (s) => s.put(key, KEY_ID));
    }
    db.close();
    return key;
  }

  async function encrypt(value) {
    const key = await getKey(true);
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv, additionalData: AAD },
      key,
      new TextEncoder().encode(JSON.stringify(value)),
    );
    return `${toB64(iv)}.${toB64(ciphertext)}`;
  }

  async function decrypt(blob) {
    const key = await getKey(false);
    if (!key) throw new Error("missing key");
    const [iv, ciphertext] = blob.split(".");
    const plain = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: fromB64(iv), additionalData: AAD },
      key,
      fromB64(ciphertext),
    );
    return JSON.parse(new TextDecoder().decode(plain));
  }

  async function load() {
    data = null;
    // older builds kept these in plain text
    localStorage.removeItem("jwt_token");
    localStorage.removeItem("user_email");
    try {
      const blob = localStorage.getItem(BLOB_KEY);
      if (blob) data = await decrypt(blob);
    } catch {
      // tampered, corrupted or the key is gone: treat as logged out
      localStorage.removeItem(BLOB_KEY);
    }
  }

  async function start(value) {
    data = value;
    try {
      localStorage.setItem(BLOB_KEY, await encrypt(value));
    } catch (err) {
      console.warn("Session could not be persisted:", err);
    }
  }

  function end() {
    data = null;
    try {
      localStorage.removeItem(BLOB_KEY);
    } catch {}
  }

  return {
    load,
    start,
    end,
    get token() {
      return data?.token || null;
    },
    get email() {
      return data?.email || null;
    },
    get role() {
      return data?.role || null;
    },
  };
})();
