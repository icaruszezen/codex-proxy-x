import { isCredentialError, requestAccountToggleEnabled, requestRecoverAuth, requestStatsPage } from "./api.js";
import { buildAlert, escapeHtml, formatDate, formatNumber } from "./ui.js";

const CACHE_KEY = "stats_cache_v2";

function formatDurationText(value) {
  const durationMs = Number(value ?? 0);
  return durationMs > 0 ? `${formatNumber(durationMs)} ms` : "耗时未知";
}

function buildRecoverDescription(result, durationMs, defaultMessage = "") {
  const parts = [];
  if (defaultMessage) {
    parts.push(defaultMessage);
  }
  if (result?.reason_code) {
    parts.push(`原因：${result.reason_code}`);
  }
  if (result?.detail) {
    parts.push(`详情：${result.detail}`);
  }
  if (Number(durationMs ?? 0) > 0) {
    parts.push(`耗时：${formatDurationText(durationMs)}`);
  }
  return escapeHtml(parts.join("；"));
}

function buildSingleRecoverAlert(result, durationMs) {
  const email = result?.email || "该账号";
  switch (String(result?.status || "")) {
    case "refreshed":
      return buildAlert(
        "success",
        `账号 ${email} 已恢复`,
        buildRecoverDescription(result, durationMs, "后端已完成同步刷新，请以刷新后的统计状态为准。")
      );
    case "cooldown_429_quota_ok":
      return buildAlert(
        "info",
        `账号 ${email} 已进入保留恢复状态`,
        buildRecoverDescription(result, durationMs, "后端在恢复过程中应用了 429 相关保留/冷却策略，请以刷新后的统计状态为准。")
      );
    case "skipped_busy":
      return buildAlert(
        "warning",
        `账号 ${email} 暂未执行恢复`,
        buildRecoverDescription(result, durationMs, "该账号可能正在被其他流程刷新，或后端同步恢复并发已满。")
      );
    case "disabled":
      return buildAlert(
        "error",
        `账号 ${email} 已被禁用`,
        buildRecoverDescription(result, durationMs, "后端判断该账号本次恢复未通过，已将账号标记为不可用。")
      );
    case "removed":
      return buildAlert(
        "error",
        `账号 ${email} 已从账号池移除`,
        buildRecoverDescription(result, durationMs, "后端判断该账号无法继续恢复，已将其移出当前账号池。")
      );
    case "invalid_input":
      return buildAlert(
        "error",
        "401 恢复请求无效",
        buildRecoverDescription(result, durationMs, "后端未接受这次恢复请求，请检查账号数据后重试。")
      );
    default:
      return buildAlert(
        "info",
        `账号 ${email} 已返回恢复结果`,
        buildRecoverDescription(result, durationMs)
      );
  }
}

function buildBulkRecoverAlert(response) {
  const results = Array.isArray(response?.results) ? response.results : [];
  const counts = {
    refreshed: 0,
    cooldown_429_quota_ok: 0,
    skipped_busy: 0,
    disabled: 0,
    removed: 0,
    invalid_input: 0,
    other: 0
  };

  for (const item of results) {
    const status = String(item?.status || "");
    if (Object.prototype.hasOwnProperty.call(counts, status)) {
      counts[status] += 1;
    } else {
      counts.other += 1;
    }
  }

  const total = Number(response?.count ?? results.length ?? 0);
  const parts = [];
  if (counts.refreshed) {
    parts.push(`已刷新 ${formatNumber(counts.refreshed)}`);
  }
  if (counts.cooldown_429_quota_ok) {
    parts.push(`保留/冷却 ${formatNumber(counts.cooldown_429_quota_ok)}`);
  }
  if (counts.skipped_busy) {
    parts.push(`跳过 ${formatNumber(counts.skipped_busy)}`);
  }
  if (counts.disabled) {
    parts.push(`禁用 ${formatNumber(counts.disabled)}`);
  }
  if (counts.removed) {
    parts.push(`移除 ${formatNumber(counts.removed)}`);
  }
  if (counts.invalid_input) {
    parts.push(`无效 ${formatNumber(counts.invalid_input)}`);
  }
  if (counts.other) {
    parts.push(`其他状态 ${formatNumber(counts.other)}`);
  }

  const detailParts = [];
  detailParts.push(parts.length ? parts.join("，") : "后端未返回可统计的恢复结果。");
  if (Number(response?.durationMs ?? 0) > 0) {
    detailParts.push(`总耗时：${formatDurationText(response.durationMs)}`);
  }

  let type = "success";
  if (counts.disabled || counts.removed || counts.invalid_input) {
    type = "error";
  } else if (counts.skipped_busy || counts.other) {
    type = "warning";
  } else if (counts.cooldown_429_quota_ok) {
    type = "info";
  }

  return buildAlert(
    type,
    `批量 401 恢复已完成（共 ${formatNumber(total)} 个账号）`,
    escapeHtml(detailParts.join("；"))
  );
}

