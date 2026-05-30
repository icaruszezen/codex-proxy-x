/**
 * 响应转换模块
 * 将 Codex (OpenAI Responses API) 的流式和非流式响应转换为 OpenAI Chat Completions 格式
 * 处理文本内容、工具调用、推理内容、用量元数据等的格式映射
 */
package translator

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var dataPrefix = []byte("data:")

type chatUsageDetails struct {
	PromptTokens        int64
	CompletionTokens    int64
	TotalTokens         int64
	CachedTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	ReasoningTokens     int64

	HasPrompt        bool
	HasCompletion    bool
	HasTotal         bool
	HasCached        bool
	HasCacheCreation bool
	HasCacheRead     bool
	HasReasoning     bool
}

func firstUsageInt(usage gjson.Result, paths ...string) (int64, bool) {
	for _, path := range paths {
		if v := usage.Get(path); v.Exists() {
			return v.Int(), true
		}
	}
	return 0, false
}

func normalizeChatUsage(usage gjson.Result) chatUsageDetails {
	d := chatUsageDetails{}
	if v, ok := firstUsageInt(usage, "input_tokens", "prompt_tokens"); ok {
		d.PromptTokens = v
		d.HasPrompt = true
	}
	if v, ok := firstUsageInt(usage, "output_tokens", "completion_tokens"); ok {
		d.CompletionTokens = v
		d.HasCompletion = true
	}
	if v, ok := firstUsageInt(usage, "total_tokens"); ok {
		d.TotalTokens = v
		d.HasTotal = true
	}
	if v, ok := firstUsageInt(usage,
		"cache_creation_input_tokens",
		"input_tokens_details.cache_creation_tokens",
		"prompt_tokens_details.cache_creation_tokens",
		"cache_creation.input_tokens",
		"prompt_cache_miss_tokens",
	); ok {
		d.CacheCreationTokens = v
		d.HasCacheCreation = true
	}
	if v, ok := firstUsageInt(usage,
		"cache_read_input_tokens",
		"input_tokens_details.cache_read_tokens",
		"prompt_tokens_details.cache_read_tokens",
		"cache_read.input_tokens",
		"input_cached_tokens",
		"prompt_cache_hit_tokens",
	); ok {
		d.CacheReadTokens = v
		d.HasCacheRead = true
	}
	if v, ok := firstUsageInt(usage,
		"input_tokens_details.cached_tokens",
		"prompt_tokens_details.cached_tokens",
		"input_cached_tokens",
		"cached_tokens",
		"cache_read_input_tokens",
		"prompt_cache_hit_tokens",
	); ok {
		d.CachedTokens = v
		d.HasCached = true
	} else if d.HasCacheRead {
		d.CachedTokens = d.CacheReadTokens
		d.HasCached = true
	}
	if v, ok := firstUsageInt(usage, "output_tokens_details.reasoning_tokens", "completion_tokens_details.reasoning_tokens"); ok {
		d.ReasoningTokens = v
		d.HasReasoning = true
	}
	return d
}

func (d chatUsageDetails) hasAny() bool {
	return d.HasPrompt || d.HasCompletion || d.HasTotal || d.HasCached || d.HasCacheCreation || d.HasCacheRead || d.HasReasoning
}

func (d chatUsageDetails) withComputedTotal() chatUsageDetails {
	if !d.HasTotal && (d.HasPrompt || d.HasCompletion) {
		d.TotalTokens = d.PromptTokens + d.CompletionTokens
		d.HasTotal = true
	}
	return d
}

func setChatUsage(tpl string, d chatUsageDetails) string {
	d = d.withComputedTotal()
	if d.HasPrompt {
		tpl, _ = sjson.Set(tpl, "usage.prompt_tokens", d.PromptTokens)
	}
	if d.HasCompletion {
		tpl, _ = sjson.Set(tpl, "usage.completion_tokens", d.CompletionTokens)
	}
	if d.HasTotal {
		tpl, _ = sjson.Set(tpl, "usage.total_tokens", d.TotalTokens)
	}
	if d.HasCached {
		tpl, _ = sjson.Set(tpl, "usage.prompt_tokens_details.cached_tokens", d.CachedTokens)
	}
	if d.HasCacheCreation {
		tpl, _ = sjson.Set(tpl, "usage.prompt_tokens_details.cache_creation_tokens", d.CacheCreationTokens)
		tpl, _ = sjson.Set(tpl, "usage.cache_creation_input_tokens", d.CacheCreationTokens)
	}
	if d.HasCacheRead {
		tpl, _ = sjson.Set(tpl, "usage.prompt_tokens_details.cache_read_tokens", d.CacheReadTokens)
		tpl, _ = sjson.Set(tpl, "usage.cache_read_input_tokens", d.CacheReadTokens)
	}
	if d.HasReasoning {
		tpl, _ = sjson.Set(tpl, "usage.completion_tokens_details.reasoning_tokens", d.ReasoningTokens)
	}
	return tpl
}

