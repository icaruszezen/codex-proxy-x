import { isCredentialError, requestCodexAuthURL, requestCodexExchange } from "./api.js";
import { buildAlert, copyToClipboard, escapeHtml } from "./ui.js";

export function createCodexLoginFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onStatsRefresh
}) {
  const state = {
    generating: false,
    exchanging: false,
    currentState: "",
    expiresAt: 0,
    expireTimer: null
  };

  function renderGenerateState(message) {
    els.loginGenerateState.innerHTML = message || "";
  }

  function renderExchangeState(message) {
    els.loginExchangeState.innerHTML = message || "";
  }

  function setExpireHint() {
    if (!state.expiresAt) {
      els.loginStateHint.textContent = "";
      return;
    }
    const remain = Math.max(0, Math.floor((state.expiresAt - Date.now()) / 1000));
    if (remain <= 0) {
      els.loginStateHint.textContent = "链接已过期，请重新生成。";
      clearExpireTimer();
      return;
    }
    const minutes = Math.floor(remain / 60);
    const seconds = remain % 60;
    els.loginStateHint.textContent = `state: ${state.currentState}，剩余 ${minutes} 分 ${seconds.toString().padStart(2, "0")} 秒有效`;
  }

  function clearExpireTimer() {
    if (state.expireTimer) {
      clearInterval(state.expireTimer);
      state.expireTimer = null;
    }
  }

  function startExpireTimer() {
    clearExpireTimer();
    setExpireHint();
    state.expireTimer = setInterval(setExpireHint, 1000);
  }

  function setGenerating(flag) {
    state.generating = flag;
    els.loginGenerateBtn.disabled = flag;
    els.loginGenerateBtn.textContent = flag ? "生成中..." : "生成登录链接";
  }

  function setExchanging(flag) {
    state.exchanging = flag;
    els.loginExchangeBtn.disabled = flag;
    els.loginExchangeBtn.textContent = flag ? "处理中..." : "提交并换取凭证";
  }

  async function handleGenerate() {
    const cred = getCredentials();
    if (!cred?.apiUrl || !cred?.token) {
      renderGenerateState(buildAlert("warn", "请先在右上角『设置接口』中配置 API 地址和 Token"));
      onMissingCredentials?.();
      return;
    }
    if (state.generating) return;
    renderGenerateState("");
    setGenerating(true);
    try {
      const result = await requestCodexAuthURL(cred);
      if (!result.url) {
        throw new Error("后端未返回登录链接");
      }
      els.loginAuthUrl.value = result.url;
      els.loginUrlBlock.classList.remove("hidden");
      state.currentState = result.state;
      state.expiresAt = Date.now() + Math.max(60, result.expiresIn || 600) * 1000;
      startExpireTimer();
      renderGenerateState(buildAlert("success", "已生成登录链接，请复制到浏览器登录"));
    } catch (error) {
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "认证失败");
        return;
      }
      renderGenerateState(buildAlert("error", "生成失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      setGenerating(false);
    }
  }

  async function handleCopy() {
    const url = els.loginAuthUrl.value;
    if (!url) return;
    const ok = await copyToClipboard(url);
    renderGenerateState(buildAlert(ok ? "success" : "warn", ok ? "已复制到剪贴板" : "复制失败，请手动选中复制"));
  }

  function handleOpen() {
    const url = els.loginAuthUrl.value;
    if (!url) return;
    window.open(url, "_blank", "noopener,noreferrer");
  }

  async function handleExchange() {
    const cred = getCredentials();
    if (!cred?.apiUrl || !cred?.token) {
      renderExchangeState(buildAlert("warn", "请先在右上角『设置接口』中配置 API 地址和 Token"));
      onMissingCredentials?.();
      return;
    }
    const callbackUrl = els.loginCallbackInput.value.trim();
    if (!callbackUrl) {
      renderExchangeState(buildAlert("warn", "请粘贴回调地址"));
      return;
    }
    if (state.exchanging) return;
    renderExchangeState("");
    setExchanging(true);
    try {
      const result = await requestCodexExchange(cred, { callbackUrl });
      const lines = [];
      if (result.email) lines.push(`邮箱：${escapeHtml(result.email)}`);
      if (result.accountId) lines.push(`Account ID：${escapeHtml(result.accountId)}`);
      renderExchangeState(buildAlert("success", "登录成功，凭据已保存", lines.join("<br/>")));
      els.loginCallbackInput.value = "";
      els.loginUrlBlock.classList.add("hidden");
      els.loginAuthUrl.value = "";
      state.currentState = "";
      state.expiresAt = 0;
      clearExpireTimer();
      els.loginStateHint.textContent = "";
      try {
        await onStatsRefresh?.();
      } catch (refreshError) {
        // 统计刷新失败不影响登录结果
      }
    } catch (error) {
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "认证失败");
        return;
      }
      renderExchangeState(buildAlert("error", "换取凭证失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      setExchanging(false);
    }
  }

  function init() {
    els.loginGenerateBtn.addEventListener("click", handleGenerate);
    els.loginCopyUrlBtn.addEventListener("click", handleCopy);
    els.loginOpenUrlBtn.addEventListener("click", handleOpen);
    els.loginExchangeBtn.addEventListener("click", handleExchange);
  }

  return { init };
}
