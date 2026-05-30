import { fetchUpstreamProviderModels, isCredentialError, requestUpstreamProviderConfig, saveUpstreamProviderConfig, testUpstreamProvider } from "./api.js";
import { buildAlert, escapeHtml } from "./ui.js";

export function createUpstreamProviderFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  let controller = null;
  let loaded = false;
  let savedAPIKeyMasked = "";

  function setState(html) {
    els.upstreamProviderState.innerHTML = html || "";
  }

  function setSubmitting(isSubmitting, label = "保存配置") {
    els.upstreamProviderSaveBtn.disabled = isSubmitting;
    els.upstreamProviderFetchModelsBtn.disabled = isSubmitting;
    els.upstreamProviderTestBtn.disabled = isSubmitting;
    els.upstreamProviderLoadBtn.disabled = isSubmitting;
    els.upstreamProviderSaveBtn.textContent = isSubmitting ? label : "保存配置";
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

  function parseModels(value) {
    return String(value || "")
      .split(/[\n,]/)
      .map(item => item.trim())
      .filter(Boolean);
  }

  function applyConfig(config = {}) {
    els.upstreamProviderAutoSwitch.checked = Boolean(config.auto_switch);
    els.upstreamProviderBaseURL.value = String(config.base_url || "");
    els.upstreamProviderAPIKey.value = "";
    savedAPIKeyMasked = String(config.api_key_masked || "");
    els.upstreamProviderAPIKey.placeholder = savedAPIKeyMasked
      ? `已保存：${savedAPIKeyMasked}，留空则不修改`
      : "输入上游 Provider API Key";
    els.upstreamProviderTimeout.value = String(Number(config.timeout_sec ?? 15) || 15);
    const models = Array.isArray(config.models) ? config.models : [];
    els.upstreamProviderModels.value = models.join("\n");
    const configured = Boolean(config.configured);
    const active = Boolean(config.active);
    els.upstreamProviderConfigStatus.textContent = configured
      ? `已配置${config.auto_switch ? "，自动切换已开启" : "，自动切换未开启"}${active ? "，当前使用中" : ""}`
      : "未配置";
  }

  function collectConfig() {
    return {
      autoSwitch: els.upstreamProviderAutoSwitch.checked,
      baseUrl: els.upstreamProviderBaseURL.value.trim(),
      apiKey: els.upstreamProviderAPIKey.value.trim(),
      models: parseModels(els.upstreamProviderModels.value),
      timeoutSec: Number(els.upstreamProviderTimeout.value || 15) || 15
    };
  }

  function validateConfigForSave(config) {
    if (!config.autoSwitch && !config.baseUrl && !config.apiKey && !savedAPIKeyMasked) {
      return "";
    }
    if (!config.baseUrl) return "请填写上游 Provider API 地址。";
    if (!config.apiKey && !savedAPIKeyMasked) return "请填写上游 Provider API Key。";
    return "";
  }

  async function loadConfig() {
    abort();
    const cred = getCredOrPrompt();
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "加载中...");
    setState("");
    try {
      const config = await requestUpstreamProviderConfig(cred, controller.signal);
      applyConfig(config);
      loaded = true;
      setState(buildAlert("success", "配置已加载", "上游 Provider 配置已从服务端读取。"));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", "加载失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setSubmitting(false);
    }
  }

  async function saveConfig() {
    abort();
    const cred = getCredOrPrompt();
    const config = collectConfig();
    const validationError = validateConfigForSave(config);
    if (validationError) {
      setState(buildAlert("error", "保存失败", validationError));
      return;
    }
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "保存中...");
    setState("");
    try {
      const saved = await saveUpstreamProviderConfig(cred, config, controller.signal);
      applyConfig(saved);
      loaded = true;
      setState(buildAlert("success", "保存成功", "配置已写入服务端本地文件，并会立即用于主池无账号时的上游切换。"));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", "保存失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setSubmitting(false);
    }
  }

  async function fetchModels() {
    abort();
    const cred = getCredOrPrompt();
    const config = collectConfig();
    if (!config.baseUrl) {
      setState(buildAlert("error", "拉取失败", "请先填写上游 Provider API 地址。"));
      return;
    }
    if (!config.apiKey && !savedAPIKeyMasked) {
      setState(buildAlert("error", "拉取失败", "请先填写上游 Provider API Key。"));
      return;
    }
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "拉取中...");
    setState("");
    try {
      const data = await fetchUpstreamProviderModels(cred, config, controller.signal);
      const models = Array.isArray(data.models) ? data.models : [];
      if (data.config) {
        applyConfig(data.config);
      }
      els.upstreamProviderModels.value = models.join("\n");
      setState(buildAlert("success", "模型列表已拉取", `共获取 ${escapeHtml(models.length)} 个模型。`));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", "拉取失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setSubmitting(false);
    }
  }

  async function testConnection() {
    abort();
    const cred = getCredOrPrompt();
    const config = collectConfig();
    if (!config.baseUrl) {
      setState(buildAlert("error", "测试失败", "请先填写上游 Provider API 地址。"));
      return;
    }
    if (!config.apiKey && !savedAPIKeyMasked) {
      setState(buildAlert("error", "测试失败", "请先填写上游 Provider API Key。"));
      return;
    }
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "测试中...");
    setState("");
    try {
      const data = await testUpstreamProvider(cred, config, controller.signal);
      const status = data?.result?.status_code ? `HTTP ${escapeHtml(data.result.status_code)}` : "上游 Provider 已返回成功响应。";
      const modelCount = data?.result?.model_count ? `，模型数 ${escapeHtml(data.result.model_count)}` : "";
      setState(buildAlert("success", "测试成功", `${status}${modelCount}`));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", "测试失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setSubmitting(false);
    }
  }

  function init() {
    els.upstreamProviderLoadBtn.addEventListener("click", () => {
      loadConfig().catch(error => {
        setState(buildAlert("error", "加载失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.upstreamProviderSaveBtn.addEventListener("click", () => {
      saveConfig().catch(error => {
        setState(buildAlert("error", "保存失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.upstreamProviderFetchModelsBtn.addEventListener("click", () => {
      fetchModels().catch(error => {
        setState(buildAlert("error", "拉取失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.upstreamProviderTestBtn.addEventListener("click", () => {
      testConnection().catch(error => {
        setState(buildAlert("error", "测试失败", escapeHtml(error?.message || "未知错误")));
      });
    });
  }

  return {
    init,
    loadConfig,
    ensureLoaded: () => {
      if (!loaded) {
        return loadConfig();
      }
      return Promise.resolve();
    },
    abort
  };
}