func (s *StreamState) setUsage(d chatUsageDetails) {
	if s == nil {
		return
	}
	if d.HasPrompt {
		s.UsageInput = d.PromptTokens
		s.HasUsageInput = true
	}
	if d.HasCompletion {
		s.UsageOutput = d.CompletionTokens
		s.HasUsageOutput = true
	}
	if d.HasTotal {
		s.UsageTotal = d.TotalTokens
		s.HasUsageTotal = true
	}
	if d.HasCached {
		s.UsageCached = d.CachedTokens
		s.HasUsageCached = true
	}
	if d.HasCacheCreation {
		s.UsageCacheCreation = d.CacheCreationTokens
		s.HasUsageCacheCreation = true
	}
	if d.HasCacheRead {
		s.UsageCacheRead = d.CacheReadTokens
		s.HasUsageCacheRead = true
	}
	if d.HasReasoning {
		s.UsageReasoning = d.ReasoningTokens
		s.HasUsageReasoning = true
	}
}

func (s *StreamState) usageDetails() chatUsageDetails {
	if s == nil {
		return chatUsageDetails{}
	}
	return chatUsageDetails{
		PromptTokens:        s.UsageInput,
		CompletionTokens:    s.UsageOutput,
		TotalTokens:         s.UsageTotal,
		CachedTokens:        s.UsageCached,
		CacheCreationTokens: s.UsageCacheCreation,
		CacheReadTokens:     s.UsageCacheRead,
		ReasoningTokens:     s.UsageReasoning,
		HasPrompt:           s.HasUsageInput,
		HasCompletion:       s.HasUsageOutput,
		HasTotal:            s.HasUsageTotal,
		HasCached:           s.HasUsageCached,
		HasCacheCreation:    s.HasUsageCacheCreation,
		HasCacheRead:        s.HasUsageCacheRead,
		HasReasoning:        s.HasUsageReasoning,
	}
}

/**
 * StreamState 流式响应转换的状态对象
 * 在多次调用之间维护上下文（如 response ID、函数调用索引等）
 * @field ResponseID - Codex 响应 ID
 * @field CreatedAt - 创建时间戳
 * @field Model - 模型名称
 * @field FunctionCallIndex - 当前函数调用的索引
 * @field HasReceivedArgsDelta - 是否已接收到函数参数增量
 * @field HasToolCallAnnounced - 是否已发送过工具调用通知
 * @field baseTpl - 预构建的基础 JSON 模板（id/created/model 已设置，避免每事件重复构建）
 * @field UsageInput - response.completed 时提取的 input_tokens
 * @field UsageOutput - response.completed 时提取的 output_tokens
 * @field UsageTotal - response.completed 时提取的 total_tokens
 * @field HasReasoning - 是否已向客户端输出过思维链（reasoning_content）
 * @field reasoningDeltaByItem - 按 item_id 累积 reasoning_text.delta，用于与 reasoning_text.done 对齐补尾
 */
type StreamState struct {
	ResponseID               string
	CreatedAt                int64
	Model                    string
	FunctionCallIndex        int
	HasText                  bool
	HasToolCall              bool
	HasReasoning             bool
	Completed                bool
	HasReceivedArgsDelta     bool
	HasToolCallAnnounced     bool
	baseTpl                  string
	UsageInput               int64
	UsageOutput              int64
	UsageTotal               int64
	UsageCached              int64
	UsageCacheCreation       int64
	UsageCacheRead           int64
	UsageReasoning           int64
	HasUsageInput            bool
	HasUsageOutput           bool
	HasUsageTotal            bool
	HasUsageCached           bool
	HasUsageCacheCreation    bool
	HasUsageCacheRead        bool
	HasUsageReasoning        bool
	reasoningDeltaByItem     map[string]string
	hasReasoningSummaryDelta bool

	/* 空响应排查：每条有效 data: JSON 行计数及首尾 type（仅日志） */
	diagUpstreamDataLines int
	diagFirstEventType    string
	diagLastEventType     string
	/* 上游 SSE 级错误（error / response.failed），无 Chat delta 时仍应换号重试 */
	UpstreamErrCode string
	UpstreamErrMsg  string
}

