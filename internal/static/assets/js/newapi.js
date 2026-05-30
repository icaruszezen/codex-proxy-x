import { isCredentialError, requestNewapiConfig, saveNewapiConfig, testNewapiDisable, testNewapiEnable } from "./api.js";
import { buildAlert, escapeHtml } from "./ui.js";

export function createNewapiFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  let controller = null;
  let loaded = false;
  let savedTokenMasked = "";

  function setState(html) {
    els.newapiState.innerHTML = html || "";
  }

  function setSubmitting(isSubmitting, label = "保存配置") {
    els.newapiSaveBtn.disabled = isSubmitting;
    els.newapiTestEnableBtn.disabled = isSubmitting;
    els.newapiTestDisableBtn.disabled = isSubmitting;
    els.newapiLoadBtn.disabled = isSubmitting;
    els.newapiSaveBtn.textContent = isSubmitting ? label : "保存配置";
  }

  function setTesting(isTesting, action = "") {
    els.newapiSaveBtn.disabled = isTesting;
    els.newapiTestEnableBtn.disabled = isTesting;
    els.newapiTestDisableBtn.disabled = isTesting;
    els.newapiLoadBtn.disabled = isTesting;
    els.newapiTestEnableBtn.textContent = isTesting && action === "enable" ? "启用中..." : "测试启用";
    els.newapiTestDisableBtn.textContent = isTesting && action === "disable" ? "禁用中..." : "测试禁用";
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

  function applyConfig(config = {}) {
    els.newapiAutoSwitch.checked = Boolean(config.auto_switch);
    els.newapiBaseURL.value = String(config.base_url || "");
    els.newapiToken.value = "";
    savedTokenMasked = String(config.token_masked || "");
    els.newapiToken.placeholder = savedTokenMasked ? `已保存：${savedTokenMasked}，留空则不修改` : "输入 NewAPI 管理员令牌";
    els.newapiAdminUserID.value = config.admin_user_id ? String(config.admin_user_id) : "";
    els.newapiChannelID.value = config.channel_id ? String(config.channel_id) : "";
    els.newapiTimeout.value = String(Number(config.timeout_sec ?? 10) || 10);
    const configured = Boolean(config.configured);
    els.newapiConfigStatus.textContent = configured
      ? `已配置${config.auto_switch ? "，自动启停已开启" : "，自动启停未开启"}`
      : "未配置";
  }

  function collectConfig() {
    return {
      autoSwitch: els.newapiAutoSwitch.checked,
      baseUrl: els.newapiBaseURL.value.trim(),
      adminToken: els.newapiToken.value.trim(),
      adminUserId: Number(els.newapiAdminUserID.value || 0) || 0,
      channelId: Number(els.newapiChannelID.value || 0) || 0,
      timeoutSec: Number(els.newapiTimeout.value || 10) || 10
    };
  }

  function validateConfigForSave(config) {
    if (!config.autoSwitch) {
      return "";
    }
    if (!config.baseUrl) return "开启自动禁用/启用时必须填写 NewAPI 项目地址。";
    if (!config.adminToken && !savedTokenMasked) return "开启自动禁用/启用时必须填写管理员令牌。";
    if (config.adminUserId <= 0) return "开启自动禁用/启用时必须填写有效的管理员 ID。";
    if (config.channelId <= 0) return "开启自动禁用/启用时必须填写有效的渠道 ID。";
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
      const config = await requestNewapiConfig(cred, controller.signal);
      applyConfig(config);
      loaded = true;
      setState(buildAlert("success", "配置已加载", "NewAPI 渠道配置已从服务端读取。"));
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
      const saved = await saveNewapiConfig(cred, config, controller.signal);
      applyConfig(saved);
      loaded = true;
      setState(buildAlert("success", "保存成功", "配置已写入服务端本地文件，并立即用于后续备用池自动切换。"));
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

  async function testChannel(action) {
    abort();
    const cred = getCredOrPrompt();
    controller = new AbortController();
    onLoadingChange?.(true);
    setTesting(true, action);
    setState("");
    try {
      const data = action === "enable"
        ? await testNewapiEnable(cred, controller.signal)
        : await testNewapiDisable(cred, controller.signal);
      const status = data?.result?.status_code ? `HTTP ${escapeHtml(data.result.status_code)}` : "NewAPI 已返回成功响应。";
      setState(buildAlert("success", action === "enable" ? "测试启用成功" : "测试禁用成功", status));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", action === "enable" ? "测试启用失败" : "测试禁用失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setTesting(false);
    }
  }

  function init() {
    els.newapiLoadBtn.addEventListener("click", () => {
      loadConfig().catch(error => {
        setState(buildAlert("error", "加载失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.newapiSaveBtn.addEventListener("click", () => {
      saveConfig().catch(error => {
        setState(buildAlert("error", "保存失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.newapiTestEnableBtn.addEventListener("click", () => {
      testChannel("enable").catch(error => {
        setState(buildAlert("error", "测试失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.newapiTestDisableBtn.addEventListener("click", () => {
      testChannel("disable").catch(error => {
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
