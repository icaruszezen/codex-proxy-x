import { isCredentialError, requestStatsPage } from "./api.js";
import { escapeHtml, formatDate, formatNumber } from "./ui.js";

const CACHE_KEY = "stats_cache_v2";

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
    pagination: null,
    memoryOnlyCache: false,
    searchTimer: 0,
    fetchController: null,
    initialized: false
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
      tr.innerHTML = `
        <td>${escapeHtml(row.email || "--")}</td>
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
    }
  }

  function render(data) {
    renderSummary(data.summary, data.accounts || []);
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
      query: els.searchInput.value.trim()
    }, signal);
  }

  function abort() {
    if (state.fetchController) {
      state.fetchController.abort();
      state.fetchController = null;
    }
  }

  async function fetchStats() {
    els.cacheState.textContent = "缓存：加载中...";
    els.cacheNote.textContent = "";
    onLoadingChange(true);
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
        onLoadingChange(false);
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
    els.searchInput.addEventListener("input", () => {
      window.clearTimeout(state.searchTimer);
      state.searchTimer = window.setTimeout(() => {
        state.currentPage = 1;
        void fetchStats();
      }, 250);
    });
  }

  function init() {
    if (state.initialized) {
      return;
    }
    state.initialized = true;
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