func (s *StreamState) noteUpstreamDataEvent(dataType string) {
	if s == nil {
		return
	}
	s.diagUpstreamDataLines++
	if dataType == "" {
		return
	}
	if s.diagFirstEventType == "" {
		s.diagFirstEventType = dataType
	}
	s.diagLastEventType = dataType
}

// EmptyUpstreamDiag 供 executor 在 ErrEmptyResponse 时打日志（不含请求体原文）
func (s *StreamState) EmptyUpstreamDiag(pumpScanLines int) string {
	if s == nil {
		return "state=nil"
	}
	base := fmt.Sprintf("upstream_scan_lines=%d upstream_data_events=%d first_type=%q last_type=%q response_id=%q codex_completed=%v mapped(text=%v tool=%v reasoning=%v)",
		pumpScanLines, s.diagUpstreamDataLines, s.diagFirstEventType, s.diagLastEventType, s.ResponseID, s.Completed, s.HasText, s.HasToolCall, s.HasReasoning)
	if s.UpstreamErrCode != "" || s.UpstreamErrMsg != "" {
		msg := s.UpstreamErrMsg
		const maxMsg = 240
		if len(msg) > maxMsg {
			msg = msg[:maxMsg] + "…(truncated)"
		}
		return base + fmt.Sprintf(" upstream_sse_error code=%q msg=%q", s.UpstreamErrCode, msg)
	}
	return base
}

/**
 * NewStreamState 创建新的流式状态对象
 * @param model - 模型名称
 * @returns *StreamState - 流式状态实例
 */
func NewStreamState(model string) *StreamState {
	return &StreamState{
		Model:             model,
		FunctionCallIndex: -1,
	}
}

/**
 * ConvertStreamChunk 将单个 Codex SSE 事件转换为 OpenAI Chat Completions 流式格式
 *
 * 支持的事件类型映射：
 *   - response.created → 缓存 ID/时间戳（不输出）
 *   - response.output_text.delta → choices[0].delta.content
 *   - response.reasoning_summary_text.delta → choices[0].delta.reasoning_content
 *   - response.output_item.added (function_call) → choices[0].delta.tool_calls
 *   - response.function_call_arguments.delta → tool_calls arguments
 *   - response.completed → finish_reason
 *
 * @param ctx - 上下文（当前未使用，预留用于将来添加取消/超时控制或保持与现有接口一致）
 * @param rawLine - 原始 SSE 行数据
 * @param state - 流式状态对象
 * @param reverseToolMap - 缩短名→原始名的工具名映射
 * @param usageFinalSeparateChunk - 为 true 时（客户端 stream_options.include_usage）不把 usage 写入本块，由调用方在 [DONE] 前单独发 choices 为 [] 的 usage 块，兼容 One API 等依赖该格式的网关计费
 * @returns []string - 转换后的 OpenAI 格式 JSON 字符串列表
 */
