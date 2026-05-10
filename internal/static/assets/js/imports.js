import { isCredentialError, requestAccountsIngest, requestStatsPage } from "./api.js";
import { buildAlert, escapeHtml, formatNumber } from "./ui.js";

const IMPORT_PRECHECK_PAGE_SIZE = 200;
const IMPORT_MAX_DUPLICATE_PREVIEW = 10;
const REQUIRED_TOKEN_FIELDS = ["refresh_token", "rk", "access_token", "id_token"];
const NESTED_TOKEN_FIELDS = ["refresh_token", "rk", "access_token", "id_token", "account_id", "email", "expired"];
const NESTED_TOKEN_OBJECTS = ["tokens", "credentials"];
const SUB2API_METADATA_FIELDS = ["platform", "group_ids"];

export function createImportsFeature({
  els,
  getCredentials,
  onMissingCredentials,
  onCredentialError,
  onStatsRefresh
}) {
  const state = {
    controller: null,
    validation: null,
    parseError: "",
    selectedFile: "",
    submitError: "",
    submitNotice: "",
    submitNoticeType: "info",
    result: null,
    submitting: false,
    initialized: false
  };

  function isRecord(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
  }

  function hasNonEmptyString(value) {
    return typeof value === "string" && value.trim().length > 0;
  }

  function normalizeImportEmail(value) {
    if (typeof value !== "string") {
      return undefined;
    }
    const normalized = value.trim().toLowerCase();
    return normalized.length > 0 ? normalized : undefined;
  }

  function assertCredentialLikeRecord(record, index) {
    const hasCredentialField = REQUIRED_TOKEN_FIELDS.some(field => hasNonEmptyString(record[field]));
    if (!hasCredentialField) {
      throw new Error(`第 ${index} 条记录缺少 refresh_token/rk、access_token 或 id_token，至少需要一项`);
    }
  }

  function getImportIdentity(record) {
    return normalizeImportEmail(record.email) || normalizeImportEmail(record.name);
  }

  function isSub2APIRecord(record) {
    return Boolean(record.__sub2api)
      || isRecord(record.credentials)
      || SUB2API_METADATA_FIELDS.some(field => Object.prototype.hasOwnProperty.call(record, field));
  }

  function normalizeImportRecord(record, index) {
    const normalizedRecord = { ...record };
    const sub2apiLike = isSub2APIRecord(record);
    NESTED_TOKEN_OBJECTS.forEach(key => {
      const nestedTokens = isRecord(record[key]) ? record[key] : undefined;
      delete normalizedRecord[key];
      if (!nestedTokens) {
        return;
      }
      NESTED_TOKEN_FIELDS.forEach(field => {
        if (!hasNonEmptyString(normalizedRecord[field]) && hasNonEmptyString(nestedTokens[field])) {
          normalizedRecord[field] = nestedTokens[field];
        }
      });
    });
    if (sub2apiLike) {
      Object.defineProperty(normalizedRecord, "__sub2api", {
        value: true,
        enumerable: false
      });
    }
    if (!hasNonEmptyString(normalizedRecord.email) && hasNonEmptyString(record.name)) {
      normalizedRecord.email = record.name;
    }
    assertCredentialLikeRecord(normalizedRecord, index);
    return normalizedRecord;
  }

  function buildImportWarnings(records) {
    let missingEmailCount = 0;
    let noRefreshTokenCount = 0;
    let sub2apiLikeCount = 0;
    records.forEach(record => {
      const hasEmail = Boolean(getImportIdentity(record));
      const hasRefreshToken = hasNonEmptyString(record.refresh_token) || hasNonEmptyString(record.rk);
      const hasFallbackCredential = hasNonEmptyString(record.access_token) || hasNonEmptyString(record.id_token);
      if (isSub2APIRecord(record)) {
        sub2apiLikeCount += 1;
      }
      if (!hasEmail) {
        missingEmailCount += 1;
      }
      if (!hasRefreshToken && hasFallbackCredential) {
        noRefreshTokenCount += 1;
      }
    });
    const warnings = [];
    if (sub2apiLikeCount > 0) {
      warnings.push(`已识别 ${sub2apiLikeCount} 条 sub2api 风格记录；platform/type/group_ids 等元数据不会写入本项目账号池`);
    }
    if (missingEmailCount > 0) {
      warnings.push(`${missingEmailCount} 条记录缺少 email，可导入，但列表搜索和按邮箱恢复不可用`);
    }
    if (noRefreshTokenCount > 0) {
      warnings.push(`${noRefreshTokenCount} 条记录没有 refresh_token/rk，仅依赖 access_token/id_token，可导入，但后续恢复能力可能受限`);
    }
    return warnings;
  }

  function createValidationResult(format, text, records, submitFormat = format) {
    return {
      format,
      submitFormat,
      text,
      records,
      recordCount: records.length,
      warnings: buildImportWarnings(records),
      normalizedEmails: Array.from(new Set(
        records
          .map(record => getImportIdentity(record))
          .filter(email => Boolean(email))
      ))
    };
  }

  function parseJsonPayload(raw) {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      if (parsed.length === 0) {
        throw new Error("JSON 数组不能为空");
      }
      const normalizedRecords = parsed.map((item, index) => {
        if (!isRecord(item)) {
          throw new Error(`第 ${index + 1} 条记录不是有效对象`);
        }
        return normalizeImportRecord(item, index + 1);
      });
      return createValidationResult(
        "json-array",
        JSON.stringify(normalizedRecords, null, 2),
        normalizedRecords
      );
    }
    if (!isRecord(parsed)) {
      throw new Error("JSON 内容必须是对象或数组");
    }
    const normalizedRecord = normalizeImportRecord(parsed, 1);
    return createValidationResult(
      "json-object",
      JSON.stringify(normalizedRecord, null, 2),
      [normalizedRecord]
    );
  }

  function parseNdjsonPayload(raw) {
    const lines = raw
      .split(/\r?\n/)
      .map(line => line.trim())
      .filter(line => line.length > 0 && !line.startsWith("#"));
    if (lines.length === 0) {
      throw new Error("NDJSON 内容不能为空");
    }
    const records = [];
    lines.forEach((line, index) => {
      let parsed;
      try {
        parsed = JSON.parse(line);
      } catch (error) {
        throw new Error(`第 ${index + 1} 行不是有效 JSON`);
      }
      if (!isRecord(parsed)) {
        throw new Error(`第 ${index + 1} 行不是有效对象`);
      }
      records.push(normalizeImportRecord(parsed, index + 1));
    });
    return createValidationResult(
      "ndjson",
      JSON.stringify(records, null, 2),
      records,
      "json-array"
    );
  }

  function parseImportPayload(rawInput) {
    const raw = String(rawInput || "").trim();
    if (!raw) {
      throw new Error("请先输入或上传账号内容");
    }
    const firstChar = raw[0];
    if (firstChar === "{" || firstChar === "[") {
      try {
        return parseJsonPayload(raw);
      } catch (error) {
        if (firstChar === "{" && raw.includes("\n")) {
          return parseNdjsonPayload(raw);
        }
        throw error;
      }
    }
    return parseNdjsonPayload(raw);
  }

  function formatImportPayloadFormat(format) {
    switch (format) {
      case "json-object":
        return "JSON 对象";
      case "json-array":
        return "JSON 数组";
      default:
        return "NDJSON";
    }
  }

  function renderImportValidation() {
    const validationBlocks = [];
    if (state.parseError) {
      validationBlocks.push(buildAlert("error", state.parseError));
    } else if (state.validation) {
      validationBlocks.push(buildAlert(
        "success",
        `已识别为 ${formatImportPayloadFormat(state.validation.format)}，共 ${formatNumber(state.validation.recordCount)} 条记录`
      ));
      if (state.validation.warnings.length > 0) {
        validationBlocks.push(buildAlert(
          "warning",
          "导入内容包含非阻断提示",
          `<ul class="alert-list">${state.validation.warnings.map(warning => `<li>${escapeHtml(warning)}</li>`).join("")}</ul>`
        ));
      }
    } else {
      validationBlocks.push(buildAlert("info", "输入内容后，这里会显示校验结果与导入提示。"));
    }
    els.importValidationState.innerHTML = validationBlocks.join("");

    const submitBlocks = [];
    if (state.submitNotice) {
      submitBlocks.push(buildAlert(state.submitNoticeType, state.submitNotice));
    }
    if (state.submitError) {
      submitBlocks.push(buildAlert("error", state.submitError));
    }
    els.importSubmissionState.innerHTML = submitBlocks.join("");
  }

  function renderImportResult() {
    if (!state.result) {
      els.importResultPanel.classList.add("hidden");
      els.importErrorsBlock.classList.add("hidden");
      els.importErrorsList.innerHTML = "";
      return;
    }
    els.importResultPanel.classList.remove("hidden");
    els.importAdded.textContent = formatNumber(state.result.added);
    els.importUpdated.textContent = formatNumber(state.result.updated);
    els.importFailed.textContent = formatNumber(state.result.failed);
    els.importPoolTotal.textContent = formatNumber(state.result.poolTotal);
    if (state.result.errors.length > 0) {
      els.importErrorsBlock.classList.remove("hidden");
      els.importErrorsList.innerHTML = state.result.errors
        .map(item => `<li>${escapeHtml(item)}</li>`)
        .join("");
    } else {
      els.importErrorsBlock.classList.add("hidden");
      els.importErrorsList.innerHTML = "";
    }
  }

  function refreshImportControls(label = "处理中...") {
    els.importSubmitBtn.disabled = state.submitting || !state.validation;
    els.importSubmitBtn.textContent = state.submitting ? label : "提交导入";
    els.chooseImportFileBtn.disabled = state.submitting;
    els.clearImportFileBtn.disabled = state.submitting;
    els.importTextarea.disabled = state.submitting;
    els.selectedImportFile.classList.toggle("hidden", !state.selectedFile);
    els.clearImportFileBtn.classList.toggle("hidden", !state.selectedFile);
    els.selectedImportFile.textContent = state.selectedFile || "";
  }

  function setImportSubmitting(isSubmitting, label = "处理中...") {
    state.submitting = isSubmitting;
    refreshImportControls(label);
  }

  function recalculateImportPayload() {
    const rawPayload = els.importTextarea.value;
    state.submitError = "";
    state.submitNotice = "";
    state.submitNoticeType = "info";
    if (!rawPayload.trim()) {
      state.validation = null;
      state.parseError = "";
      renderImportValidation();
      refreshImportControls();
      return;
    }
    try {
      state.validation = parseImportPayload(rawPayload);
      state.parseError = "";
    } catch (error) {
      state.validation = null;
      state.parseError = error?.message || "导入内容解析失败";
    }
    renderImportValidation();
    refreshImportControls();
  }

  function clearImportFileSelection() {
    state.selectedFile = "";
    els.importFileInput.value = "";
    refreshImportControls();
  }

  async function handleImportFileSelection(file) {
    if (!file) {
      return;
    }
    try {
      const text = await file.text();
      state.selectedFile = file.name;
      els.importTextarea.value = text;
      recalculateImportPayload();
    } catch (error) {
      state.submitNotice = "";
      state.submitError = "读取文件失败，请重试";
      renderImportValidation();
    }
    refreshImportControls();
  }

  function buildDuplicateImportMessage(duplicateEmails, skippedRecordsWithoutEmail) {
    const preview = duplicateEmails.slice(0, IMPORT_MAX_DUPLICATE_PREVIEW).join("\n");
    const moreCount = Math.max(0, duplicateEmails.length - IMPORT_MAX_DUPLICATE_PREVIEW);
    const lines = [
      `检测到 ${formatNumber(duplicateEmails.length)} 个重复邮箱。`,
      "继续导入可能会触发后端更新已有账号。"
    ];
    if (preview) {
      lines.push("", "重复邮箱预览：", preview);
    }
    if (moreCount > 0) {
      lines.push(`... 另外还有 ${formatNumber(moreCount)} 个重复邮箱未展示`);
    }
    if (skippedRecordsWithoutEmail > 0) {
      lines.push("", `${formatNumber(skippedRecordsWithoutEmail)} 条记录缺少 email，无法在导入前判断是否已存在。`);
    }
    lines.push("", "确定继续导入吗？");
    return lines.join("\n");
  }

  async function fetchDuplicateImportEmails(cred, payload, signal) {
    const skippedRecordsWithoutEmail = payload.records.filter(record => !getImportIdentity(record)).length;
    if (payload.normalizedEmails.length === 0) {
      return {
        duplicateEmails: [],
        skippedRecordsWithoutEmail
      };
    }
    const existingEmails = new Set();
    let page = 1;
    while (true) {
      const statsPage = await requestStatsPage(cred, {
        page,
        pageSize: IMPORT_PRECHECK_PAGE_SIZE,
        includeQuota: false,
        query: ""
      }, signal);
      statsPage.accounts.forEach(account => {
        const normalizedEmail = normalizeImportEmail(account.email);
        if (normalizedEmail) {
          existingEmails.add(normalizedEmail);
        }
      });
      const pageMeta = statsPage.pagination || {};
      const hasNext = Boolean(pageMeta.has_next ?? pageMeta.hasNext);
      const totalPages = Number(pageMeta.total_pages ?? pageMeta.totalPages ?? page);
      if (!hasNext || page >= totalPages) {
        break;
      }
      page += 1;
    }
    const normalizedEmails = payload.normalizedEmails.filter(email => existingEmails.has(email));
    return {
      duplicateEmails: normalizedEmails,
      skippedRecordsWithoutEmail
    };
  }

  async function handleImportSubmit() {
    if (state.submitting) {
      return;
    }
    if (!state.validation) {
      state.submitNotice = "";
      state.submitError = state.parseError || "请先准备有效的导入内容";
      renderImportValidation();
      return;
    }
    const cred = getCredentials();
    if (!cred?.apiUrl) {
      state.submitNotice = "";
      state.submitError = "请先设置可访问的 /stats 接口地址和 Bearer Token";
      renderImportValidation();
      onMissingCredentials();
      return;
    }
    if (state.controller) {
      state.controller.abort();
    }
    const controller = new AbortController();
    state.controller = controller;
    state.submitError = "";
    state.submitNotice = "";
    state.submitNoticeType = "info";
    renderImportValidation();
    setImportSubmitting(true, "校验中...");
    try {
      const duplicateCheck = await fetchDuplicateImportEmails(cred, state.validation, controller.signal);
      if (duplicateCheck.duplicateEmails.length > 0) {
        const shouldContinue = window.confirm(
          buildDuplicateImportMessage(
            duplicateCheck.duplicateEmails,
            duplicateCheck.skippedRecordsWithoutEmail
          )
        );
        if (!shouldContinue) {
          state.submitNoticeType = "info";
          state.submitNotice = "已取消导入，未向服务端提交数据。";
          renderImportValidation();
          return;
        }
      } else if (duplicateCheck.skippedRecordsWithoutEmail > 0) {
        state.submitNoticeType = "warning";
        state.submitNotice = `${formatNumber(duplicateCheck.skippedRecordsWithoutEmail)} 条记录缺少 email，无法在导入前判断是否已存在。`;
        renderImportValidation();
      }
      setImportSubmitting(true, "导入中...");
      const result = await requestAccountsIngest(cred, state.validation, controller.signal);
      state.result = result;
      renderImportResult();
      state.submitNoticeType = "success";
      state.submitNotice = duplicateCheck.skippedRecordsWithoutEmail > 0
        ? `账号导入完成，另有 ${formatNumber(duplicateCheck.skippedRecordsWithoutEmail)} 条缺少 email 的记录未参与重复预检查。`
        : "账号导入完成，可返回统计查看最新数据。";
      state.submitError = "";
      renderImportValidation();
      await onStatsRefresh();
    } catch (error) {
      if (error?.name === "AbortError") {
        return;
      }
      state.submitNotice = "";
      state.submitError = error?.message || "账号导入失败";
      if (isCredentialError(error) && typeof onCredentialError === "function") {
        onCredentialError(state.submitError, error);
      }
      renderImportValidation();
    } finally {
      if (state.controller === controller) {
        state.controller = null;
      }
      setImportSubmitting(false);
    }
  }

  function bindEvents() {
    els.chooseImportFileBtn.addEventListener("click", () => {
      els.importFileInput.click();
    });
    els.importFileInput.addEventListener("change", event => {
      void handleImportFileSelection(event.target.files?.[0]);
    });
    els.clearImportFileBtn.addEventListener("click", () => {
      clearImportFileSelection();
    });
    els.importTextarea.addEventListener("input", () => {
      recalculateImportPayload();
    });
    els.importSubmitBtn.addEventListener("click", () => {
      void handleImportSubmit();
    });
  }

  function init() {
    if (state.initialized) {
      return;
    }
    state.initialized = true;
    renderImportValidation();
    renderImportResult();
    refreshImportControls();
    bindEvents();
  }

  return {
    init
  };
}
