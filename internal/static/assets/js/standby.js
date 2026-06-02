import {
  isCredentialError,
  requestStandbyState,
  requestStandbyConfig,
  saveStandbyConfig,
  requestStandbyIngest,
  requestStandbyExport,
  requestStandbyDelete,
  requestStandbyToggleEnabled,
  requestStandbyHealthCheck
} from "./api.js";
import { buildAlert, cooldownStatusText, cooldownStatusTitle, cooldownUntilAttr, ensureCooldownTicker, escapeHtml, formatDate, formatNumber } from "./ui.js";

/**
 * createStandbyFeature 备用账号池视图特性
 *
 * 界面布局（参考 index.html #standbyView）：
 *   - 状态横幅 #standbyStatusBanner：当前是否激活、备用池账号总数、说明
 *   - 操作栏：手动健康检查、刷新、导出选中
 *   - 表格 #standbyTableBody：账号列表
 *   - 导入区 #standbyImportTextarea + #standbyImportSubmitBtn
 */
export function createStandbyFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  const state = {
    initialized: false,
    snapshot: null,
    rows: [],
    selectedEmails: new Set(),
    togglingEmails: new Set(),
    deletingEmails: new Set(),
    isHealthChecking: false,
    isLoading: false,
    isImporting: false,
    isExporting: false,
    isSavingConfig: false,
    fetchController: null,
    configController: null,
    importController: null,
    healthController: null,
    importResult: null
  };

  function notifyLoading(flag) {
    if (typeof onLoadingChange === "function") {
      onLoadingChange(flag);
    }
  }

  function setActionMessage(html = "") {
    if (els.standbyActionState) {
      els.standbyActionState.innerHTML = html;
    }
  }

  function renderBanner(snapshot) {
    if (!els.standbyStatusBanner) return;
    const active = Boolean(snapshot?.active);
    const note = String(snapshot?.note || "");
    const stat = `主池 ${formatNumber(snapshot?.primaryTotal ?? 0)}，备用池 ${formatNumber(snapshot?.standbyTotal ?? 0)}`;
    const cls = active ? "standby-banner standby-banner-active" : "standby-banner";
    const label = active ? "备用账号池：使用中" : "备用账号池：待机";
    els.standbyStatusBanner.className = cls;
    els.standbyStatusBanner.innerHTML = `
      <div class="standby-banner-title">${escapeHtml(label)}</div>
      <div class="standby-banner-meta">${escapeHtml(stat)}${note ? ` · ${escapeHtml(note)}` : ""}</div>
    `;
  }

  function renderSummary(snapshot) {
    if (!els.standbySummary) return;
    const summary = snapshot?.summary || {};
    const cards = [
      { label: "备用池总数", value: snapshot?.standbyTotal ?? 0 },
      { label: "活跃", value: summary.active ?? 0 },
      { label: "冷却中", value: summary.cooldown ?? 0 },
      { label: "禁用", value: summary.disabled ?? 0 },
      { label: "不可刷新", value: summary.refresh_disabled ?? 0 }
    ];
    els.standbySummary.innerHTML = cards.map(card => `
      <div class="card">
        <h3>${escapeHtml(card.label)}</h3>
        <div class="value">${formatNumber(card.value)}</div>
      </div>
    `).join("");
  }

  function normalizeEmail(email) {
    return String(email || "").trim();
  }

  function rowIsBusy(email) {
    const safe = normalizeEmail(email);
    if (!safe) return false;
    return state.togglingEmails.has(safe) || state.deletingEmails.has(safe);
  }

  function buildStatusCell(row) {
    const statusClass = ["active", "cooldown", "disabled"].includes(row?.status) ? row.status : "disabled";
    let statusText = String(row?.status || "--");
    let title = "";
    let dataCooldownUntil = "";
    if (row?.status === "cooldown") {
      statusText = cooldownStatusText(row.cooldown_until);
      title = cooldownStatusTitle(row.cooldown_until);
      dataCooldownUntil = cooldownUntilAttr(row.cooldown_until);
    }
    const dataAttr = dataCooldownUntil ? ` data-cooldown-until="${escapeHtml(dataCooldownUntil)}"` : "";
    const titleAttr = title ? ` title="${escapeHtml(title)}"` : "";
    return `<span class="status ${statusClass}"${dataAttr}${titleAttr}>${escapeHtml(statusText)}</span>`;
  }

  function renderTable(rows) {
    if (!els.standbyTableBody) return;
    if (!rows.length) {
      els.standbyTableBody.innerHTML = `<tr><td colspan="6" class="muted">备用账号池为空</td></tr>`;
      return;
    }
    els.standbyTableBody.innerHTML = rows.map(row => {
      const email = normalizeEmail(row?.email);
      const enabled = String(row?.status || "") !== "disabled";
      const isBusy = rowIsBusy(email);
      const lastUsed = row?.last_used_at ? formatDate(row.last_used_at) : "--";
      const tokenExpire = row?.token_expire ? formatDate(row.token_expire) : (row?.expire ? formatDate(row.expire) : "--");
      const plan = String(row?.plan_type || "--");
      return `
        <tr>
          <td class="select-col">
            <label class="select-toggle-wrap">
              <input
                type="checkbox"
                class="standby-select"
                data-action="standby-select"
                data-email="${encodeURIComponent(email)}"
                ${state.selectedEmails.has(email) ? "checked" : ""}
                ${isBusy || !email ? "disabled" : ""}
              />
            </label>
          </td>
          <td data-label="邮箱">
            <div class="account-cell">
              <span class="account-email">${escapeHtml(email || "(无 email)")}</span>
              <label class="enabled-toggle-wrap">
                <input
                  type="checkbox"
                  class="enabled-toggle"
                  data-action="standby-toggle"
                  data-email="${encodeURIComponent(email)}"
                  ${enabled ? "checked" : ""}
                  ${isBusy ? "disabled" : ""}
                />
                <span>${enabled ? "启用" : "停用"}</span>
              </label>
              <button
                type="button"
                class="ghost row-action danger"
                data-action="standby-delete"
                data-email="${encodeURIComponent(email)}"
                ${isBusy ? "disabled" : ""}
              >${state.deletingEmails.has(email) ? "删除中..." : "删除"}</button>
            </div>
          </td>
          <td data-label="状态">${buildStatusCell(row)}</td>
          <td data-label="套餐">${escapeHtml(plan)}</td>
          <td data-label="最后使用">${escapeHtml(lastUsed)}</td>
          <td data-label="Token 过期">${escapeHtml(tokenExpire)}</td>
        </tr>
      `;
    }).join("");
  }

  function updateConfigControls() {
    if (els.standbyConfigSaveBtn) {
      els.standbyConfigSaveBtn.disabled = state.isSavingConfig;
      els.standbyConfigSaveBtn.textContent = state.isSavingConfig ? "保存中..." : "保存开关";
    }
    if (els.standbyForceGPT55Enabled) {
      els.standbyForceGPT55Enabled.disabled = state.isSavingConfig;
    }
  }

  function applyConfig(config = {}) {
    const enabled = Boolean(config.standby_force_gpt55_enabled ?? config.standbyForceGPT55Enabled);
    if (els.standbyForceGPT55Enabled) {
      els.standbyForceGPT55Enabled.checked = enabled;
    }
    if (els.standbyConfigStatus) {
      els.standbyConfigStatus.textContent = enabled ? "已开启：备用池强制 gpt-5.5" : "未开启";
    }
  }

  async function loadConfig() {
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    if (state.configController) state.configController.abort();
    const controller = new AbortController();
    state.configController = controller;
    try {
      const config = await requestStandbyConfig(cred, controller.signal);
      applyConfig(config);
    } catch (error) {
      if (error?.name === "AbortError") return;
      const message = error?.message || "加载备用池配置失败";
      if (els.standbyConfigState) {
        els.standbyConfigState.innerHTML = buildAlert("error", message);
      }
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      if (state.configController === controller) state.configController = null;
    }
  }

  async function saveConfig() {
    if (state.isSavingConfig) return;
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    const enabled = Boolean(els.standbyForceGPT55Enabled?.checked);
    const controller = new AbortController();
    state.isSavingConfig = true;
    updateConfigControls();
    if (els.standbyConfigState) els.standbyConfigState.innerHTML = "";
    try {
      const saved = await saveStandbyConfig(cred, { standbyForceGPT55Enabled: enabled }, controller.signal);
      applyConfig(saved);
      if (els.standbyConfigState) {
        els.standbyConfigState.innerHTML = buildAlert("success", "备用池模型路由开关已保存", enabled ? "备用池请求将强制使用 gpt-5.5。" : "备用池请求将保留原始模型。");
      }
    } catch (error) {
      const message = error?.message || "保存备用池配置失败";
      if (els.standbyConfigState) {
        els.standbyConfigState.innerHTML = buildAlert("error", "保存失败", escapeHtml(message));
      }
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      state.isSavingConfig = false;
      updateConfigControls();
    }
  }

  function updateExportButton() {
    if (els.standbyExportSelectedBtn) {
      const disabled = state.selectedEmails.size === 0 || state.isExporting;
      els.standbyExportSelectedBtn.disabled = disabled;
      els.standbyExportSelectedBtn.textContent = state.isExporting
        ? "导出中..."
        : `导出选中 (${formatNumber(state.selectedEmails.size)})`;
    }
    if (els.standbyClearSelectionBtn) {
      els.standbyClearSelectionBtn.disabled = state.selectedEmails.size === 0;
    }
  }

  function updateHealthCheckButton() {
    if (els.standbyHealthCheckBtn) {
      els.standbyHealthCheckBtn.disabled = state.isHealthChecking;
      els.standbyHealthCheckBtn.textContent = state.isHealthChecking ? "健康检查中..." : "手动健康检查";
    }
  }

  function refresh(snapshot) {
    state.snapshot = snapshot;
    state.rows = Array.isArray(snapshot?.accounts) ? snapshot.accounts : [];
    /* 清理失效的选择 */
    const validEmails = new Set(state.rows.map(r => normalizeEmail(r.email)).filter(Boolean));
    for (const email of [...state.selectedEmails]) {
      if (!validEmails.has(email)) {
        state.selectedEmails.delete(email);
      }
    }
    renderBanner(snapshot);
    renderSummary(snapshot);
    renderTable(state.rows);
    updateExportButton();
    updateHealthCheckButton();
  }

  async function fetchState({ showLoading = true } = {}) {
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    if (state.fetchController) {
      state.fetchController.abort();
    }
    const controller = new AbortController();
    state.fetchController = controller;
    state.isLoading = true;
    if (showLoading) notifyLoading(true);
    try {
      const snapshot = await requestStandbyState(cred, {}, controller.signal);
      refresh(snapshot);
    } catch (error) {
      if (error?.name === "AbortError") return;
      const message = error?.message || "拉取备用池状态失败";
      setActionMessage(buildAlert("error", message));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      if (state.fetchController === controller) {
        state.fetchController = null;
        state.isLoading = false;
        if (showLoading) notifyLoading(false);
      }
    }
  }

  /* ---- 导入 ---- */

  function setImportSubmitting(flag) {
    state.isImporting = flag;
    if (els.standbyImportSubmitBtn) {
      els.standbyImportSubmitBtn.disabled = flag;
      els.standbyImportSubmitBtn.textContent = flag ? "导入中..." : "提交导入";
    }
    if (els.standbyImportTextarea) {
      els.standbyImportTextarea.disabled = flag;
    }
  }

  function renderImportResult(result) {
    state.importResult = result;
    if (!els.standbyImportState) return;
    if (!result) {
      els.standbyImportState.innerHTML = "";
      return;
    }
    const summary = `新增 ${formatNumber(result.added)} · 更新 ${formatNumber(result.updated)} · 失败 ${formatNumber(result.failed)} · 备用池合计 ${formatNumber(result.poolTotal)}`;
    const errs = (result.errors || []).map(e => `<li>${escapeHtml(String(e))}</li>`).join("");
    const errBlock = errs ? `<ul class="alert-list">${errs}</ul>` : "";
    els.standbyImportState.innerHTML = buildAlert(
      result.failed > 0 && result.added + result.updated === 0 ? "error" : "success",
      "备用池导入完成",
      `${escapeHtml(summary)}${errBlock}`
    );
  }

  async function handleImport() {
    if (state.isImporting) return;
    const raw = (els.standbyImportTextarea?.value || "").trim();
    if (!raw) {
      els.standbyImportState.innerHTML = buildAlert("warning", "请先粘贴账号 JSON / NDJSON 内容");
      return;
    }
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    if (state.importController) state.importController.abort();
    const controller = new AbortController();
    state.importController = controller;
    setImportSubmitting(true);
    els.standbyImportState.innerHTML = "";
    try {
      const submitFormat = raw.startsWith("[") || raw.startsWith("{") ? "json-array" : "ndjson";
      const result = await requestStandbyIngest(cred, { text: raw, submitFormat }, controller.signal);
      renderImportResult(result);
      await fetchState({ showLoading: false });
    } catch (error) {
      if (error?.name === "AbortError") return;
      const message = error?.message || "导入失败";
      els.standbyImportState.innerHTML = buildAlert("error", "备用池导入失败", escapeHtml(message));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      if (state.importController === controller) state.importController = null;
      setImportSubmitting(false);
    }
  }

  /* ---- 选择与导出 ---- */

  function clearSelection() {
    state.selectedEmails.clear();
    updateExportButton();
    renderTable(state.rows);
  }

  function buildExportFilename(format) {
    const now = new Date();
    const stamp = [
      now.getFullYear(),
      String(now.getMonth() + 1).padStart(2, "0"),
      String(now.getDate()).padStart(2, "0"),
      "-",
      String(now.getHours()).padStart(2, "0"),
      String(now.getMinutes()).padStart(2, "0"),
      String(now.getSeconds()).padStart(2, "0")
    ].join("");
    return format === "sub2api-array"
      ? `standby-sub2api-accounts-${stamp}.json`
      : `standby-sub2api-export-${stamp}.json`;
  }

  function downloadExportPayload(data, format) {
    const blob = new Blob([`${JSON.stringify(data, null, 2)}\n`], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = buildExportFilename(format);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  async function handleExport() {
    if (state.isExporting) return;
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    if (state.selectedEmails.size === 0) {
      setActionMessage(buildAlert("warning", "请先勾选要导出的账号"));
      return;
    }
    const format = els.standbyExportFormatSelect?.value || "sub2api-export";
    state.isExporting = true;
    updateExportButton();
    try {
      const result = await requestStandbyExport(cred, {
        emails: [...state.selectedEmails],
        format
      });
      downloadExportPayload(result.data, format);
      const parts = [`已导出 ${formatNumber(result.exported)} 个账号`];
      if (result.notFound.length) parts.push(`未找到 ${result.notFound.length}`);
      if (result.failed.length) parts.push(`失败 ${result.failed.length}`);
      setActionMessage(buildAlert("success", "备用池账号已导出", escapeHtml(parts.join("；"))));
    } catch (error) {
      const message = error?.message || "导出失败";
      setActionMessage(buildAlert("error", "备用池导出失败", escapeHtml(message)));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      state.isExporting = false;
      updateExportButton();
    }
  }

  /* ---- 启停 / 删除 ---- */

  async function handleToggle(email, enabled) {
    const safe = normalizeEmail(email);
    if (!safe) return;
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    state.togglingEmails.add(safe);
    renderTable(state.rows);
    try {
      await requestStandbyToggleEnabled(cred, { email: safe, enabled });
      await fetchState({ showLoading: false });
    } catch (error) {
      const message = error?.message || "切换失败";
      setActionMessage(buildAlert("error", `账号 ${safe} 启停失败`, escapeHtml(message)));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      state.togglingEmails.delete(safe);
      renderTable(state.rows);
    }
  }

  async function handleDelete(email) {
    const safe = normalizeEmail(email);
    if (!safe) return;
    if (!window.confirm(`确定删除备用池账号 ${safe}？此操作不可撤销`)) return;
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    state.deletingEmails.add(safe);
    renderTable(state.rows);
    try {
      await requestStandbyDelete(cred, { email: safe });
      state.selectedEmails.delete(safe);
      await fetchState({ showLoading: false });
    } catch (error) {
      const message = error?.message || "删除失败";
      setActionMessage(buildAlert("error", `账号 ${safe} 删除失败`, escapeHtml(message)));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      state.deletingEmails.delete(safe);
      renderTable(state.rows);
    }
  }

  /* ---- 手动健康检查 ---- */

  async function handleHealthCheck() {
    if (state.isHealthChecking) return;
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      return;
    }
    if (state.healthController) state.healthController.abort();
    const controller = new AbortController();
    state.healthController = controller;
    state.isHealthChecking = true;
    updateHealthCheckButton();
    setActionMessage(buildAlert("info", "手动健康检查已启动，等待进度..."));
    try {
      await requestStandbyHealthCheck(cred, event => {
        if (event?.type === "item") {
          const total = event.total || 0;
          const cur = event.current || 0;
          setActionMessage(buildAlert("info", `健康检查进度 ${cur}/${total}`, escapeHtml(`最近检查：${event.email || ""}`)));
        } else if (event?.type === "done") {
          setActionMessage(buildAlert("success", "备用池健康检查完成", escapeHtml(
            `成功 ${event.success_count ?? 0}，失败 ${event.failed_count ?? 0}，剩余 ${event.remaining ?? 0}，耗时 ${event.duration || ""}`
          )));
        }
      }, controller.signal);
      await fetchState({ showLoading: false });
    } catch (error) {
      if (error?.name === "AbortError") return;
      const message = error?.message || "健康检查失败";
      setActionMessage(buildAlert("error", "备用池健康检查失败", escapeHtml(message)));
      if (isCredentialError(error)) onCredentialError?.(message, error);
    } finally {
      if (state.healthController === controller) state.healthController = null;
      state.isHealthChecking = false;
      updateHealthCheckButton();
    }
  }

  /* ---- 事件绑定 ---- */

  function handleTableClick(event) {
    const target = event.target;
    const button = target.closest("button[data-action]");
    if (button) {
      const action = button.getAttribute("data-action");
      const email = decodeURIComponent(button.getAttribute("data-email") || "");
      if (action === "standby-delete") {
        handleDelete(email);
      }
      return;
    }
  }

  function handleTableChange(event) {
    const target = event.target;
    if (!(target instanceof HTMLInputElement)) return;
    const action = target.getAttribute("data-action");
    const email = decodeURIComponent(target.getAttribute("data-email") || "");
    if (action === "standby-toggle") {
      handleToggle(email, target.checked);
    } else if (action === "standby-select") {
      const safe = normalizeEmail(email);
      if (!safe) return;
      if (target.checked) {
        state.selectedEmails.add(safe);
      } else {
        state.selectedEmails.delete(safe);
      }
      updateExportButton();
    }
  }

  function handleSelectAllChange(event) {
    const target = event.target;
    if (!(target instanceof HTMLInputElement)) return;
    if (target.checked) {
      state.rows.forEach(row => {
        const safe = normalizeEmail(row?.email);
        if (safe) state.selectedEmails.add(safe);
      });
    } else {
      state.selectedEmails.clear();
    }
    updateExportButton();
    renderTable(state.rows);
  }

  function bindEvents() {
    if (els.standbyRefreshBtn) {
      els.standbyRefreshBtn.addEventListener("click", () => {
        void fetchState();
        void loadConfig();
      });
    }
    if (els.standbyConfigSaveBtn) {
      els.standbyConfigSaveBtn.addEventListener("click", () => {
        void saveConfig();
      });
    }
    if (els.standbyForceGPT55Enabled) {
      els.standbyForceGPT55Enabled.addEventListener("change", () => {
        if (els.standbyConfigStatus) {
          els.standbyConfigStatus.textContent = els.standbyForceGPT55Enabled.checked ? "待保存：将开启" : "待保存：将关闭";
        }
      });
    }
    if (els.standbyImportSubmitBtn) {
      els.standbyImportSubmitBtn.addEventListener("click", () => {
        void handleImport();
      });
    }
    if (els.standbyExportSelectedBtn) {
      els.standbyExportSelectedBtn.addEventListener("click", () => {
        void handleExport();
      });
    }
    if (els.standbyClearSelectionBtn) {
      els.standbyClearSelectionBtn.addEventListener("click", () => {
        clearSelection();
      });
    }
    if (els.standbyHealthCheckBtn) {
      els.standbyHealthCheckBtn.addEventListener("click", () => {
        void handleHealthCheck();
      });
    }
    if (els.standbyTableBody) {
      els.standbyTableBody.addEventListener("click", handleTableClick);
      els.standbyTableBody.addEventListener("change", handleTableChange);
    }
    if (els.standbySelectAll) {
      els.standbySelectAll.addEventListener("change", handleSelectAllChange);
    }
  }

  function init() {
    if (state.initialized) return;
    state.initialized = true;
    bindEvents();
    ensureCooldownTicker();
  }

  return {
    init,
    fetchState,
    ensureLoaded: () => {
      const load = loadConfig();
      if (!state.snapshot) {
        return Promise.all([fetchState(), load]).then(() => undefined);
      }
      return load;
    }
  };
}