func ConvertStreamChunk(_ context.Context, rawLine []byte, state *StreamState, reverseToolMap map[string]string, usageFinalSeparateChunk bool) []string {
	if !bytes.HasPrefix(rawLine, dataPrefix) {
		return nil
	}
	rawJSON := bytes.TrimSpace(rawLine[5:])
	if len(rawJSON) == 0 {
		return nil
	}

	root := gjson.ParseBytes(rawJSON)
	dataType := root.Get("type").String()
	state.noteUpstreamDataEvent(dataType)
	if dataType == "response.created" {
		state.ResponseID = root.Get("response.id").String()
		state.CreatedAt = root.Get("response.created_at").Int()
		if m := root.Get("response.model").String(); m != "" {
			state.Model = m
		}
		state.reasoningDeltaByItem = nil
		state.HasReasoning = false
		state.hasReasoningSummaryDelta = false
		tpl := `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[{"index":0,"delta":{"role":null,"content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null}]}`
		tpl, _ = sjson.Set(tpl, "id", state.ResponseID)
		tpl, _ = sjson.Set(tpl, "created", state.CreatedAt)
		tpl, _ = sjson.Set(tpl, "model", state.Model)
		state.baseTpl = tpl
		return nil
	}

	/*
	 * 复用预构建的基础模板（id/created/model 已设置好）
	 * Go 字符串不可变，sjson.Set 返回新字符串，不会污染 baseTpl
	 * 每个 delta 事件省去 3 次 sjson.Set
	 */
	tpl := state.baseTpl
	if tpl == "" {
		tpl = `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[{"index":0,"delta":{"role":null,"content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null}]}`
		tpl, _ = sjson.Set(tpl, "id", state.ResponseID)
		tpl, _ = sjson.Set(tpl, "created", state.CreatedAt)
		tpl, _ = sjson.Set(tpl, "model", state.Model)
	}

	switch dataType {
	case "response.reasoning_summary_text.delta":
		if delta := root.Get("delta"); delta.Exists() {
			ds := delta.String()
			if ds == "" {
				return nil
			}
			state.hasReasoningSummaryDelta = true
			state.HasReasoning = true
			tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
			tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", ds)
		}

	case "response.reasoning_summary_text.done":
		/* 仅有 .done、无 delta 时用 text 补全摘要，避免重复输出 */
		if txt := root.Get("text").String(); txt != "" && !state.hasReasoningSummaryDelta {
			state.HasReasoning = true
			tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
			tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", txt)
		} else {
			tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
			tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", "\n\n")
		}

	case "response.reasoning.delta", "response.reasoning_text.delta":
		itemID := root.Get("item_id").String()
		if itemID == "" {
			itemID = fmt.Sprintf("_idx:%d", root.Get("output_index").Int())
		}
		delta := root.Get("delta")
		if !delta.Exists() {
			return nil
		}
		ds := delta.String()
		if state.reasoningDeltaByItem == nil {
			state.reasoningDeltaByItem = make(map[string]string)
		}
		state.reasoningDeltaByItem[itemID] += ds
		if ds == "" {
			return nil
		}
		state.HasReasoning = true
		tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
		tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", ds)

	case "response.reasoning_text.done":
		full := root.Get("text").String()
		itemID := root.Get("item_id").String()
		if itemID == "" {
			itemID = fmt.Sprintf("_idx:%d", root.Get("output_index").Int())
		}
		acc := ""
		if state.reasoningDeltaByItem != nil {
			acc = state.reasoningDeltaByItem[itemID]
			delete(state.reasoningDeltaByItem, itemID)
		}
		if full == "" {
			return nil
		}
		/* 无 delta 时 .done 的 text 为全文；有 delta 时补发尾部（避免上游只发 done） */
		var toEmit string
		if acc == "" {
			toEmit = full
		} else if strings.HasPrefix(full, acc) && len(full) > len(acc) {
			toEmit = full[len(acc):]
		} else if full != acc {
			toEmit = full
		}
		if toEmit == "" {
			return nil
		}
		state.HasReasoning = true
		tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
		tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", toEmit)

	case "response.content_part.added":
		/* 部分上游用 content_part 承载 reasoning_text（与 reasoning_text.delta 二选一或并存） */
		part := root.Get("part")
		if part.Get("type").String() == "reasoning_text" {
			if t := part.Get("text").String(); t != "" {
				state.HasReasoning = true
				tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
				tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", t)
			}
		}

	case "response.output_text.delta":
		delta := root.Get("delta").String()
		if delta == "" {
			return nil
		}
		state.HasText = true
		tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
		tpl, _ = sjson.Set(tpl, "choices.0.delta.content", delta)

	case "response.completed":
		state.Completed = true
		/* 上游已 completed 但无任何正文/工具/思维：不向客户端发 finish_reason chunk，由 executor 按空流换号，避免 chunkCount>0 阻断重试 */
		if !state.HasText && !state.HasToolCall && !state.HasReasoning {
			return nil
		}
		finishReason := "stop"
		if state.FunctionCallIndex != -1 {
			finishReason = "tool_calls"
		}
		tpl, _ = sjson.Set(tpl, "choices.0.finish_reason", finishReason)

		/* usage 只在 response.completed 事件中存在，提取并存入 state */
		if usage := root.Get("response.usage"); usage.Exists() {
			details := normalizeChatUsage(usage)
			state.setUsage(details)
			if !usageFinalSeparateChunk {
				tpl = setChatUsage(tpl, details)
			}
		}

	case "response.output_item.added":
		item := root.Get("item")
		if !item.Exists() || item.Get("type").String() != "function_call" {
			return nil
		}
		state.HasToolCall = true
		state.FunctionCallIndex++
		state.HasReceivedArgsDelta = false
		state.HasToolCallAnnounced = true

		fc := `{"index":0,"id":"","type":"function","function":{"name":"","arguments":""}}`
		fc, _ = sjson.Set(fc, "index", state.FunctionCallIndex)
		fc, _ = sjson.Set(fc, "id", item.Get("call_id").String())
		name := item.Get("name").String()
		if orig, ok := reverseToolMap[name]; ok {
			name = orig
		}
		fc, _ = sjson.Set(fc, "function.name", name)
		fc, _ = sjson.Set(fc, "function.arguments", "")

		tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls", `[]`)
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls.-1", fc)

	case "response.function_call_arguments.delta":
		state.HasToolCall = true
		state.HasReceivedArgsDelta = true
		fc := `{"index":0,"function":{"arguments":""}}`
		fc, _ = sjson.Set(fc, "index", state.FunctionCallIndex)
		fc, _ = sjson.Set(fc, "function.arguments", root.Get("delta").String())
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls", `[]`)
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls.-1", fc)

	case "response.function_call_arguments.done":
		state.HasToolCall = true
		if state.HasReceivedArgsDelta {
			return nil
		}
		fc := `{"index":0,"function":{"arguments":""}}`
		fc, _ = sjson.Set(fc, "index", state.FunctionCallIndex)
		fc, _ = sjson.Set(fc, "function.arguments", root.Get("arguments").String())
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls", `[]`)
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls.-1", fc)

	case "response.output_item.done":
		item := root.Get("item")
		if !item.Exists() {
			return nil
		}
		if item.Get("type").String() == "image_generation_call" {
			/* 把最终图片以 Markdown data URL 形式作为一条 content chunk 推给客户端，
			 * 兼容标准 Chat Completions 流式协议。 */
			r := item.Get("result").String()
			if r == "" {
				return nil
			}
			ext := item.Get("output_format").String()
			if ext == "" {
				ext = "png"
			}
			md := "\n![image](data:image/" + ext + ";base64," + r + ")"
			state.HasText = true
			tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
			tpl, _ = sjson.Set(tpl, "choices.0.delta.content", md)
			return []string{tpl}
		}
		if item.Get("type").String() != "function_call" {
			return nil
		}
		state.HasToolCall = true
		if state.HasToolCallAnnounced {
			state.HasToolCallAnnounced = false
			return nil
		}
		state.FunctionCallIndex++
		fc := `{"index":0,"id":"","type":"function","function":{"name":"","arguments":""}}`
		fc, _ = sjson.Set(fc, "index", state.FunctionCallIndex)
		fc, _ = sjson.Set(fc, "id", item.Get("call_id").String())
		name := item.Get("name").String()
		if orig, ok := reverseToolMap[name]; ok {
			name = orig
		}
		fc, _ = sjson.Set(fc, "function.name", name)
		fc, _ = sjson.Set(fc, "function.arguments", item.Get("arguments").String())
		tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls", `[]`)
		tpl, _ = sjson.SetRaw(tpl, "choices.0.delta.tool_calls.-1", fc)

	case "error":
		state.UpstreamErrCode = root.Get("error.type").String()
		if state.UpstreamErrCode == "" {
			state.UpstreamErrCode = root.Get("error.code").String()
		}
		state.UpstreamErrMsg = root.Get("error.message").String()
		return nil

	case "response.failed":
		if code := root.Get("response.error.code").String(); code != "" {
			state.UpstreamErrCode = code
		} else if t := root.Get("response.error.type").String(); t != "" {
			state.UpstreamErrCode = t
		}
		if msg := root.Get("response.error.message").String(); msg != "" {
			state.UpstreamErrMsg = msg
		}
		return nil

	default:
		if strings.Contains(dataType, "reasoning") && strings.HasSuffix(dataType, ".delta") {
			if delta := root.Get("delta"); delta.Exists() && delta.String() != "" {
				state.HasReasoning = true
				tpl, _ = sjson.Set(tpl, "choices.0.delta.role", "assistant")
				tpl, _ = sjson.Set(tpl, "choices.0.delta.reasoning_content", delta.String())
				return []string{tpl}
			}
		}
		return nil
	}

	return []string{tpl}
}

