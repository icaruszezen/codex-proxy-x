export function escapeHtml(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export function formatNumber(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value ?? 0));
}

export function formatDate(value) {
  if (!value || String(value).startsWith("0001")) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return date.toLocaleString("zh-CN");
}

export function formatRemainingDuration(seconds) {
  const total = Math.max(0, Math.floor(Number(seconds) || 0));
  if (total <= 0) {
    return "";
  }
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total - days * 86400) / 3600);
  const minutes = Math.floor((total - days * 86400 - hours * 3600) / 60);
  if (days > 0) {
    return `${days}d${String(hours).padStart(2, "0")}h`;
  }
  if (hours > 0) {
    return `${hours}h${String(minutes).padStart(2, "0")}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${total}s`;
}

export function cooldownRemainingSeconds(until) {
  if (!until || String(until).startsWith("0001")) {
    return 0;
  }
  const resetMs = Date.parse(String(until));
  if (!Number.isFinite(resetMs)) {
    return 0;
  }
  const remaining = Math.ceil((resetMs - Date.now()) / 1000);
  return remaining > 0 ? remaining : 0;
}

export function cooldownStatusText(until) {
  const remaining = cooldownRemainingSeconds(until);
  const formatted = formatRemainingDuration(remaining);
  return formatted ? `冷却 ${formatted}` : "冷却（已到期）";
}

export function cooldownStatusTitle(until) {
  if (!until || String(until).startsWith("0001")) {
    return "";
  }
  const formatted = formatDate(until);
  return formatted === "--" ? "" : `冷却至 ${formatted}`;
}

export function cooldownUntilAttr(until) {
  if (!until || String(until).startsWith("0001")) {
    return "";
  }
  return String(until);
}

let cooldownTickerId = null;

export function updateCooldownDisplays(root = document) {
  const nodes = root.querySelectorAll("[data-cooldown-until]");
  for (const node of nodes) {
    const until = node.getAttribute("data-cooldown-until");
    if (!until) {
      continue;
    }
    node.textContent = cooldownStatusText(until);
  }
}

export function ensureCooldownTicker() {
  if (cooldownTickerId) {
    return;
  }
  cooldownTickerId = window.setInterval(() => {
    if (!document.querySelector("[data-cooldown-until]")) {
      return;
    }
    updateCooldownDisplays(document);
  }, 1000);
}

export function stopCooldownTicker() {
  if (cooldownTickerId) {
    window.clearInterval(cooldownTickerId);
    cooldownTickerId = null;
  }
}

export function buildAlert(type, title, description = "") {
  return `
    <div class="alert ${type}">
      <strong>${escapeHtml(title)}</strong>
      ${description ? `<div>${description}</div>` : ""}
    </div>
  `;
}

export function setLoading(loadingOverlay, isLoading) {
  loadingOverlay.classList.toggle("active", Boolean(isLoading));
}

export async function copyToClipboard(text) {
  const value = String(text ?? "");
  if (!value) {
    return false;
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch (error) {
    /* fallback below */
  }
  try {
    const textarea = document.createElement("textarea");
    textarea.value = value;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.left = "-9999px";
    document.body.appendChild(textarea);
    textarea.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(textarea);
    return ok;
  } catch (error) {
    return false;
  }
}
