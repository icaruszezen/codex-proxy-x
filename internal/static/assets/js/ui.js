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
