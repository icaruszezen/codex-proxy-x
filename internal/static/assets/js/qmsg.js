import { isCredentialError, requestQmsgConfig, saveQmsgConfig, testQmsgChannel } from "./api.js";
import { buildAlert, escapeHtml } from "./ui.js";

const DEFAULT_MESSAGE_TEMPLATE = "账号自动{{action}}通知\n邮箱：{{email}}\n原因：{{reason_code}}\n详情：{{detail}}\n存储：{{storage_mode}}\n时间：{{timestamp}}";

export function createQmsgFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onLoadingChange
}) {
  let controller = null;
  let loaded = false;
  let savedKeyMasked = "";

  function setState(html) {
    els.qmsgState.innerHTML = html || "";
  }

  function setSubmitting(isSubmitting, label = "保存配置") {
    els.qmsgSaveBtn.disabled = isSubmitting;
    els.qmsgTestBtn.disabled = isSubmitting;
    els.qmsgLoadBtn.disabled = isSubmitting;
    els.qmsgSaveBtn.textContent = isSubmitting ? label : "保存配置";
  }

  function setTesting(isTesting) {
    els.qmsgSaveBtn.disabled = isTesting;
    els.qmsgTestBtn.disabled = isTesting;
    els.qmsgLoadBtn.disabled = isTesting;
    els.qmsgTestBtn.textContent = isTesting ? "测试中..." : "测试推送";
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
    els.qmsgEnabled.checked = Boolean(config.enabled);
    els.qmsgKey.value = "";
    savedKeyMasked = String(config.key_masked || "");
    els.qmsgKey.placeholder = savedKeyMasked ? `已保存：${savedKeyMasked}，留空则不修改` : "输入 qmsg KEY";
    els.qmsgQQ.value = String(config.qq || "");
    els.qmsgBot.value = String(config.bot || "");
    els.qmsgTimeout.value = String(Number(config.timeout_sec ?? 10) || 10);
    els.qmsgTemplate.value = String(config.message_template || DEFAULT_MESSAGE_TEMPLATE);
    const configured = Boolean(config.configured);
    els.qmsgConfigStatus.textContent = configured
      ? `已配置${config.enabled ? "，推送已启用" : "，推送未启用"}`
      : "未配置";
  }

  function collectConfig() {
    return {
      enabled: els.qmsgEnabled.checked,
      key: els.qmsgKey.value.trim(),
      qq: els.qmsgQQ.value.trim(),
      bot: els.qmsgBot.value.trim(),
      timeoutSec: Number(els.qmsgTimeout.value || 10) || 10,
      messageTemplate: els.qmsgTemplate.value.trim() || DEFAULT_MESSAGE_TEMPLATE
    };
  }

  async function loadConfig() {
    abort();
    const cred = getCredOrPrompt();
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "加载中...");
    setState("");
    try {
      const config = await requestQmsgConfig(cred, controller.signal);
      applyConfig(config);
      loaded = true;
      setState(buildAlert("success", "配置已加载", "qmsg 运行时配置已从服务端读取。"));
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
    if (config.enabled && !config.key && !savedKeyMasked) {
      setState(buildAlert("error", "保存失败", "启用 qmsg 时必须填写 KEY。"));
      return;
    }
    controller = new AbortController();
    onLoadingChange?.(true);
    setSubmitting(true, "保存中...");
    setState("");
    try {
      const saved = await saveQmsgConfig(cred, config, controller.signal);
      applyConfig(saved);
      loaded = true;
      setState(buildAlert("success", "保存成功", "配置已写入服务端本地文件，并立即用于后续自动删除/停用通知。"));
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

  async function testChannel() {
    abort();
    const cred = getCredOrPrompt();
    controller = new AbortController();
    onLoadingChange?.(true);
    setTesting(true);
    setState("");
    try {
      const message = els.qmsgTestMessage.value.trim();
      const data = await testQmsgChannel(cred, message, controller.signal);
      const result = data?.result || {};
      const msgID = result.msg_id ? `消息 ID：${escapeHtml(result.msg_id)}` : "qmsg 已返回成功响应。";
      setState(buildAlert("success", "测试推送成功", msgID));
    } catch (error) {
      if (error?.name === "AbortError") return;
      if (isCredentialError(error)) {
        onCredentialError?.(error.message || "接口鉴权失败，请重新设置 Token");
        return;
      }
      setState(buildAlert("error", "测试推送失败", escapeHtml(error?.message || "未知错误")));
    } finally {
      controller = null;
      onLoadingChange?.(false);
      setTesting(false);
    }
  }

  function init() {
    els.qmsgLoadBtn.addEventListener("click", () => {
      loadConfig().catch(error => {
        setState(buildAlert("error", "加载失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.qmsgSaveBtn.addEventListener("click", () => {
      saveConfig().catch(error => {
        setState(buildAlert("error", "保存失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.qmsgTestBtn.addEventListener("click", () => {
      testChannel().catch(error => {
        setState(buildAlert("error", "测试失败", escapeHtml(error?.message || "未知错误")));
      });
    });
    els.qmsgResetTemplateBtn.addEventListener("click", () => {
      els.qmsgTemplate.value = DEFAULT_MESSAGE_TEMPLATE;
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
