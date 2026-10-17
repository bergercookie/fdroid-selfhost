"use strict";

const $ = (id) => document.getElementById(id);
const tokenInput = $("token");

// The token only lives for the browser session; it is never stored on disk.
tokenInput.value = sessionStorage.getItem("token") || "";

async function api(path, options = {}) {
  const response = await fetch(path, options);
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || response.statusText);
  }
  return response;
}

function authHeader() {
  return { Authorization: `Bearer ${tokenInput.value}` };
}

function say(text, isError = false) {
  const message = $("message");
  message.textContent = text;
  message.className = isError ? "error" : "";
}

function formatSize(bytes) {
  return bytes < 1048576
    ? `${(bytes / 1024).toFixed(0)} KiB`
    : `${(bytes / 1048576).toFixed(1)} MiB`;
}

async function loadInfo() {
  const info = await (await api("/api/info")).json();
  document.title = info.name;
  $("name").textContent = info.name;
  $("repo-url").textContent = info.repoUrl;
  $("fingerprint").textContent = info.fingerprint;
  $("add-link").href = info.fdroidLink;
  $("version").textContent = info.version;
}

async function loadStatus() {
  const status = await (await api("/api/status")).json();
  const text = {
    pending: "Index not built yet.",
    updating: "Updating index…",
    ok: `Index updated ${new Date(status.lastRun).toLocaleString()}.`,
    error: `Index update failed: ${status.lastError}`,
  };
  $("index-status").textContent = text[status.state] || "";
}

async function loadApks() {
  const apks = await (await api("/api/apks")).json();
  $("empty").hidden = apks.length > 0;
  const rows = apks.map((apk) => {
    const row = document.createElement("tr");
    const link = Object.assign(document.createElement("a"), {
      href: `/repo/${encodeURIComponent(apk.name)}`,
      textContent: apk.name,
    });
    const remove = Object.assign(document.createElement("button"), {
      textContent: "Delete",
      className: "danger",
    });
    remove.addEventListener("click", () => deleteApk(apk.name));
    for (const content of [link, formatSize(apk.size), new Date(apk.modified).toLocaleString(), remove]) {
      row.appendChild(document.createElement("td")).append(content);
    }
    return row;
  });
  $("apks").replaceChildren(...rows);
}

async function refresh() {
  await Promise.all([loadStatus(), loadApks()]);
}

async function deleteApk(name) {
  if (!confirm(`Delete ${name}?`)) return;
  try {
    await api(`/api/apks/${encodeURIComponent(name)}`, { method: "DELETE", headers: authHeader() });
    say(`Deleted ${name}.`);
    await refresh();
  } catch (error) {
    say(error.message, true);
  }
}

$("upload").addEventListener("submit", async (event) => {
  event.preventDefault();
  const file = $("file").files[0];
  sessionStorage.setItem("token", tokenInput.value);
  say(`Uploading ${file.name}…`);
  try {
    await api(`/api/apks/${encodeURIComponent(file.name)}`, { method: "PUT", headers: authHeader(), body: file });
    say(`Uploaded ${file.name}. The index refreshes within a few seconds.`);
    $("upload").reset();
    tokenInput.value = sessionStorage.getItem("token");
    await refresh();
  } catch (error) {
    say(error.message, true);
  }
});

loadInfo().catch((error) => say(error.message, true));
refresh().catch((error) => say(error.message, true));
setInterval(() => refresh().catch(() => {}), 10000);