/**
 * BuildChatCompletionStreamUsageOnlyChunk 生成 OpenAI 流式收尾 chunk：choices 为 []，仅含 usage（与 stream_options.include_usage 一致，供 One API 等网关解析计费）。
 */
func BuildChatCompletionStreamUsageOnlyChunk(state *StreamState) string {
	chunk := `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[]}`
	chunk, _ = sjson.Set(chunk, "id", state.ResponseID)
	chunk, _ = sjson.Set(chunk, "created", state.CreatedAt)
	chunk, _ = sjson.Set(chunk, "model", state.Model)
	return setChatUsage(chunk, state.usageDetails())
}

/**
 * ConvertStreamSSEToNonStreamResponse 将上游 Codex SSE 流聚合为 OpenAI Chat Completions 非流式 JSON。
 * 用于 response.completed.response.output 为空但流式 delta 正常的场景。
 */
func ConvertStreamSSEToNonStreamResponse(data []byte, model string, reverseToolMap map[string]string) (string, bool) {
	state := NewStreamState(model)
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	type toolCallAgg struct {
		ID        string
		Type      string
		Name      string
		Arguments string
	}
	toolCalls := make(map[int]*toolCallAgg)
	var toolOrder []int
	ensureToolCall := func(index int) *toolCallAgg {
		if index < 0 {
			index = len(toolOrder)
		}
		if tc := toolCalls[index]; tc != nil {
			return tc
		}
		tc := &toolCallAgg{Type: "function"}
		toolCalls[index] = tc
		toolOrder = append(toolOrder, index)
		return tc
	}

	finishReason := ""
	usageExists := false
	usageDetails := chatUsageDetails{}
	for _, rawLine := range bytes.Split(data, []byte("\n")) {
		line := bytes.TrimSpace(rawLine)
		if len(line) == 0 || bytes.Equal(line, []byte("data: [DONE]")) {
			continue
		}
		chunks := ConvertStreamChunk(context.Background(), line, state, reverseToolMap, false)
		for _, chunk := range chunks {
			root := gjson.Parse(chunk)
			choice := root.Get("choices.0")
			if v := choice.Get("delta.content"); v.Exists() {
				if s := v.String(); s != "" {
					contentBuilder.WriteString(s)
				}
			}
			if v := choice.Get("delta.reasoning_content"); v.Exists() {
				if s := v.String(); s != "" {
					reasoningBuilder.WriteString(s)
				}
			}
			if arr := choice.Get("delta.tool_calls"); arr.IsArray() {
				for _, rawTC := range arr.Array() {
					idx := int(rawTC.Get("index").Int())
					tc := ensureToolCall(idx)
					if v := rawTC.Get("id"); v.Exists() && v.String() != "" {
						tc.ID = v.String()
					}
					if v := rawTC.Get("type"); v.Exists() && v.String() != "" {
						tc.Type = v.String()
					}
					if v := rawTC.Get("function.name"); v.Exists() && v.String() != "" {
						tc.Name = v.String()
					}
					if v := rawTC.Get("function.arguments"); v.Exists() {
						tc.Arguments += v.String()
					}
				}
			}
			if v := choice.Get("finish_reason"); v.Exists() && v.String() != "" {
				finishReason = v.String()
			}
			if usage := root.Get("usage"); usage.Exists() {
				usageExists = true
				usageDetails = normalizeChatUsage(usage)
			}
		}
	}

	if state.usageDetails().hasAny() {
		usageExists = true
		usageDetails = state.usageDetails()
	}

	hasOutput := contentBuilder.Len() > 0 || reasoningBuilder.Len() > 0 || len(toolOrder) > 0
	if !hasOutput {
		return "", false
	}
	if finishReason == "" {
		if len(toolOrder) > 0 {
			finishReason = "tool_calls"
		} else {
			finishReason = "stop"
		}
	}

	tpl := `{"id":"","object":"chat.completion","created":0,"model":"","choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null}]}`
	if state.ResponseID != "" {
		tpl, _ = sjson.Set(tpl, "id", state.ResponseID)
	}
	created := state.CreatedAt
	if created <= 0 {
		created = time.Now().Unix()
	}
	tpl, _ = sjson.Set(tpl, "created", created)
	outModel := state.Model
	if outModel == "" {
		outModel = model
	}
	tpl, _ = sjson.Set(tpl, "model", outModel)
	if content := contentBuilder.String(); content != "" {
		tpl, _ = sjson.Set(tpl, "choices.0.message.content", content)
	}
	if reasoning := reasoningBuilder.String(); reasoning != "" {
		tpl, _ = sjson.Set(tpl, "choices.0.message.reasoning_content", reasoning)
	}
	if len(toolOrder) > 0 {
		tpl, _ = sjson.SetRaw(tpl, "choices.0.message.tool_calls", `[]`)
		for _, idx := range toolOrder {
			tc := toolCalls[idx]
			if tc == nil {
				continue
			}
			raw := `{"id":"","type":"function","function":{"name":"","arguments":""}}`
			raw, _ = sjson.Set(raw, "id", tc.ID)
			if tc.Type != "" {
				raw, _ = sjson.Set(raw, "type", tc.Type)
			}
			raw, _ = sjson.Set(raw, "function.name", tc.Name)
			raw, _ = sjson.Set(raw, "function.arguments", tc.Arguments)
			tpl, _ = sjson.SetRaw(tpl, "choices.0.message.tool_calls.-1", raw)
		}
	}
	tpl, _ = sjson.Set(tpl, "choices.0.finish_reason", finishReason)
	if usageExists {
		tpl = setChatUsage(tpl, usageDetails)
	}
	return tpl, true
}