const EVENT_REASON_LABELS = {
  auth_401: "健康检查返回 401，已自动停用",
  auth_401_no_refresh_token: "401 且缺少 refresh_token，已自动删除",
  auth_401_disabled: "401 恢复失败，已自动停用",
  refresh_failed: "刷新失败，已自动删除",
  health_check_failed: "健康检查失败",
  quota_recheck_failed: "429 恢复复核失败，已自动删除",
  quota_invalid_after_refresh: "刷新后额度校验无效，已自动删除",
  restore_probe_failed: "禁用凭据恢复探测失败，已自动删除",
  refresh_http_429: "刷新接口返回 429",
  quota_http_429: "额度接口返回 429",
  empty_access_token: "access_token 为空，已自动删除",
  missing_refresh_token: "refresh_token 为空，已自动删除"
};

function formatEventActionLabel(action) {
  return String(action || "") === "remove" ? "自动删除" : "自动停用";
}

function formatEventActionClass(action) {
  return String(action || "") === "remove" ? "remove" : "disable";
}

function formatEventReason(reasonCode) {
  const key = String(reasonCode || "").trim();
  if (!key) {
    return "原因未记录";
  }
  return EVENT_REASON_LABELS[key] || key;
}

function formatEventMeta(event) {
  const parts = [];
  const storageMode = String(event?.storage_mode || "").trim().toLowerCase();
  if (storageMode === "db") {
    parts.push("数据库模式");
  } else if (storageMode === "file") {
    parts.push("文件模式");
  }
  const detail = String(event?.detail || "").trim();
  if (detail) {
    parts.push(detail);
  }
  return parts.join("；");
}

