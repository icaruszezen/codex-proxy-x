function joinPath(basePath, endpointPath) {
  const cleanBase = String(basePath || "").replace(/\/+$/, "");
  const cleanEndpoint = `/${String(endpointPath || "").replace(/^\/+/, "")}`;
  const joined = `${cleanBase}${cleanEndpoint}`.replace(/\/+/g, "/");
  return joined || "/";
}

export function buildEndpointUrl(apiUrl, endpointPath, params = {}) {
  let url;
  try {
    url = new URL(apiUrl, window.location.href);
  } catch (error) {
    throw new Error("接口地址无效");
  }
  const currentPath = url.pathname || "";
  const basePath = /\/stats\/?$/i.test(currentPath)
    ? currentPath.replace(/\/stats\/?$/i, "")
    : "";
  url.pathname = joinPath(basePath, endpointPath);
  url.search = "";
  url.hash = "";
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === "") {
      return;
    }
    url.searchParams.set(key, String(value));
  });
  return url.toString();
}

export function buildStatsUrl(apiUrl, options = {}) {
  const query = String(options.query ?? "").trim();
  const status = String(options.status ?? "all").trim().toLowerCase();
  return buildEndpointUrl(apiUrl, "/stats", {
    page: Number(options.page ?? 1) || 1,
    page_size: Number(options.pageSize ?? 25) || 25,
    include_quota: options.includeQuota ? 1 : 0,
    q: query || undefined,
    status: status === "all" ? undefined : status
  });
}

export async function buildRequestError(res) {
  let message = `HTTP ${res.status}`;
  let code = "";
  try {
    const payload = await res.clone().json();
    message = payload?.error?.message || payload?.message || message;
    code = payload?.error?.code || payload?.code || "";
  } catch (error) {
    const text = (await res.text()).trim();
    if (text) {
      message = text;
    }
  }
  const requestError = new Error(message);
  requestError.status = res.status;
  requestError.code = code;
  return requestError;
}

export function isCredentialError(error) {
  return error?.status === 401 || error?.status === 403 || error?.code === "invalid_api_key";
}

export function buildHeaders(cred, extra = {}) {
  const headers = { ...extra };
  if (cred?.token) {
    headers.Authorization = `Bearer ${cred.token}`;
  }
  return headers;
}

export function normalizeStatsResponse(data, options = {}) {
  const targetPage = Math.max(1, Number(options.page ?? 1) || 1);
  const targetPageSize = Math.max(1, Number(options.pageSize ?? 25) || 25);
  const keyword = String(options.query ?? "").trim().toLowerCase();
  const statusFilter = String(options.status ?? "all").trim().toLowerCase();
  const recentEvents = Array.isArray(data?.recent_events) ? data.recent_events : [];
  if (data?.pagination) {
    return {
      ...data,
      summary: data.summary || {},
      accounts: Array.isArray(data.accounts) ? data.accounts : [],
      recent_events: recentEvents,
      pagination: {
        ...data.pagination,
        page: Number(data.pagination.page ?? targetPage) || targetPage,
        page_size: Number(data.pagination.page_size ?? targetPageSize) || targetPageSize
      }
    };
  }
  const allRows = Array.isArray(data?.accounts) ? data.accounts : [];
  const filteredRows = allRows.filter(row => {
    const status = String(row?.status || "").toLowerCase();
    if (statusFilter === "enabled" && status === "disabled") {
      return false;
    }
    if (statusFilter === "disabled" && status !== "disabled") {
      return false;
    }
    if (keyword && !String(row?.email || "").toLowerCase().includes(keyword)) {
      return false;
    }
    return true;
  });
  const totalPages = Math.max(1, Math.ceil(filteredRows.length / targetPageSize));
  const safePage = Math.min(Math.max(1, targetPage), totalPages);
  const start = (safePage - 1) * targetPageSize;
  const pageRows = filteredRows.slice(start, start + targetPageSize);
  return {
    summary: data?.summary || {},
    accounts: pageRows,
    recent_events: recentEvents,
    pagination: {
      page: safePage,
      page_size: targetPageSize,
      total: allRows.length,
      filtered_total: filteredRows.length,
      total_pages: totalPages,
      returned: pageRows.length,
      has_prev: safePage > 1,
      has_next: safePage < totalPages,
      query: keyword
    }
  };
}

export async function requestStatsPage(cred, options = {}, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildStatsUrl(cred.apiUrl, options), {
    method: "GET",
    headers: buildHeaders(cred),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  return normalizeStatsResponse(await res.json(), options);
}

export function getImportContentType(format) {
  return format === "ndjson" ? "text/plain" : "application/json";
}

export async function requestAccountsIngest(cred, payload, signal) {
  const submitFormat = payload?.submitFormat || payload?.format;
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/accounts/ingest"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": getImportContentType(submitFormat)
    }),
    body: payload.text,
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    added: Number(data?.added ?? 0),
    updated: Number(data?.updated ?? 0),
    failed: Number(data?.failed ?? 0),
    poolTotal: Number(data?.pool_total ?? data?.poolTotal ?? 0),
    errors: Array.isArray(data?.errors) ? data.errors : []
  };
}

