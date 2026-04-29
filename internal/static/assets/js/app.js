import { createImportsFeature } from "./imports.js";
import { createCodexLoginFeature } from "./login.js";
import { createStatsFeature } from "./stats.js";
import { setLoading } from "./ui.js";

const CRED_KEY = "stats_credentials_v1";
const VIEW_STATS = "stats";
const VIEW_IMPORT = "import";
const VIEW_LOGIN = "login";

const els = {
  pageSubtitle: document.getElementById("pageSubtitle"),
  statsViewBtn: document.getElementById("statsViewBtn"),
  importViewBtn: document.getElementById("importViewBtn"),
  loginViewBtn: document.getElementById("loginViewBtn"),
  refreshBtn: document.getElementById("refreshBtn"),
  clearBtn: document.getElementById("clearBtn"),
  recoverAllBtn: document.getElementById("recoverAllBtn"),
  settingsBtn: document.getElementById("settingsBtn"),
  cacheState: document.getElementById("cacheState"),
  statsView: document.getElementById("statsView"),
  importView: document.getElementById("importView"),
  loginView: document.getElementById("loginView"),
  loginGenerateBtn: document.getElementById("loginGenerateBtn"),
  loginAuthUrl: document.getElementById("loginAuthUrl"),
  loginUrlBlock: document.getElementById("loginUrlBlock"),
  loginCopyUrlBtn: document.getElementById("loginCopyUrlBtn"),
  loginOpenUrlBtn: document.getElementById("loginOpenUrlBtn"),
  loginStateHint: document.getElementById("loginStateHint"),
  loginGenerateState: document.getElementById("loginGenerateState"),
  loginCallbackInput: document.getElementById("loginCallbackInput"),
  loginExchangeBtn: document.getElementById("loginExchangeBtn"),
  loginExchangeState: document.getElementById("loginExchangeState"),
  backToStatsFromLoginBtn: document.getElementById("backToStatsFromLoginBtn"),
  openSettingsFromLoginBtn: document.getElementById("openSettingsFromLoginBtn"),
  summaryCards: document.getElementById("summaryCards"),
  statsActionState: document.getElementById("statsActionState"),
  tableBody: document.getElementById("tableBody"),
  pageSize: document.getElementById("pageSize"),
  pageInfo: document.getElementById("pageInfo"),
  rowsInfo: document.getElementById("rowsInfo"),
  prevBtn: document.getElementById("prevBtn"),
  nextBtn: document.getElementById("nextBtn"),
  updatedAt: document.getElementById("updatedAt"),
  cacheNote: document.getElementById("cacheNote"),
  searchInput: document.getElementById("searchInput"),
  placeholder: document.getElementById("placeholder"),
  loadingOverlay: document.getElementById("loadingOverlay"),
  loginOverlay: document.getElementById("loginOverlay"),
  apiInput: document.getElementById("apiInput"),
  tokenInput: document.getElementById("tokenInput"),
  loginError: document.getElementById("loginError"),
  saveCredentialsBtn: document.getElementById("saveCredentialsBtn"),
  cancelCredentialsBtn: document.getElementById("cancelCredentialsBtn"),
  backToStatsBtn: document.getElementById("backToStatsBtn"),
  openSettingsFromImportBtn: document.getElementById("openSettingsFromImportBtn"),
  chooseImportFileBtn: document.getElementById("chooseImportFileBtn"),
  importFileInput: document.getElementById("importFileInput"),
  selectedImportFile: document.getElementById("selectedImportFile"),
  clearImportFileBtn: document.getElementById("clearImportFileBtn"),
  importTextarea: document.getElementById("importTextarea"),
  importSubmitBtn: document.getElementById("importSubmitBtn"),
  importValidationState: document.getElementById("importValidationState"),
  importSubmissionState: document.getElementById("importSubmissionState"),
  importResultPanel: document.getElementById("importResultPanel"),
  importAdded: document.getElementById("importAdded"),
  importUpdated: document.getElementById("importUpdated"),
  importFailed: document.getElementById("importFailed"),
  importPoolTotal: document.getElementById("importPoolTotal"),
  importErrorsBlock: document.getElementById("importErrorsBlock"),
  importErrorsList: document.getElementById("importErrorsList"),
  resultToStatsBtn: document.getElementById("resultToStatsBtn")
};

let credentials = null;
let activeView = VIEW_STATS;
let credentialsController = null;

function safeGetItem(key) {
  try {
    return localStorage.getItem(key);
  } catch (error) {
    return null;
  }
}

function safeSetItem(key, value) {
  try {
    localStorage.setItem(key, value);
    return true;
  } catch (error) {
    return false;
  }
}

function getCredentials() {
  if (credentials) {
    return credentials;
  }
  const raw = safeGetItem(CRED_KEY);
  if (!raw) {
    return null;
  }
  try {
    credentials = JSON.parse(raw);
    return credentials;
  } catch (error) {
    return null;
  }
}

function saveCredentials(apiUrl, token) {
  credentials = { apiUrl, token };
  return safeSetItem(CRED_KEY, JSON.stringify(credentials));
}

function setLoginError(message) {
  const text = (message || "").trim();
  els.loginError.textContent = text;
  els.tokenInput.classList.toggle("invalid", Boolean(text));
}

function setLoginSubmitting(isSubmitting) {
  els.saveCredentialsBtn.disabled = isSubmitting;
  els.cancelCredentialsBtn.disabled = isSubmitting;
  els.saveCredentialsBtn.textContent = isSubmitting ? "验证中..." : "保存并加载";
}

function getDefaultApiUrl() {
  const origin = window.location.origin || "";
  if (origin && origin !== "null") {
    return `${origin}/stats`;
  }
  return "http://127.0.0.1/stats";
}

