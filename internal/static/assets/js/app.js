import { createApiDebugFeature } from "./api-debug.js";
import { createImportsFeature } from "./imports.js";
import { createCodexLoginFeature } from "./login.js";
import { createNewapiFeature } from "./newapi.js";
import { createQmsgFeature } from "./qmsg.js";
import { createStandbyFeature } from "./standby.js";
import { createStatsFeature } from "./stats.js";
import { createUpstreamProviderFeature } from "./upstream-provider.js";
import { setLoading } from "./ui.js";

const CRED_KEY = "stats_credentials_v1";
const VIEW_STATS = "stats";
const VIEW_EVENTS = "events";
const VIEW_IMPORT = "import";
const VIEW_LOGIN = "login";
const VIEW_QMSG = "qmsg";
const VIEW_STANDBY = "standby";
const VIEW_NEWAPI = "newapi";
const VIEW_UPSTREAM_PROVIDER = "upstream-provider";
const VIEW_API_DEBUG = "api-debug";

const els = {
  pageSubtitle: document.getElementById("pageSubtitle"),
  statsViewBtn: document.getElementById("statsViewBtn"),
  eventsViewBtn: document.getElementById("eventsViewBtn"),
  importViewBtn: document.getElementById("importViewBtn"),
  standbyViewBtn: document.getElementById("standbyViewBtn"),
  loginViewBtn: document.getElementById("loginViewBtn"),
  qmsgViewBtn: document.getElementById("qmsgViewBtn"),
  newapiViewBtn: document.getElementById("newapiViewBtn"),
  upstreamProviderViewBtn: document.getElementById("upstreamProviderViewBtn"),
  apiDebugViewBtn: document.getElementById("apiDebugViewBtn"),
  refreshBtn: document.getElementById("refreshBtn"),
  quotaCheckBtn: document.getElementById("quotaCheckBtn"),
  clearBtn: document.getElementById("clearBtn"),
  recoverAllBtn: document.getElementById("recoverAllBtn"),
  settingsBtn: document.getElementById("settingsBtn"),
  cacheState: document.getElementById("cacheState"),
  statsView: document.getElementById("statsView"),
  eventsView: document.getElementById("eventsView"),
  importView: document.getElementById("importView"),
  standbyView: document.getElementById("standbyView"),
  loginView: document.getElementById("loginView"),
  qmsgView: document.getElementById("qmsgView"),
  newapiView: document.getElementById("newapiView"),
  upstreamProviderView: document.getElementById("upstreamProviderView"),
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
  backToStatsFromEventsBtn: document.getElementById("backToStatsFromEventsBtn"),
  openSettingsFromLoginBtn: document.getElementById("openSettingsFromLoginBtn"),
  summaryCards: document.getElementById("summaryCards"),
  statsActionState: document.getElementById("statsActionState"),
  recentEventsList: document.getElementById("recentEventsList"),
  recentEventsEmpty: document.getElementById("recentEventsEmpty"),
  recentEventsCount: document.getElementById("recentEventsCount"),
  tableBody: document.getElementById("tableBody"),
  pageSize: document.getElementById("pageSize"),
  pageInfo: document.getElementById("pageInfo"),
  rowsInfo: document.getElementById("rowsInfo"),
  exportFormatSelect: document.getElementById("exportFormatSelect"),
  exportSelectedBtn: document.getElementById("exportSelectedBtn"),
  clearSelectionBtn: document.getElementById("clearSelectionBtn"),
  selectAllAccounts: document.getElementById("selectAllAccounts"),
  prevBtn: document.getElementById("prevBtn"),
  nextBtn: document.getElementById("nextBtn"),
  updatedAt: document.getElementById("updatedAt"),
  cacheNote: document.getElementById("cacheNote"),
  searchInput: document.getElementById("searchInput"),
  statusFilter: document.getElementById("statusFilter"),
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
  resultToStatsBtn: document.getElementById("resultToStatsBtn"),
  backToStatsFromQmsgBtn: document.getElementById("backToStatsFromQmsgBtn"),
  openSettingsFromQmsgBtn: document.getElementById("openSettingsFromQmsgBtn"),
  qmsgConfigStatus: document.getElementById("qmsgConfigStatus"),
  qmsgLoadBtn: document.getElementById("qmsgLoadBtn"),
  qmsgEnabled: document.getElementById("qmsgEnabled"),
  qmsgKey: document.getElementById("qmsgKey"),
  qmsgQQ: document.getElementById("qmsgQQ"),
  qmsgBot: document.getElementById("qmsgBot"),
  qmsgTimeout: document.getElementById("qmsgTimeout"),
  qmsgTemplate: document.getElementById("qmsgTemplate"),
  qmsgResetTemplateBtn: document.getElementById("qmsgResetTemplateBtn"),
  qmsgTestMessage: document.getElementById("qmsgTestMessage"),
  qmsgSaveBtn: document.getElementById("qmsgSaveBtn"),
  qmsgTestBtn: document.getElementById("qmsgTestBtn"),
  qmsgState: document.getElementById("qmsgState"),
  backToStatsFromNewapiBtn: document.getElementById("backToStatsFromNewapiBtn"),
  openSettingsFromNewapiBtn: document.getElementById("openSettingsFromNewapiBtn"),
  newapiConfigStatus: document.getElementById("newapiConfigStatus"),
  newapiLoadBtn: document.getElementById("newapiLoadBtn"),
  newapiAutoSwitch: document.getElementById("newapiAutoSwitch"),
  newapiBaseURL: document.getElementById("newapiBaseURL"),
  newapiToken: document.getElementById("newapiToken"),
  newapiAdminUserID: document.getElementById("newapiAdminUserID"),
  newapiChannelID: document.getElementById("newapiChannelID"),
  newapiTimeout: document.getElementById("newapiTimeout"),
  newapiSaveBtn: document.getElementById("newapiSaveBtn"),
  newapiTestEnableBtn: document.getElementById("newapiTestEnableBtn"),
  newapiTestDisableBtn: document.getElementById("newapiTestDisableBtn"),
  newapiState: document.getElementById("newapiState"),
  backToStatsFromUpstreamProviderBtn: document.getElementById("backToStatsFromUpstreamProviderBtn"),
  openSettingsFromUpstreamProviderBtn: document.getElementById("openSettingsFromUpstreamProviderBtn"),
  upstreamProviderConfigStatus: document.getElementById("upstreamProviderConfigStatus"),
  upstreamProviderLoadBtn: document.getElementById("upstreamProviderLoadBtn"),
  upstreamProviderAutoSwitch: document.getElementById("upstreamProviderAutoSwitch"),
  upstreamProviderBaseURL: document.getElementById("upstreamProviderBaseURL"),
  upstreamProviderAPIKey: document.getElementById("upstreamProviderAPIKey"),
  upstreamProviderTimeout: document.getElementById("upstreamProviderTimeout"),
  upstreamProviderModels: document.getElementById("upstreamProviderModels"),
  upstreamProviderSaveBtn: document.getElementById("upstreamProviderSaveBtn"),
  upstreamProviderFetchModelsBtn: document.getElementById("upstreamProviderFetchModelsBtn"),
  upstreamProviderTestBtn: document.getElementById("upstreamProviderTestBtn"),
  upstreamProviderState: document.getElementById("upstreamProviderState"),
  apiDebugView: document.getElementById("apiDebugView"),
  apiDebugConfigStatus: document.getElementById("apiDebugConfigStatus"),
  apiDebugLoadBtn: document.getElementById("apiDebugLoadBtn"),
  apiDebugEnabled: document.getElementById("apiDebugEnabled"),
  apiDebugSaveBtn: document.getElementById("apiDebugSaveBtn"),
  apiDebugRefreshBtn: document.getElementById("apiDebugRefreshBtn"),
  apiDebugState: document.getElementById("apiDebugState"),
  apiDebugTraceList: document.getElementById("apiDebugTraceList"),
  apiDebugEmptyHint: document.getElementById("apiDebugEmptyHint"),
  backToStatsFromApiDebugBtn: document.getElementById("backToStatsFromApiDebugBtn"),
  openSettingsFromApiDebugBtn: document.getElementById("openSettingsFromApiDebugBtn"),
  standbyStatusBanner: document.getElementById("standbyStatusBanner"),
  standbyConfigStatus: document.getElementById("standbyConfigStatus"),
  standbyForceGPT55Enabled: document.getElementById("standbyForceGPT55Enabled"),
  standbyConfigSaveBtn: document.getElementById("standbyConfigSaveBtn"),
  standbyConfigState: document.getElementById("standbyConfigState"),
  standbySummary: document.getElementById("standbySummary"),
  standbyActionState: document.getElementById("standbyActionState"),
  standbyTableBody: document.getElementById("standbyTableBody"),
  standbySelectAll: document.getElementById("standbySelectAll"),
  standbyExportFormatSelect: document.getElementById("standbyExportFormatSelect"),
  standbyExportSelectedBtn: document.getElementById("standbyExportSelectedBtn"),
  standbyClearSelectionBtn: document.getElementById("standbyClearSelectionBtn"),
  standbyHealthCheckBtn: document.getElementById("standbyHealthCheckBtn"),
  standbyRefreshBtn: document.getElementById("standbyRefreshBtn"),
  standbyImportTextarea: document.getElementById("standbyImportTextarea"),
  standbyImportSubmitBtn: document.getElementById("standbyImportSubmitBtn"),
  standbyImportState: document.getElementById("standbyImportState"),
  backToStatsFromStandbyBtn: document.getElementById("backToStatsFromStandbyBtn")
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
  if (hash === "#events") return VIEW_EVENTS;
  if (hash === "#import") return VIEW_IMPORT;
  if (hash === "#login") return VIEW_LOGIN;
  if (hash === "#qmsg") return VIEW_QMSG;
  if (hash === "#standby") return VIEW_STANDBY;
  if (hash === "#newapi") return VIEW_NEWAPI;
  if (hash === "#upstream-provider" || hash === "#provider") return VIEW_UPSTREAM_PROVIDER;
  if (hash === "#api-debug") return VIEW_API_DEBUG;
  return VIEW_STATS;
}