export async function requestCodexAuthURL(cred, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/auth/codex/url"), {
    method: "GET",
    headers: buildHeaders(cred),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    url: String(data?.url || ""),
    state: String(data?.state || ""),
    expiresIn: Number(data?.expires_in ?? 0)
  };
}

export async function requestCodexExchange(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {};
  const callbackUrl = String(payload?.callbackUrl || "").trim();
  const code = String(payload?.code || "").trim();
  const state = String(payload?.state || "").trim();
  if (callbackUrl) body.callback_url = callbackUrl;
  if (code) body.code = code;
  if (state) body.state = state;
  if (!body.callback_url && !body.code) {
    throw new Error("请粘贴回调地址或填写 code");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/auth/codex/exchange"), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    status: String(data?.status || ""),
    message: String(data?.message || ""),
    email: String(data?.email || ""),
    accountId: String(data?.account_id || "")
  };
}

export async function requestRecoverAuth(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = payload?.all
    ? { all: true }
    : { email: String(payload?.email || "").trim() };
  if (!body.all && !body.email) {
    throw new Error("缺少可恢复的账号邮箱");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/recover-auth"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": "application/json"
    }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    object: String(data?.object || ""),
    result: data?.result || null,
    results: Array.isArray(data?.results) ? data.results : [],
    count: Number(data?.count ?? 0),
    durationMs: Number(data?.duration_ms ?? data?.durationMs ?? 0)
  };
}

export async function requestAccountToggleEnabled(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {
    enabled: Boolean(payload?.enabled)
  };
  const email = String(payload?.email || "").trim();
  const filePath = String(payload?.filePath || payload?.file_path || "").trim();
  if (email) {
    body.email = email;
  }
  if (filePath) {
    body.file_path = filePath;
  }
  if (!body.email && !body.file_path) {
    throw new Error("缺少可切换状态的账号标识");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/accounts/toggle-enabled"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": "application/json"
    }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    email: String(data?.email || body.email || ""),
    enabled: Boolean(data?.enabled),
    status: String(data?.status || ""),
    disableReason: String(data?.disable_reason || "")
  };
}

export async function requestAccountsExport(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const emails = Array.isArray(payload?.emails)
    ? payload.emails.map(email => String(email || "").trim()).filter(Boolean)
    : [];
  if (!emails.length) {
    throw new Error("请至少选择一个账号");
  }
  const format = String(payload?.format || "sub2api-export").trim() || "sub2api-export";
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/accounts/export"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": "application/json"
    }),
    body: JSON.stringify({ emails, format }),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    format: String(data?.format || format),
    exported: Number(data?.exported ?? 0),
    notFound: Array.isArray(data?.not_found) ? data.not_found : [],
    failed: Array.isArray(data?.failed) ? data.failed : [],
    data: data?.data
  };
}

export async function requestAccountDelete(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {};
  const email = String(payload?.email || "").trim();
  const filePath = String(payload?.filePath || payload?.file_path || "").trim();
  if (email) {
    body.email = email;
  }
  if (filePath) {
    body.file_path = filePath;
  }
  if (!body.email && !body.file_path) {
    throw new Error("缺少可删除的账号标识");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/accounts/delete"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": "application/json"
    }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    email: String(data?.email || body.email || ""),
    filePath: String(data?.file_path || body.file_path || ""),
    deleted: Boolean(data?.deleted),
    poolTotal: Number(data?.pool_total ?? 0)
  };
}

export async function requestQmsgConfig(cred, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/qmsg/config"), {
    method: "GET",
    headers: buildHeaders(cred),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return data?.config || {};
}

export async function saveQmsgConfig(cred, config, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {
    enabled: Boolean(config?.enabled),
    key: String(config?.key || "").trim(),
    qq: String(config?.qq || "").trim(),
    bot: String(config?.bot || "").trim(),
    timeout_sec: Number(config?.timeoutSec ?? config?.timeout_sec ?? 10) || 10,
    message_template: String(config?.messageTemplate ?? config?.message_template ?? ""),
    endpoint_template: String(config?.endpointTemplate ?? config?.endpoint_template ?? "")
  };
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/qmsg/config"), {
    method: "PUT",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return data?.config || {};
}

export async function testQmsgChannel(cred, message, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/qmsg/test"), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify({ message: String(message || "").trim() }),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  return await res.json();
}

/* ========== NewAPI 渠道 ========== */

export async function requestNewapiConfig(cred, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/newapi/config"), {
    method: "GET",
    headers: buildHeaders(cred),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return data?.config || {};
}

export async function saveNewapiConfig(cred, config, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {
    auto_switch: Boolean(config?.autoSwitch ?? config?.auto_switch),
    base_url: String(config?.baseUrl ?? config?.base_url ?? "").trim(),
    admin_token: String(config?.adminToken ?? config?.admin_token ?? "").trim(),
    admin_user_id: Number(config?.adminUserId ?? config?.admin_user_id ?? 0) || 0,
    channel_id: Number(config?.channelId ?? config?.channel_id ?? 0) || 0,
    timeout_sec: Number(config?.timeoutSec ?? config?.timeout_sec ?? 10) || 10
  };
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/newapi/config"), {
    method: "PUT",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return data?.config || {};
}

