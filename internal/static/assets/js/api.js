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
  return buildEndpointUrl(apiUrl, "/stats", {
    page: Number(options.page ?? 1) || 1,
    page_size: Number(options.pageSize ?? 25) || 25,
    include_quota: options.includeQuota ? 1 : 0,
    q: query || undefined
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
  if (data?.pagination) {
    return {
      ...data,
      summary: data.summary || {},
      accounts: Array.isArray(data.accounts) ? data.accounts : [],
      pagination: {
        ...data.pagination,
        page: Number(data.pagination.page ?? targetPage) || targetPage,
        page_size: Number(data.pagination.page_size ?? targetPageSize) || targetPageSize
      }
    };
  }
  const allRows = Array.isArray(data?.accounts) ? data.accounts : [];
  const filteredRows = keyword
    ? allRows.filter(row => String(row?.email || "").toLowerCase().includes(keyword))
    : allRows;
  const totalPages = Math.max(1, Math.ceil(filteredRows.length / targetPageSize));
  const safePage = Math.min(Math.max(1, targetPage), totalPages);
  const start = (safePage - 1) * targetPageSize;
  const pageRows = filteredRows.slice(start, start + targetPageSize);
  return {
    summary: data?.summary || {},
    accounts: pageRows,
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