function updateViewState() {
  const isEvents = activeView === VIEW_EVENTS;
  const isImport = activeView === VIEW_IMPORT;
  const isLogin = activeView === VIEW_LOGIN;
  const isQmsg = activeView === VIEW_QMSG;
  const isStandby = activeView === VIEW_STANDBY;
  const isNewapi = activeView === VIEW_NEWAPI;
  const isUpstreamProvider = activeView === VIEW_UPSTREAM_PROVIDER;
  const isApiDebug = activeView === VIEW_API_DEBUG;
  const isStats = !isEvents && !isImport && !isLogin && !isQmsg && !isStandby && !isNewapi && !isUpstreamProvider && !isApiDebug;
  els.pageSubtitle.textContent = isEvents
    ? "查看最近自动删除或自动停用的账号事件"
    : isImport
      ? "支持 JSON、NDJSON 与 sub2api 多账号导入"
      : isLogin
        ? "通过 OpenAI OAuth 粘贴回调链接添加新账号"
        : isQmsg
          ? "配置账号自动删除/停用时的 qmsg 私聊通知"
          : isStandby
            ? "主池失效时自动回退的备用账号池"
            : isNewapi
              ? "配置备用池切换时联动的 NewAPI 渠道状态"
              : isUpstreamProvider
                ? "配置主池无账号时优先切换的上游 API 提供商"
                : isApiDebug
                  ? "查看最近 20 条推理 API 请求调试记录（手动刷新）"
                : "数据只在点击刷新时更新";
  els.statsView.classList.toggle("hidden", !isStats);
  els.eventsView.classList.toggle("hidden", !isEvents);
  els.importView.classList.toggle("hidden", !isImport);
  els.loginView.classList.toggle("hidden", !isLogin);
  els.qmsgView.classList.toggle("hidden", !isQmsg);
  if (els.standbyView) els.standbyView.classList.toggle("hidden", !isStandby);
  if (els.newapiView) els.newapiView.classList.toggle("hidden", !isNewapi);
  if (els.upstreamProviderView) els.upstreamProviderView.classList.toggle("hidden", !isUpstreamProvider);
  if (els.apiDebugView) els.apiDebugView.classList.toggle("hidden", !isApiDebug);
  els.statsViewBtn.classList.toggle("active", isStats);
  els.eventsViewBtn.classList.toggle("active", isEvents);
  els.importViewBtn.classList.toggle("active", isImport);
  els.loginViewBtn.classList.toggle("active", isLogin);
  els.qmsgViewBtn.classList.toggle("active", isQmsg);
  if (els.standbyViewBtn) els.standbyViewBtn.classList.toggle("active", isStandby);
  if (els.newapiViewBtn) els.newapiViewBtn.classList.toggle("active", isNewapi);
  if (els.upstreamProviderViewBtn) els.upstreamProviderViewBtn.classList.toggle("active", isUpstreamProvider);
  if (els.apiDebugViewBtn) els.apiDebugViewBtn.classList.toggle("active", isApiDebug);
  els.refreshBtn.classList.toggle("hidden", !isStats);
  els.clearBtn.classList.toggle("hidden", !isStats);
  els.recoverAllBtn.classList.toggle("hidden", !isStats);
}