export async function testNewapiEnable(cred, signal) {
  return testNewapiChannel(cred, "/admin/newapi/test/enable", signal);
}

export async function testNewapiDisable(cred, signal) {
  return testNewapiChannel(cred, "/admin/newapi/test/disable", signal);
}

async function testNewapiChannel(cred, endpoint, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, endpoint), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: "{}",
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  return await res.json();
}

/* ========== 备用账号池 ========== */

export async function requestStandbyState(cred, options = {}, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const params = {};
  const query = String(options.query || "").trim();
  const status = String(options.status || "all").trim().toLowerCase();
  if (query) params.q = query;
  if (status && status !== "all") params.status = status;
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/state", params), {
    method: "GET",
    headers: buildHeaders(cred),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    active: Boolean(data?.active),
    primaryTotal: Number(data?.primary_total ?? 0),
    standbyTotal: Number(data?.standby_total ?? 0),
    note: String(data?.note || ""),
    summary: data?.summary || {},
    accounts: Array.isArray(data?.accounts) ? data.accounts : []
  };
}

export async function requestStandbyIngest(cred, payload, signal) {
  const submitFormat = payload?.submitFormat || payload?.format;
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/accounts/ingest"), {
    method: "POST",
    headers: buildHeaders(cred, {
      "Content-Type": getImportContentType(submitFormat)
    }),
    body: payload.text,
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    added: Number(data?.added ?? 0),
    updated: Number(data?.updated ?? 0),
    failed: Number(data?.failed ?? 0),
    poolTotal: Number(data?.pool_total ?? data?.poolTotal ?? 0),
    errors: Array.isArray(data?.errors) ? data.errors : []
  };
}

export async function requestStandbyExport(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const emails = Array.isArray(payload?.emails)
    ? payload.emails.map(email => String(email || "").trim()).filter(Boolean)
    : [];
  if (!emails.length) {
    throw new Error("请至少选择一个账号");
  }
  const format = String(payload?.format || "sub2api-export").trim() || "sub2api-export";
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/accounts/export"), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify({ emails, format }),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    format: String(data?.format || format),
    exported: Number(data?.exported ?? 0),
    notFound: Array.isArray(data?.not_found) ? data.not_found : [],
    failed: Array.isArray(data?.failed) ? data.failed : [],
    data: data?.data
  };
}

export async function requestStandbyDelete(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = {};
  const email = String(payload?.email || "").trim();
  const filePath = String(payload?.filePath || payload?.file_path || "").trim();
  if (email) body.email = email;
  if (filePath) body.file_path = filePath;
  if (!body.email && !body.file_path) {
    throw new Error("缺少可删除的账号标识");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/accounts/delete"), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    email: String(data?.email || body.email || ""),
    filePath: String(data?.file_path || body.file_path || ""),
    deleted: Boolean(data?.deleted),
    poolTotal: Number(data?.pool_total ?? 0)
  };
}

export async function requestStandbyToggleEnabled(cred, payload, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const body = { enabled: Boolean(payload?.enabled) };
  const email = String(payload?.email || "").trim();
  const filePath = String(payload?.filePath || payload?.file_path || "").trim();
  if (email) body.email = email;
  if (filePath) body.file_path = filePath;
  if (!body.email && !body.file_path) {
    throw new Error("缺少可切换状态的账号标识");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/accounts/toggle-enabled"), {
    method: "POST",
    headers: buildHeaders(cred, { "Content-Type": "application/json" }),
    body: JSON.stringify(body),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const data = await res.json();
  return {
    email: String(data?.email || body.email || ""),
    enabled: Boolean(data?.enabled),
    status: String(data?.status || ""),
    disableReason: String(data?.disable_reason || "")
  };
}

/**
 * requestStandbyHealthCheck 触发一次手动健康检查；以 SSE 流式接收进度事件
 * @param onEvent 进度事件回调；每收到一条 ProgressEvent (item 或 done) 调用一次
 */
export async function requestStandbyHealthCheck(cred, onEvent, signal) {
  if (!cred?.apiUrl) {
    throw new Error("请输入 API 地址");
  }
  const res = await fetch(buildEndpointUrl(cred.apiUrl, "/admin/standby/health-check"), {
    method: "POST",
    headers: buildHeaders(cred, { Accept: "text/event-stream" }),
    signal
  });
  if (!res.ok) {
    throw await buildRequestError(res);
  }
  const reader = res.body?.getReader();
  if (!reader) {
    return;
  }
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let idx;
    while ((idx = buffer.indexOf("\n\n")) !== -1) {
      const rawBlock = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      const lines = rawBlock.split("\n");
      let dataLine = "";
      for (const line of lines) {
        if (line.startsWith("data:")) {
          dataLine += line.slice(5).trim();
        }
      }
      if (!dataLine) continue;
      try {
        const event = JSON.parse(dataLine);
        if (typeof onEvent === "function") {
          onEvent(event);
        }
      } catch (error) {
        /* 忽略解析失败 */
      }
    }
  }
}