function showLogin(show) {
  els.loginOverlay.classList.toggle("active", show);
  if (show) {
    const existing = getCredentials();
    const fallbackUrl = getDefaultApiUrl();
    els.apiInput.value = existing?.apiUrl || fallbackUrl;
    els.apiInput.placeholder = fallbackUrl;
    els.tokenInput.value = existing?.token || "";
  } else {
    setLoginError("");
    setLoginSubmitting(false);
  }
}

function getViewFromHash() {
  const hash = window.location.hash.toLowerCase();
  if (hash === "#import") return VIEW_IMPORT;
  if (hash === "#login") return VIEW_LOGIN;
  return VIEW_STATS;
}

function updateViewState() {
  const isImport = activeView === VIEW_IMPORT;
  const isLogin = activeView === VIEW_LOGIN;
  const isStats = !isImport && !isLogin;
  els.pageSubtitle.textContent = isImport
    ? "支持 JSON 对象、JSON 数组和 NDJSON 导入"
    : isLogin
      ? "通过 OpenAI OAuth 粘贴回调链接添加新账号"
      : "数据只在点击刷新时更新";
  els.statsView.classList.toggle("hidden", !isStats);
  els.importView.classList.toggle("hidden", !isImport);
  els.loginView.classList.toggle("hidden", !isLogin);
  els.statsViewBtn.classList.toggle("active", isStats);
  els.importViewBtn.classList.toggle("active", isImport);
  els.loginViewBtn.classList.toggle("active", isLogin);
  els.refreshBtn.classList.toggle("hidden", !isStats);
  els.clearBtn.classList.toggle("hidden", !isStats);
  els.recoverAllBtn.classList.toggle("hidden", !isStats);
}

function setView(view, options = {}) {
  const { updateHash = true } = options;
  if (view === VIEW_IMPORT) activeView = VIEW_IMPORT;
  else if (view === VIEW_LOGIN) activeView = VIEW_LOGIN;
  else activeView = VIEW_STATS;
  updateViewState();
  if (updateHash) {
    const targetHash =
      activeView === VIEW_IMPORT ? "#import" : activeView === VIEW_LOGIN ? "#login" : "#stats";
    if (window.location.hash !== targetHash) {
      window.location.hash = targetHash;
    }
  }
}

const statsFeature = createStatsFeature({
  els,
  getCredentials,
  onMissingCredentials: () => {
    setLoginError("");
    showLogin(true);
  },
  onCredentialError: message => {
    setLoginError(message);
    showLogin(true);
  },
  onLoadingChange: isLoading => {
    setLoading(els.loadingOverlay, isLoading);
  }
});

const loginFeature = createCodexLoginFeature({
  els,
  getCredentials,
  onMissingCredentials: () => {
    setLoginError("");
    showLogin(true);
  },
  onCredentialError: message => {
    setLoginError(message);
    showLogin(true);
  },
  onStatsRefresh: async () => {
    statsFeature.clearCache();
    await statsFeature.fetchStats();
  }
});

const importsFeature = createImportsFeature({
  els,
  getCredentials,
  onMissingCredentials: () => {
    setLoginError("");
    showLogin(true);
  },
  onCredentialError: message => {
    setLoginError(message);
    showLogin(true);
  },
  onStatsRefresh: async () => {
    statsFeature.clearCache();
    await statsFeature.fetchStats();
  }
});

function bindGlobalEvents() {
  els.statsViewBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.importViewBtn.addEventListener("click", () => {
    setView(VIEW_IMPORT);
  });
  els.loginViewBtn.addEventListener("click", () => {
    setView(VIEW_LOGIN);
  });
  els.backToStatsBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.backToStatsFromLoginBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.openSettingsFromLoginBtn.addEventListener("click", () => {
    setLoginError("");
    showLogin(true);
  });
  els.resultToStatsBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.settingsBtn.addEventListener("click", () => {
    setLoginError("");
    showLogin(true);
  });
  els.openSettingsFromImportBtn.addEventListener("click", () => {
    setLoginError("");
    showLogin(true);
  });
  window.addEventListener("hashchange", () => {
    setView(getViewFromHash(), { updateHash: false });
  });
  els.saveCredentialsBtn.addEventListener("click", async () => {
    const apiUrl = els.apiInput.value.trim();
    const token = els.tokenInput.value.trim();
    if (!apiUrl) {
      setLoginError("请输入 API 地址");
      return;
    }
    els.cacheNote.textContent = "";
    setLoginError("");
    setLoginSubmitting(true);
    setLoading(els.loadingOverlay, true);
    statsFeature.abort();
    if (credentialsController) {
      credentialsController.abort();
    }
    const controller = new AbortController();
    credentialsController = controller;
    try {
      const data = await statsFeature.validateCredentials({ apiUrl, token }, controller.signal);
      const persisted = saveCredentials(apiUrl, token);
      statsFeature.applyStatsData(data);
      if (!persisted) {
        els.cacheNote.textContent = "浏览器存储不可用，接口配置仅在本次页面会话中生效。";
      }
      showLogin(false);
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      const message = error?.message || "未知错误";
      els.cacheNote.textContent = `加载失败：${message}`;
      setLoginError(message);
    } finally {
      if (credentialsController === controller) {
        credentialsController = null;
        setLoading(els.loadingOverlay, false);
      }
      setLoginSubmitting(false);
    }
  });
  els.cancelCredentialsBtn.addEventListener("click", () => {
    showLogin(false);
  });
}

function init() {
  statsFeature.init();
  importsFeature.init();
  loginFeature.init();
  bindGlobalEvents();
  setView(getViewFromHash(), { updateHash: false });
}

init();