function setView(view, options = {}) {
  const { updateHash = true } = options;
  if (view === VIEW_EVENTS) activeView = VIEW_EVENTS;
  else if (view === VIEW_IMPORT) activeView = VIEW_IMPORT;
  else if (view === VIEW_LOGIN) activeView = VIEW_LOGIN;
  else if (view === VIEW_QMSG) activeView = VIEW_QMSG;
  else if (view === VIEW_STANDBY) activeView = VIEW_STANDBY;
  else if (view === VIEW_NEWAPI) activeView = VIEW_NEWAPI;
  else if (view === VIEW_UPSTREAM_PROVIDER) activeView = VIEW_UPSTREAM_PROVIDER;
  else if (view === VIEW_API_DEBUG) activeView = VIEW_API_DEBUG;
  else activeView = VIEW_STATS;
  updateViewState();
  if (updateHash) {
    const targetHash = activeView === VIEW_EVENTS
      ? "#events"
      : activeView === VIEW_IMPORT
        ? "#import"
        : activeView === VIEW_LOGIN
          ? "#login"
          : activeView === VIEW_QMSG
            ? "#qmsg"
            : activeView === VIEW_STANDBY
              ? "#standby"
              : activeView === VIEW_NEWAPI
                ? "#newapi"
                : activeView === VIEW_UPSTREAM_PROVIDER
                  ? "#upstream-provider"
                  : activeView === VIEW_API_DEBUG
                    ? "#api-debug"
                : "#stats";
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

const qmsgFeature = createQmsgFeature({
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

const newapiFeature = createNewapiFeature({
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

const standbyFeature = createStandbyFeature({
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

const upstreamProviderFeature = createUpstreamProviderFeature({
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

const apiDebugFeature = createApiDebugFeature({
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

function bindGlobalEvents() {
  els.statsViewBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.eventsViewBtn.addEventListener("click", () => {
    setView(VIEW_EVENTS);
  });
  els.importViewBtn.addEventListener("click", () => {
    setView(VIEW_IMPORT);
  });
  els.loginViewBtn.addEventListener("click", () => {
    setView(VIEW_LOGIN);
  });
  els.qmsgViewBtn.addEventListener("click", () => {
    setView(VIEW_QMSG);
    qmsgFeature.ensureLoaded().catch(() => {});
  });
  if (els.newapiViewBtn) {
    els.newapiViewBtn.addEventListener("click", () => {
      setView(VIEW_NEWAPI);
      newapiFeature.ensureLoaded().catch(() => {});
    });
  }
  if (els.upstreamProviderViewBtn) {
    els.upstreamProviderViewBtn.addEventListener("click", () => {
      setView(VIEW_UPSTREAM_PROVIDER);
      upstreamProviderFeature.ensureLoaded().catch(() => {});
    });
  }
  if (els.apiDebugViewBtn) {
    els.apiDebugViewBtn.addEventListener("click", () => {
      setView(VIEW_API_DEBUG);
      apiDebugFeature.ensureLoaded().catch(() => {});
    });
  }
  if (els.standbyViewBtn) {
    els.standbyViewBtn.addEventListener("click", () => {
      setView(VIEW_STANDBY);
      standbyFeature.ensureLoaded().catch(() => {});
    });
  }
  if (els.backToStatsFromStandbyBtn) {
    els.backToStatsFromStandbyBtn.addEventListener("click", () => {
      setView(VIEW_STATS);
    });
  }
  els.backToStatsBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.backToStatsFromEventsBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.backToStatsFromLoginBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  els.backToStatsFromQmsgBtn.addEventListener("click", () => {
    setView(VIEW_STATS);
  });
  if (els.backToStatsFromNewapiBtn) {
    els.backToStatsFromNewapiBtn.addEventListener("click", () => {
      setView(VIEW_STATS);
    });
  }
  if (els.backToStatsFromUpstreamProviderBtn) {
    els.backToStatsFromUpstreamProviderBtn.addEventListener("click", () => {
      setView(VIEW_STATS);
    });
  }
  if (els.backToStatsFromApiDebugBtn) {
    els.backToStatsFromApiDebugBtn.addEventListener("click", () => {
      setView(VIEW_STATS);
    });
  }
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
  els.openSettingsFromQmsgBtn.addEventListener("click", () => {
    setLoginError("");
    showLogin(true);
  });
  if (els.openSettingsFromNewapiBtn) {
    els.openSettingsFromNewapiBtn.addEventListener("click", () => {
      setLoginError("");
      showLogin(true);
    });
  }
  if (els.openSettingsFromUpstreamProviderBtn) {
    els.openSettingsFromUpstreamProviderBtn.addEventListener("click", () => {
      setLoginError("");
      showLogin(true);
    });
  }
  if (els.openSettingsFromApiDebugBtn) {
    els.openSettingsFromApiDebugBtn.addEventListener("click", () => {
      setLoginError("");
      showLogin(true);
    });
  }
  window.addEventListener("hashchange", () => {
    setView(getViewFromHash(), { updateHash: false });
    if (activeView === VIEW_QMSG) {
      qmsgFeature.ensureLoaded().catch(() => {});
    }
    if (activeView === VIEW_NEWAPI) {
      newapiFeature.ensureLoaded().catch(() => {});
    }
    if (activeView === VIEW_STANDBY) {
      standbyFeature.ensureLoaded().catch(() => {});
    }
    if (activeView === VIEW_UPSTREAM_PROVIDER) {
      upstreamProviderFeature.ensureLoaded().catch(() => {});
    }
    if (activeView === VIEW_API_DEBUG) {
      apiDebugFeature.ensureLoaded().catch(() => {});
    }
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
  qmsgFeature.init();
  newapiFeature.init();
  standbyFeature.init();
  upstreamProviderFeature.init();
  apiDebugFeature.init();
  bindGlobalEvents();
  setView(getViewFromHash(), { updateHash: false });
  if (activeView === VIEW_QMSG) {
    qmsgFeature.ensureLoaded().catch(() => {});
  }
  if (activeView === VIEW_NEWAPI) {
    newapiFeature.ensureLoaded().catch(() => {});
  }
  if (activeView === VIEW_STANDBY) {
    standbyFeature.ensureLoaded().catch(() => {});
  }
  if (activeView === VIEW_UPSTREAM_PROVIDER) {
    upstreamProviderFeature.ensureLoaded().catch(() => {});
  }
  if (activeView === VIEW_API_DEBUG) {
    apiDebugFeature.ensureLoaded().catch(() => {});
  }
}

init();
