import { isCredentialError, requestApiDebugConfig, requestApiDebugTraces, saveApiDebugConfig } from "./api.js";
import { buildAlert, copyToClipboard, escapeHtml, formatDate } from "./ui.js";

const ROUTE_LABELS = {
  codex: "Codex",
  provider: "Provider",
  codex_then_provider: "Codex→Provider"
};

export function createApiDebugFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  let controller = null;
  let configLoaded = false;
  let debugEnabled = false;
  let displayedTraces = [];

  function setState(html) {
    els.apiDebugState.innerHTML = html || "";
  }

  function setTraces(html) {
    els.apiDebugTraceList.innerHTML = html || "";
  }

  function abort() {
    if (controller) {
      controller.abort();
      controller = null;
    }
  }

  function getCredOrPrompt() {
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      onMissingCredentials?.();
      throw new Error("请先设置管理接口地址");
    }
    return cred;
  }

  function setSubmitting(isSubmitting) {
    els.apiDebugSaveBtn.disabled = isSubmitting;
    els.apiDebugRefreshBtn.disabled = isSubmitting;
    els.apiDebugLoadBtn.disabled = isSubmitting;
    els.apiDebugEnabled.disabled = isSubmitting;
  }

  function applyConfig(config = {}) {
    debugEnabled = Boolean(config.enabled);
    els.apiDebugEnabled.checked = debugEnabled;
    els.apiDebugConfigStatus.textContent = debugEnabled ? "调试已开启" : "调试已关闭";
    updateEmptyHint();
  }

  function updateEmptyHint() {
    if (!debugEnabled) {
      els.apiDebugEmptyHint.textContent = "请先开启调试开关，然后发起推理请求并点击刷新记录。";
      return;
    }
    if (!displayedTraces.length) {
      els.apiDebugEmptyHint.textContent = "暂无 API 记录，发起推理请求后点击「刷新记录」。";
      return;
    }
    els.apiDebugEmptyHint.textContent = "";
  }

  function renderStep(step = {}) {
    const phase = escapeHtml(step.phase || "");
    const name = escapeHtml(step.name || "step");
    const status = step.status_code ? `HTTP ${step.status_code}` : "";
    const account = step.account ? escapeHtml(step.account) : "";
    const url = step.url ? escapeHtml(step.url) : "";
    const note = step.note ? escapeHtml(step.note) : "";
    const body = step.body ? escapeHtml(step.body) : "";
    const truncated = step.truncated ? "（已截断）" : "";
    const headers = step.headers && typeof step.headers === "object"
      ? escapeHtml(JSON.stringify(step.headers, null, 2))
      : "";

    return `
      <details class="debug-step-block"${phase === "error" ? " open" : ""}>
        <summary>
          <span class="debug-step-name">${name}</span>
          <span class="debug-step-phase">${phase}</span>
          ${status ? `<span class="pill debug-step-status">${escapeHtml(status)}</span>` : ""}
        </summary>
        <div class="debug-step-body">
          ${account ? `<p class="debug-step-meta">账号：${account}</p>` : ""}
          ${url ? `<p class="debug-step-meta">URL：${url}</p>` : ""}
          ${note ? `<p class="debug-step-meta">备注：${note}</p>` : ""}
          ${headers ? `<pre class="debug-step-pre">${headers}</pre>` : ""}
          ${body ? `<pre class="debug-step-pre">${body}${truncated}</pre>` : ""}
        </div>
      </details>
    `;
  }

  function renderTrace(trace = {}, index = 0) {
    const route = ROUTE_LABELS[trace.route] || escapeHtml(trace.route || "unknown");
    const success = Boolean(trace.success);
    const steps = Array.isArray(trace.steps) ? trace.steps : [];

    return `
      <article class="debug-trace-item" data-trace-index="${index}">
        <div class="debug-trace-head">
          <div class="debug-trace-main">
            <div class="debug-trace-heading">
              <span class="debug-trace-time">${escapeHtml(formatDate(trace.started_at))}</span>
              <span class="pill">${escapeHtml(trace.method || "POST")} ${escapeHtml(trace.path || "")}</span>
              ${trace.model ? `<span class="pill">${escapeHtml(trace.model)}</span>` : ""}
              <span class="pill">${route}</span>
              <span class="pill ${success ? "debug-pill-success" : "debug-pill-error"}">${success ? "成功" : "失败"}</span>
              ${trace.duration_ms != null ? `<span class="debug-trace-duration">${Number(trace.duration_ms)}ms</span>` : ""}
            </div>
            ${trace.error ? `<p class="debug-trace-error">${escapeHtml(trace.error)}</p>` : ""}
          </div>
          <button type="button" class="secondary debug-copy-btn" data-copy-index="${index}">复制本条</button>
        </div>
        <div class="debug-step-list">
          ${steps.map(renderStep).join("")}
        </div>
      </article>
    `;
  }

  function renderTraceList(traces = []) {
    displayedTraces = Array.isArray(traces) ? traces : [];
    if (!displayedTraces.length) {
      setTraces("");
      updateEmptyHint();
      return;
    }
    els.apiDebugEmptyHint.textContent = "";
    setTraces(displayedTraces.map(renderTrace).join(""));
    els.apiDebugTraceList.querySelectorAll(".debug-copy-btn").forEach(btn => {
      btn.addEventListener("click", async () => {
        const idx = Number(btn.dataset.copyIndex);
        const trace = displayedTraces[idx];
        if (!trace) {
          return;
        }
        const ok = await copyToClipboard(JSON.stringify(trace, null, 2));
        setState(buildAlert(ok ? "success" : "error", ok ? "已复制" : "复制失败", ok ? "该条 API 记录 JSON 已写入剪贴板。" : "请手动选择内容复制。"));
      });
    });
  }

  async function loadConfig() {
    abort();
    controller = new AbortController();
    const cred = getCredOrPrompt();
    setSubmitting(true);
    onLoadingChange?.(true);
    try {
      const config = await requestApiDebugConfig(cred, controller.signal);
      applyConfig(config);
      configLoaded = true;
      setState(buildAlert("success", "配置已加载", debugEnabled ? "调试记录已开启。" : "调试记录已关闭。"));
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      if (isCredentialError(error)) {
        onCredentialError?.(error.message);
        return;
      }
      setState(buildAlert("error", "加载失败", error.message || String(error)));
    } finally {
      setSubmitting(false);
      onLoadingChange?.(false);
      controller = null;
    }
  }

  async function saveConfig() {
    abort();
    controller = new AbortController();
    const cred = getCredOrPrompt();
    setSubmitting(true);
    onLoadingChange?.(true);
    try {
      const config = await saveApiDebugConfig(cred, { enabled: els.apiDebugEnabled.checked }, controller.signal);
      applyConfig(config);
      setState(buildAlert("success", "已保存", config.enabled ? "调试记录已开启，新的推理请求将被记录。" : "调试记录已关闭。"));
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      if (isCredentialError(error)) {
        onCredentialError?.(error.message);
        return;
      }
      setState(buildAlert("error", "保存失败", error.message || String(error)));
    } finally {
      setSubmitting(false);
      onLoadingChange?.(false);
      controller = null;
    }
  }

  async function refreshTraces() {
    if (!debugEnabled) {
      setState(buildAlert("warn", "调试未开启", "请先开启调试开关并保存。"));
      return;
    }
    abort();
    controller = new AbortController();
    const cred = getCredOrPrompt();
    setSubmitting(true);
    onLoadingChange?.(true);
    try {
      const data = await requestApiDebugTraces(cred, controller.signal);
      renderTraceList(data?.traces || []);
      setState(buildAlert("success", "已刷新", `共 ${Number(data?.count ?? displayedTraces.length)} 条记录（最多 20 条）。`));
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      if (isCredentialError(error)) {
        onCredentialError?.(error.message);
        return;
      }
      setState(buildAlert("error", "刷新失败", error.message || String(error)));
    } finally {
      setSubmitting(false);
      onLoadingChange?.(false);
      controller = null;
    }
  }

  function ensureLoaded() {
    if (configLoaded) {
      return Promise.resolve();
    }
    return loadConfig();
  }

  function init() {
    els.apiDebugLoadBtn.addEventListener("click", () => {
      loadConfig().catch(() => {});
    });
    els.apiDebugSaveBtn.addEventListener("click", () => {
      saveConfig().catch(() => {});
    });
    els.apiDebugRefreshBtn.addEventListener("click", () => {
      refreshTraces().catch(() => {});
    });
  }

  return {
    init,
    ensureLoaded,
    abort
  };
}