/**
 * ConvertNonStreamResponse 将 Codex 非流式响应转换为 OpenAI Chat Completions 格式
 *
 * @param rawJSON - Codex 完整响应 JSON（response.completed 事件的 data 部分）
 * @param reverseToolMap - 缩短名→原始名的工具名映射
 * @returns string - OpenAI Chat Completions 格式的 JSON 字符串
 */
func ConvertNonStreamResponse(rawJSON []byte, reverseToolMap map[string]string) (string, bool) {
	root := gjson.ParseBytes(rawJSON)
	if root.Get("type").String() != "response.completed" {
		return "", false
	}

	resp := root.Get("response")
	tpl := `{"id":"","object":"chat.completion","created":0,"model":"","choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null}]}`

	if v := resp.Get("model"); v.Exists() {
		tpl, _ = sjson.Set(tpl, "model", v.String())
	}
	if v := resp.Get("created_at"); v.Exists() {
		tpl, _ = sjson.Set(tpl, "created", v.Int())
	} else {
		tpl, _ = sjson.Set(tpl, "created", time.Now().Unix())
	}
	if v := resp.Get("id"); v.Exists() {
		tpl, _ = sjson.Set(tpl, "id", v.String())
	}

	/* usage */
	if usage := resp.Get("usage"); usage.Exists() {
		tpl = setChatUsage(tpl, normalizeChatUsage(usage))
	}

	/* 处理 output 数组；先收集顶层 reasoning 相关字段（部分上游放在 response 下） */
	var reasoningBuilder strings.Builder
	if t := resp.Get("reasoning_summary.text").String(); t != "" {
		reasoningBuilder.WriteString(t)
	}
	if t := resp.Get("reasoning_summary").String(); t != "" && reasoningBuilder.Len() == 0 {
		reasoningBuilder.WriteString(t)
	}
	output := resp.Get("output")
	hasOutput := false
	if output.IsArray() {
		var contentBuilder strings.Builder
		var toolCalls []string

		for _, item := range output.Array() {
			switch item.Get("type").String() {
			case "reasoning":
				if summary := item.Get("summary"); summary.IsArray() {
					for _, si := range summary.Array() {
						if si.Get("type").String() == "summary_text" {
							if t := si.Get("text").String(); t != "" {
								reasoningBuilder.WriteString(t)
							}
						}
					}
				}
				/* Responses API：正文思维链在 content[] 的 reasoning_text 中（与 CLIProxy / 官方文档一致） */
				if content := item.Get("content"); content.IsArray() {
					for _, ci := range content.Array() {
						ct := ci.Get("type").String()
						if ct == "reasoning_text" || ct == "text" {
							if t := ci.Get("text").String(); t != "" {
								reasoningBuilder.WriteString(t)
							}
						}
					}
				}
				if txt := item.Get("text").String(); txt != "" {
					reasoningBuilder.WriteString(txt)
				}
			case "reasoning_text":
				if t := item.Get("text").String(); t != "" {
					reasoningBuilder.WriteString(t)
				}
				if content := item.Get("content"); content.IsArray() {
					for _, ci := range content.Array() {
						if t := ci.Get("text").String(); t != "" {
							reasoningBuilder.WriteString(t)
						}
					}
				}
			case "content_part":
				part := item.Get("part")
				if part.Get("type").String() == "reasoning_text" {
					if t := part.Get("text").String(); t != "" {
						reasoningBuilder.WriteString(t)
					}
				}
			case "message":
				if content := item.Get("content"); content.IsArray() {
					for _, ci := range content.Array() {
						if ci.Get("type").String() == "output_text" {
							if t := ci.Get("text").String(); t != "" {
								contentBuilder.WriteString(t)
							}
						}
					}
				}
			case "function_call":
				fc := `{"id":"","type":"function","function":{"name":"","arguments":""}}`
				if v := item.Get("call_id"); v.Exists() {
					fc, _ = sjson.Set(fc, "id", v.String())
				}
				if v := item.Get("name"); v.Exists() {
					n := v.String()
					if orig, ok := reverseToolMap[n]; ok {
						n = orig
					}
					fc, _ = sjson.Set(fc, "function.name", n)
				}
				if v := item.Get("arguments"); v.Exists() {
					fc, _ = sjson.Set(fc, "function.arguments", v.String())
				}
				toolCalls = append(toolCalls, fc)
			case "image_generation_call":
				/* 把图片生成结果以 Markdown data URL 形式追加到 message.content，
				 * 兼容大多数前端（OpenWebUI 等）能直接渲染。 */
				if r := item.Get("result").String(); r != "" {
					ext := item.Get("output_format").String()
					if ext == "" {
						ext = "png"
					}
					if contentBuilder.Len() > 0 {
						contentBuilder.WriteString("\n")
					}
					contentBuilder.WriteString("![image](data:image/")
					contentBuilder.WriteString(ext)
					contentBuilder.WriteString(";base64,")
					contentBuilder.WriteString(r)
					contentBuilder.WriteString(")")
				}
			}
		}

		contentText := contentBuilder.String()
		reasoningText := reasoningBuilder.String()
		if contentText != "" {
			hasOutput = true
			tpl, _ = sjson.Set(tpl, "choices.0.message.content", contentText)
		}
		if reasoningText != "" {
			hasOutput = true
			tpl, _ = sjson.Set(tpl, "choices.0.message.reasoning_content", reasoningText)
		}
		if len(toolCalls) > 0 {
			hasOutput = true
			tpl, _ = sjson.SetRaw(tpl, "choices.0.message.tool_calls", `[]`)
			for _, tc := range toolCalls {
				tpl, _ = sjson.SetRaw(tpl, "choices.0.message.tool_calls.-1", tc)
			}
		}
	}
	/* 仅有顶层 reasoning（无 output 数组或 output 为空）时也写入 reasoning_content */
	if reasoningBuilder.Len() > 0 && gjson.Get(tpl, "choices.0.message.reasoning_content").String() == "" {
		tpl, _ = sjson.Set(tpl, "choices.0.message.reasoning_content", reasoningBuilder.String())
		hasOutput = true
	}

	/* finish_reason */
	if resp.Get("status").String() == "completed" {
		toolCalls := gjson.Get(tpl, "choices.0.message.tool_calls")
		if toolCalls.IsArray() && len(toolCalls.Array()) > 0 {
			tpl, _ = sjson.Set(tpl, "choices.0.finish_reason", "tool_calls")
		} else {
			tpl, _ = sjson.Set(tpl, "choices.0.finish_reason", "stop")
		}
	}

	return tpl, hasOutput
}