export function createStatsFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  const state = {
    cache: null,
    currentRows: [],
    currentPage: 1,
    pageSize: Number(els.pageSize.value),
    statusFilter: String(els.statusFilter?.value || "all").toLowerCase(),
    pagination: null,
    memoryOnlyCache: false,
    searchTimer: 0,
    fetchController: null,
    initialized: false,
    recoveringEmails: new Set(),
    isRecoveringAll: false,
    togglingEmails: new Set()
  };

  function safeGetCacheRaw() {
    try {
      return localStorage.getItem(CACHE_KEY);
    } catch (error) {
      state.memoryOnlyCache = true;
      return null;
    }
  }

  function safeRemoveCache() {
    try {
      localStorage.removeItem(CACHE_KEY);
    } catch (error) {
      state.memoryOnlyCache = true;
    }
  }

  function setCache(data, meta) {
    state.cache = { data, meta };
    state.memoryOnlyCache = false;
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify(state.cache));
    } catch (error) {
      state.memoryOnlyCache = true;
    }
    updateCacheBadge();
  }

  function getCache() {
    if (state.cache) {
      return state.cache;
    }
    const raw = safeGetCacheRaw();
    if (!raw) {
      return null;
    }
    try {
      state.cache = JSON.parse(raw);
      return state.cache;
    } catch (error) {
      return null;
    }
  }

  function clearCache() {
    state.cache = null;
    state.memoryOnlyCache = false;
    safeRemoveCache();
    updateCacheBadge();
  }

  function hasCache() {
    return Boolean(getCache()?.data);
  }

  function setActionState(markup = "") {
    els.statsActionState.innerHTML = markup;
  }

  function updateRecoverAllButton() {
    const hasSingleRecover = state.recoveringEmails.size > 0;
    const hasTogglePending = state.togglingEmails.size > 0;
    els.recoverAllBtn.disabled = state.isRecoveringAll || hasSingleRecover || hasTogglePending;
    els.recoverAllBtn.textContent = state.isRecoveringAll ? "批量恢复中..." : "批量 401 恢复";
  }

  function updateCacheBadge() {
    if (!state.cache) {
      els.cacheState.textContent = "缓存：未加载";
      els.cacheNote.textContent = "";
      els.updatedAt.textContent = "上次更新：--";
      return;
    }
    const cachedAt = state.cache.meta?.cachedAt || "--";
    els.cacheState.textContent = state.memoryOnlyCache ? "缓存：内存" : "缓存：本地";
    els.cacheNote.textContent = "";
    els.updatedAt.textContent = `上次更新：${cachedAt}`;
  }

  function renderSummary(summary, accounts) {
    const safe = summary || {};
    const total = safe.total ?? accounts.length;
    const cards = [
      { label: "总账号", value: total },
      { label: "活跃", value: safe.active ?? 0 },
      { label: "冷却中", value: safe.cooldown ?? 0 },
      { label: "禁用", value: safe.disabled ?? 0 },
      { label: "RPM", value: safe.rpm ?? 0 },
      { label: "总输入 Token", value: safe.total_input_tokens ?? 0 },
      { label: "总输出 Token", value: safe.total_output_tokens ?? 0 }
    ];
    els.summaryCards.innerHTML = cards.map(card => `
      <div class="card">
        <h3>${card.label}</h3>
        <div class="value">${formatNumber(card.value)}</div>
      </div>
    `).join("");
  }

  function renderRecentEvents(events) {
    const rows = Array.isArray(events) ? events : [];
    if (els.recentEventsCount) {
      els.recentEventsCount.textContent = `最近 ${formatNumber(rows.length)} 条`;
    }
    if (!els.recentEventsList || !els.recentEventsEmpty) {
      return;
    }
    if (!rows.length) {
      els.recentEventsList.innerHTML = "";
      els.recentEventsEmpty.classList.remove("hidden");
      return;
    }
    els.recentEventsEmpty.classList.add("hidden");
    els.recentEventsList.innerHTML = rows.map(event => {
      const email = String(event?.email || "").trim() || "未知账号";
      const actionLabel = formatEventActionLabel(event?.action);
      const actionClass = formatEventActionClass(event?.action);
      const reasonText = formatEventReason(event?.reason_code);
      const metaText = formatEventMeta(event);
      return `
        <article class="event-item">
          <div class="event-item-main">
            <div class="event-item-heading">
              <span class="event-action ${actionClass}">${escapeHtml(actionLabel)}</span>
              <strong class="event-email">${escapeHtml(email)}</strong>
            </div>
            <div class="event-reason">${escapeHtml(reasonText)}</div>
            ${metaText ? `<div class="event-meta">${escapeHtml(metaText)}</div>` : ""}
          </div>
          <time class="event-time">${escapeHtml(formatDate(event?.timestamp))}</time>
        </article>
      `;
    }).join("");
  }

  function buildRecoverAction(email) {
    const safeEmail = String(email || "").trim();
    if (!safeEmail) {
      return "";
    }
    const isRecovering = state.isRecoveringAll || state.recoveringEmails.has(safeEmail);
    return `
      <button
        type="button"
        class="ghost row-action"
        data-action="recover-auth"
        data-email="${encodeURIComponent(safeEmail)}"
        ${isRecovering ? "disabled" : ""}
      >${isRecovering ? "恢复中..." : "401恢复"}</button>
    `;
  }

  function buildEnabledToggle(row) {
    const safeEmail = String(row?.email || "").trim();
    if (!safeEmail) {
      return "";
    }
    const isEnabled = String(row?.status || "") !== "disabled";
    const isBusy = state.isRecoveringAll
      || state.recoveringEmails.has(safeEmail)
      || state.togglingEmails.has(safeEmail);
    return `
      <label class="enabled-toggle-wrap">
        <input
          type="checkbox"
          class="enabled-toggle"
          data-action="toggle-enabled"
          data-email="${encodeURIComponent(safeEmail)}"
          ${isEnabled ? "checked" : ""}
          ${isBusy ? "disabled" : ""}
        />
        <span>${isEnabled ? "启用" : "停用"}</span>
      </label>
    `;
  }

  function renderTable(rows, pageMeta) {
    state.currentRows = rows || [];
    state.pagination = pageMeta || null;
    const total = state.pagination?.filtered_total ?? state.currentRows.length;
    const totalPages = Math.max(1, state.pagination?.total_pages ?? 1);
    state.currentPage = Math.min(Math.max(1, state.pagination?.page ?? state.currentPage), totalPages);
    const hasRows = state.currentRows.length > 0;
    const start = hasRows ? ((state.currentPage - 1) * (state.pagination?.page_size ?? state.pageSize)) + 1 : 0;
    const end = hasRows ? start + state.currentRows.length - 1 : 0;
    const fragment = document.createDocumentFragment();
    for (const row of state.currentRows) {
      const tr = document.createElement("tr");
      const usage = row.usage || {};
      const statusClass = ["active", "cooldown", "disabled"].includes(row.status) ? row.status : "disabled";
      const email = String(row.email || "").trim();
      const emailCell = email
        ? `
          <div class="account-cell">
            <span class="account-email">${escapeHtml(email)}</span>
            ${buildEnabledToggle(row)}
            ${buildRecoverAction(email)}
          </div>
        `
        : "<span class=\"muted\">--</span>";
      tr.innerHTML = `
        <td>${emailCell}</td>
        <td><span class="status ${statusClass}">${escapeHtml(row.status || "--")}</span></td>
        <td>${escapeHtml(row.plan_type || "--")}</td>
        <td>${formatNumber(row.total_requests)}</td>
        <td>${formatNumber(row.total_errors)}</td>
        <td>${formatNumber(usage.input_tokens)}</td>
        <td>${formatNumber(usage.output_tokens)}</td>
        <td>${formatDate(row.last_used_at)}</td>
        <td>${formatDate(row.token_expire)}</td>
        <td>${row.quota_exhausted ? "已用尽" : "可用"}</td>
      `;
      fragment.appendChild(tr);
    }
    els.tableBody.innerHTML = "";
    els.tableBody.appendChild(fragment);
    els.pageInfo.textContent = `${state.currentPage} / ${totalPages}`;
    els.rowsInfo.textContent = total === 0
      ? "0 条记录"
      : hasRows
        ? `${formatNumber(total)} 条记录，当前显示 ${formatNumber(start)}-${formatNumber(end)}`
        : `${formatNumber(total)} 条记录，当前页无数据`;
    els.prevBtn.disabled = state.currentPage === 1;
    els.nextBtn.disabled = state.currentPage === totalPages;
    updateRecoverAllButton();
  }

  function showPlaceholder(show) {
    els.placeholder.style.display = show ? "grid" : "none";
    if (show) {
      els.summaryCards.innerHTML = "";
      els.tableBody.innerHTML = "";
      els.rowsInfo.textContent = "0 条记录";
      els.pageInfo.textContent = "1 / 1";
      els.prevBtn.disabled = true;
      els.nextBtn.disabled = true;
      state.pagination = null;
      state.currentRows = [];
      updateRecoverAllButton();
    }
  }

  function render(data) {
    renderSummary(data.summary, data.accounts || []);
    renderRecentEvents(data.recent_events || []);
    renderTable(data.accounts || [], data.pagination || null);
    showPlaceholder(false);
    updateCacheBadge();
  }

  function applyStatsData(data) {
    const cachedAt = new Date().toLocaleString("zh-CN");
    setCache(data, { cachedAt });
    render(data);
  }

  async function requestCurrentStats(cred, signal) {
    return requestStatsPage(cred, {
      page: state.currentPage,
      pageSize: state.pageSize,
      includeQuota: false,
      query: els.searchInput.value.trim(),
      status: state.statusFilter
    }, signal);
  }

  function abort() {
    if (state.fetchController) {
      state.fetchController.abort();
      state.fetchController = null;
    }
  }

  function handleRecoverError(title, error) {
    const message = error?.message || "未知错误";
    if (isCredentialError(error) && typeof onCredentialError === "function") {
      onCredentialError(message, error);
      return;
    }
    setActionState(buildAlert("error", title, escapeHtml(message)));
  }

  async function refreshAfterRecover(options = {}) {
    const { showLoading = true } = options;
    clearCache();
    await fetchStats({ showLoading });
  }

  async function handleRecoverByEmail(email) {
    const safeEmail = String(email || "").trim();
    if (!safeEmail || state.isRecoveringAll || state.recoveringEmails.has(safeEmail)) {
      return;
    }
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials();
      return;
    }
    state.recoveringEmails.add(safeEmail);
    renderTable(state.currentRows, state.pagination);
    try {
      const response = await requestRecoverAuth(cred, { email: safeEmail });
      setActionState(buildSingleRecoverAlert(response.result || { email: safeEmail }, response.durationMs));
      await refreshAfterRecover({ showLoading: false });
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      handleRecoverError(`账号 ${safeEmail} 401 恢复失败`, error);
    } finally {
      state.recoveringEmails.delete(safeEmail);
      renderTable(state.currentRows, state.pagination);
    }
  }

  async function handleToggleEnabled(email, enabled) {
    const safeEmail = String(email || "").trim();
    if (!safeEmail || state.isRecoveringAll || state.recoveringEmails.has(safeEmail) || state.togglingEmails.has(safeEmail)) {
      return;
    }
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials();
      return;
    }
    state.togglingEmails.add(safeEmail);
    renderTable(state.currentRows, state.pagination);
    try {
      const response = await requestAccountToggleEnabled(cred, { email: safeEmail, enabled: Boolean(enabled) });
      setActionState(buildAlert(
        "success",
        `账号 ${response.email || safeEmail} 已${response.enabled ? "启用" : "停用"}`,
        escapeHtml(response.disableReason ? `原因：${response.disableReason}` : "状态已更新。")
      ));
      await refreshAfterRecover({ showLoading: false });
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      handleRecoverError(`账号 ${safeEmail} 状态切换失败`, error);
    } finally {
      state.togglingEmails.delete(safeEmail);
      renderTable(state.currentRows, state.pagination);
    }
  }

  async function handleRecoverAll() {
    if (state.isRecoveringAll || state.recoveringEmails.size > 0 || state.togglingEmails.size > 0) {
      return;
    }
    const confirmed = window.confirm("批量 401 恢复会顺序请求后端处理当前账号池中的全部账号，账号较多时可能耗时较长。确定继续吗？");
    if (!confirmed) {
      return;
    }
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials();
      return;
    }
    state.isRecoveringAll = true;
    updateRecoverAllButton();
    renderTable(state.currentRows, state.pagination);
    setActionState(buildAlert("info", "已开始批量 401 恢复", escapeHtml("正在顺序请求后端恢复全部账号，请稍候。")));
    onLoadingChange(true);
    try {
      const response = await requestRecoverAuth(cred, { all: true });
      setActionState(buildBulkRecoverAlert(response));
      await refreshAfterRecover();
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      handleRecoverError("批量 401 恢复失败", error);
    } finally {
      state.isRecoveringAll = false;
      updateRecoverAllButton();
      onLoadingChange(false);
      renderTable(state.currentRows, state.pagination);
    }
  }

  async function fetchStats(options = {}) {
    const { showLoading = true } = options;
    els.cacheState.textContent = "缓存：加载中...";
    els.cacheNote.textContent = "";
    if (showLoading) {
      onLoadingChange(true);
    }
    abort();
    const controller = new AbortController();
    state.fetchController = controller;
    try {
      const cred = getCredentials();
      if (!cred?.apiUrl) {
        onMissingCredentials();
        return;
      }
      const data = await requestCurrentStats(cred, controller.signal);
      applyStatsData(data);
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      const message = error?.message || "未知错误";
      els.cacheNote.textContent = `加载失败：${message}`;
      if (isCredentialError(error) && typeof onCredentialError === "function") {
        onCredentialError(message, error);
      }
      if (!state.cache) {
        showPlaceholder(true);
      }
    } finally {
      if (state.fetchController === controller) {
        state.fetchController = null;
        if (showLoading) {
          onLoadingChange(false);
        }
      }
    }
  }

  async function validateCredentials(cred, signal) {
    return requestCurrentStats(cred, signal);
  }

  function bindEvents() {
    els.pageSize.addEventListener("change", () => {
      state.pageSize = Number(els.pageSize.value);
      state.currentPage = 1;
      void fetchStats();
    });
    els.prevBtn.addEventListener("click", () => {
      if (state.currentPage <= 1) {
        return;
      }
      state.currentPage -= 1;
      void fetchStats();
    });
    els.nextBtn.addEventListener("click", () => {
      if (state.pagination && state.currentPage >= (state.pagination.total_pages || 1)) {
        return;
      }
      state.currentPage += 1;
      void fetchStats();
    });
    els.refreshBtn.addEventListener("click", () => {
      clearCache();
      showPlaceholder(true);
      void fetchStats();
    });
    els.clearBtn.addEventListener("click", () => {
      clearCache();
      showPlaceholder(true);
    });
    els.recoverAllBtn.addEventListener("click", () => {
      void handleRecoverAll();
    });
    els.searchInput.addEventListener("input", () => {
      window.clearTimeout(state.searchTimer);
      state.searchTimer = window.setTimeout(() => {
        state.currentPage = 1;
        void fetchStats();
      }, 250);
    });
    els.statusFilter.addEventListener("change", () => {
      state.statusFilter = String(els.statusFilter.value || "all").toLowerCase();
      state.currentPage = 1;
      void fetchStats();
    });
    els.tableBody.addEventListener("click", event => {
      const target = event.target;
      if (!(target instanceof HTMLElement)) {
        return;
      }
      const button = target.closest("[data-action='recover-auth']");
      if (!(button instanceof HTMLButtonElement)) {
        return;
      }
      const email = button.dataset.email ? decodeURIComponent(button.dataset.email) : "";
      void handleRecoverByEmail(email);
    });
    els.tableBody.addEventListener("change", event => {
      const target = event.target;
      if (!(target instanceof HTMLElement)) {
        return;
      }
      const input = target.closest("[data-action='toggle-enabled']");
      if (!(input instanceof HTMLInputElement)) {
        return;
      }
      const email = input.dataset.email ? decodeURIComponent(input.dataset.email) : "";
      void handleToggleEnabled(email, input.checked);
    });
  }

  function init() {
    if (state.initialized) {
      return;
    }
    state.initialized = true;
    setActionState("");
    updateRecoverAllButton();
    const existing = getCache();
    if (existing?.data) {
      render(existing.data);
      showPlaceholder(false);
    } else {
      showPlaceholder(true);
      void fetchStats();
    }
    updateCacheBadge();
    bindEvents();
  }

  return {
    init,
    fetchStats,
    clearCache,
    applyStatsData,
    validateCredentials,
    abort,
    hasCache
  };
}
