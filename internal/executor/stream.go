package executor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/thinking"
	"codex-proxy/internal/translator"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CodexResponsesStream 上游 /responses 已返回 2xx 后的可读流（由 Pump* 负责关闭 Body）。
type CodexResponsesStream struct {
	body         io.ReadCloser
	account      *auth.Account
	Attempts     int
	BaseModel    string
	ConvertDur   time.Duration
	SendDur      time.Duration
	reverseTools map[string]string
	/* IncludeUsage 为 true 时按 OpenAI stream_options.include_usage 在 [DONE] 前追加 choices 为 [] 的 usage 块 */
	IncludeUsage bool
	/* pumpRounds Pump 阶段最多执行的读循环轮数（含首轮）；换号重连次数 = pumpRounds-1，与 max-retry 对齐 */
	pumpRounds int
	/* reopenExcluded 已在 pump 阶段因上游读失败而排除的凭据路径，避免 reopen 再次选到同一账号 */
	reopenExcluded map[string]bool
	/* reopenFn 在 Pump 阶段遇可重试上游读错误且尚未通过 w 向响应体写入任何 SSE 字节时重建上游（可换号）。
	 * 判定仅看响应体：HTTP 状态行/响应头由 handler 在 SetBodyStreamWriter 之外发送，不计入「已发送」。
	 * 由 OpenCodexResponsesStream 设置；nil 表示不支持 pump 阶段重试。*/
	reopenFn func(ctx context.Context) (io.ReadCloser, CodexResponsesMeta, error)
	/* debugUpstreamStream 为 true 时 Info 打印上游原始 SSE（配置 debug-upstream-stream） */
	debugUpstreamStream bool
	/* dropPartialImage 为 true 时 PumpRawSSE 丢弃 response.image_generation_call.partial_image 帧 */
	dropPartialImage bool
}

/* Body 返回当前上游响应体，供外部 pump 读取 SSE */
func (s *CodexResponsesStream) Body() io.ReadCloser { return s.body }

/* Account 返回当前关联的账号 */
func (s *CodexResponsesStream) Account() *auth.Account { return s.account }

// CodexResponsesMeta bundles metadata returned by openCodexResponsesBody.
type CodexResponsesMeta struct {
	Account      *auth.Account
	Attempts     int
	BaseModel    string
	ConvertDur   time.Duration
	SendDur      time.Duration
	ReverseTools map[string]string
}

/* prefixThenRestCloser 首读已拉取的字节 + 剩余 Body，供在返回给客户端前探测 GOAWAY 后仍能透传已读数据 */
type prefixThenRestCloser struct {
	prefix []byte
	off    int
	rest   io.ReadCloser
}

func (p *prefixThenRestCloser) Read(b []byte) (int, error) {
	if p.off < len(p.prefix) {
		n := copy(b, p.prefix[p.off:])
		p.off += n
		return n, nil
	}
	if p.rest == nil {
		return 0, io.EOF
	}
	return p.rest.Read(b)
}

func (p *prefixThenRestCloser) Close() error {
	if p.rest == nil {
		return nil
	}
	err := p.rest.Close()
	p.rest = nil
	return err
}

/* bufioTailCloser 将 bufio.Reader 与底层 Body 绑定 Close，供首读探测后继续读剩余流 */
type bufioTailCloser struct {
	r *bufio.Reader
	c io.Closer
}

func (b *bufioTailCloser) Read(p []byte) (int, error) { return b.r.Read(p) }

func (b *bufioTailCloser) Close() error {
	if b.c != nil {
		return b.c.Close()
	}
	return nil
}

// upstreamStreamLogMaxBytes 单条日志中上游 SSE 行/块的最大字节（超出截断，避免撑爆日志）
const upstreamStreamLogMaxBytes = 65536

func logUpstreamStreamChunk(tag, baseModel, accountEmail string, p []byte) {
	if len(p) == 0 {
		return
	}
	if len(p) <= upstreamStreamLogMaxBytes {
		log.Infof("[upstream_stream:%s] model=%s account=%s bytes=%d body=%s", tag, baseModel, accountEmail, len(p), string(p))
		return
	}
	log.Infof("[upstream_stream:%s] model=%s account=%s bytes=%d body_prefix=%s ...(%d more bytes truncated)",
		tag, baseModel, accountEmail, len(p), string(p[:upstreamStreamLogMaxBytes]), len(p)-upstreamStreamLogMaxBytes)
}

// LogUpstreamStreamChunk 将上游 SSE 片段打到 Info 日志（Claude 等路径复用；与 debug-upstream-stream 联用）
func LogUpstreamStreamChunk(tag, baseModel, accountEmail string, p []byte) {
	logUpstreamStreamChunk(tag, baseModel, accountEmail, p)
}

// codexStreamPumpRounds 流式 Pump 阶段换号上限：首轮 + (1+maxRetry) 次换号重连，与 sendWithRetry 选号次数同量级。
func codexStreamPumpRounds(maxRetry int) int {
	n := 2 + maxRetry
	if n < 2 {
		return 2
	}
	return n
}

// openCodexResponsesBody 与 OpenCodexResponsesStream 相同：选号、sendWithRetry、首读探测空体/可重试读错并换号。
// Claude 原始流等非 Pump 路径也经此打开，避免 200 + 空 body 导致客户端 SSE 体完全无字节。
func (e *Executor) openCodexResponsesBody(ctx context.Context, rc RetryConfig, requestBody []byte, model string) (bodyRC io.ReadCloser, meta CodexResponsesMeta, err error) {
	convertStart := time.Now()
	thBody, bm, isImage := thinking.ApplyThinking(requestBody, model)
	meta.BaseModel = bm
	codexBody := translator.ConvertOpenAIRequestToCodex(meta.BaseModel, thBody, true, isImage)
	meta.ConvertDur = time.Since(convertStart)
	apiURL := e.baseURL + "/responses"
	sendStart := time.Now()

	readRounds := 1 + rc.EmptyRetryMax
	if readRounds < 2 {
		readRounds = 2
	}
	if rc.EmptyRetryMax < 0 {
		readRounds = 2
	}
	excluded := make(map[string]bool)

	for round := 0; round < readRounds; round++ {
		if ctx.Err() != nil {
			meta.SendDur = time.Since(sendStart)
			return nil, meta, ctx.Err()
		}
		rcExcl := MergeRetryConfigExcluded(rc, excluded)
		httpResp, acc, att, serr := e.sendWithRetry(ctx, rcExcl, model, apiURL, codexBody, true)
		if serr != nil {
			meta.SendDur = time.Since(sendStart)
			return nil, meta, serr
		}

		/* 行缓冲探测：若首段 SSE 仅为额度用尽（如 usage_limit_reached），换号且不把错误字节交给 PumpRawSSE 透传 */
		br := bufio.NewReader(httpResp.Body)
		var prelude bytes.Buffer
		const maxProbeLines = 96
		const maxProbeBytes = 128 * 1024
		hitQuota := false
		var probeErr error
		for i := 0; i < maxProbeLines && prelude.Len() < maxProbeBytes; i++ {
			line, rerr := br.ReadBytes('\n')
			if len(line) > 0 {
				prelude.Write(line)
			}
			if upstreamPrefixIndicatesUsageQuotaExceeded(prelude.Bytes()) {
				hitQuota = true
				probeErr = rerr
				break
			}
			if rerr != nil {
				probeErr = rerr
				break
			}
		}

		if hitQuota && round+1 < readRounds {
			_ = httpResp.Body.Close()
			acc.RecordFailure()
			handleAccountError(acc, 429, codexQuotaPayloadForCooldown(prelude.Bytes()))
			if rc.On429RecoveryFn != nil {
				go rc.On429RecoveryFn(context.Background(), acc)
			}
			excluded[acc.FilePath] = true
			log.Warnf("responses-stream 上游首段为额度用尽类错误，换号重试 (%d/%d) account=%s", round+1, readRounds, acc.GetEmail())
			continue
		}

		meta.SendDur = time.Since(sendStart)

		if probeErr != nil && probeErr != io.EOF {
			_ = httpResp.Body.Close()
			acc.RecordFailure()
			if isRetryableUpstreamReadErr(probeErr) && round+1 < readRounds {
				excluded[acc.FilePath] = true
				log.Warnf("responses-stream 首读失败，换号/重建连接重试 (%d/%d) account=%s: %v", round+1, readRounds, acc.GetEmail(), wrapReadErr(probeErr))
				continue
			}
			return nil, meta, fmt.Errorf("读取上游流失败: %w", wrapReadErr(probeErr))
		}

		pr := prelude.Bytes()
		if len(pr) == 0 && probeErr == io.EOF {
			_ = httpResp.Body.Close()
			acc.RecordFailure()
			if round+1 < readRounds {
				excluded[acc.FilePath] = true
				log.Warnf("responses-stream 上游立即 EOF，换号重试 (%d/%d) account=%s", round+1, readRounds, acc.GetEmail())
				continue
			}
			return nil, meta, fmt.Errorf("读取上游流失败: 空响应")
		}

		tail := &bufioTailCloser{r: br, c: httpResp.Body}
		var bodyOut io.ReadCloser = tail
		if len(pr) > 0 {
			bodyOut = &prefixThenRestCloser{prefix: append([]byte(nil), pr...), rest: tail}
		}

		meta.Account = acc
		meta.Attempts = att
		meta.ReverseTools = translator.BuildReverseToolNameMap(requestBody)
		return bodyOut, meta, nil
	}
	meta.SendDur = time.Since(sendStart)
	return nil, meta, fmt.Errorf("读取上游流失败")
}

// OpenCodexResponsesStream 完成选号、重试与首包前的 HTTP 往返；调用方在写入客户端 SSE 头后再 Pump。
// 在返回前做一次首读：若立即遇 GOAWAY 等可重试错误则关连接换号重来，减少「已 200 后 pump 才断」的失败率。
func (e *Executor) OpenCodexResponsesStream(ctx context.Context, rc RetryConfig, requestBody []byte, model string) (*CodexResponsesStream, error) {
	bodyRC, meta, err := e.openCodexResponsesBody(ctx, rc, requestBody, model)
	if err != nil {
		return nil, err
	}
	includeUsage := gjson.GetBytes(requestBody, "stream_options.include_usage").Bool()
	s := &CodexResponsesStream{
		body:                bodyRC,
		account:             meta.Account,
		Attempts:            meta.Attempts,
		BaseModel:           meta.BaseModel,
		ConvertDur:          meta.ConvertDur,
		SendDur:             meta.SendDur,
		reverseTools:        meta.ReverseTools,
		IncludeUsage:        includeUsage,
		pumpRounds:          codexStreamPumpRounds(rc.MaxRetry),
		reopenExcluded:      make(map[string]bool),
		debugUpstreamStream: rc.DebugUpstreamStream,
		dropPartialImage:    e.dropPartialImage,
	}
	s.reopenFn = func(ctx context.Context) (io.ReadCloser, CodexResponsesMeta, error) {
		rcEx := MergeRetryConfigExcluded(rc, s.reopenExcluded)
		return e.openCodexResponsesBody(ctx, rcEx, requestBody, model)
	}
	return s, nil
}

// CollectedCodexSSE 是下游非流式请求复用上游流式通道后，在内存中收集到的完整 Codex SSE。
type CollectedCodexSSE struct {
	Data       []byte
	Account    *auth.Account
	Attempts   int
	BaseModel  string
	ConvertDur time.Duration
	SendDur    time.Duration
	Lines      int
	CollectDur time.Duration
}

// CollectCodexResponsesSSE 打开上游 /responses SSE，缓存直到 response.completed 后返回，不向下游写入任何字节。
func (e *Executor) CollectCodexResponsesSSE(ctx context.Context, rc RetryConfig, requestBody []byte, model string) (*CollectedCodexSSE, error) {
	bridges := CodexStreamOpenBridgeMax(rc.MaxRetry)
	var lastErr error
	for b := 0; b < bridges; b++ {
		openCtx := ctx
		if b > 0 {
			openCtx = context.Background()
		}
		s, err := e.OpenCodexResponsesStream(openCtx, rc, requestBody, model)
		if err != nil {
			lastErr = err
			if b < bridges-1 && IsRetryableOpenCodexError(err) {
				log.Warnf("codex nonstream 聚合 open 全量重连 %d/%d: %v", b+1, bridges, err)
				continue
			}
			return nil, err
		}
		collected, err := s.CollectUntilCompleted(context.Background())
		if err == nil {
			return collected, nil
		}
		lastErr = err
		if b >= bridges-1 || !IsRetryableStreamPumpForBridge(err) {
			return nil, err
		}
		log.Warnf("codex nonstream 聚合 pump 全量重连 %d/%d: %v", b+1, bridges, err)
	}
	return nil, lastErr
}

// CollectUntilCompleted 读取当前上游 SSE 到 response.completed；若尚未返回给客户端，可在失败或空响应时换号重连。
func (s *CodexResponsesStream) CollectUntilCompleted(ctx context.Context) (*CollectedCodexSSE, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	collectStart := time.Now()
	var lastErr error
	totalLines := 0

	for round := 0; round < s.pumpRounds; round++ {
		if ctx.Err() != nil {
			if s.body != nil {
				_ = s.body.Close()
			}
			return nil, ctx.Err()
		}

		var buf bytes.Buffer
		scanner := bufio.NewScanner(s.body)
		scanner.Buffer(make([]byte, scannerInitSize), scannerMaxSize)
		completed := false
		localLines := 0
		firstType := ""
		lastType := ""

		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			localLines++
			totalLines++
			if s.debugUpstreamStream {
				ae := ""
				if s.account != nil {
					ae = s.account.GetEmail()
				}
				logUpstreamStreamChunk("nonstream_collect_line", s.BaseModel, ae, line)
			}
			buf.Write(line)
			buf.WriteByte('\n')

			trimmed := bytes.TrimSpace(line)
			if !bytes.HasPrefix(trimmed, []byte("data:")) {
				continue
			}
			rawJSON := bytes.TrimSpace(trimmed[len("data:"):])
			if len(rawJSON) == 0 || bytes.Equal(rawJSON, []byte("[DONE]")) {
				continue
			}
			dataType := gjson.GetBytes(rawJSON, "type").String()
			if dataType != "" {
				if firstType == "" {
					firstType = dataType
				}
				lastType = dataType
			}
			if dataType == "response.completed" {
				completed = true
				break
			}
		}

		scanErr := scanner.Err()
		if completed {
			if s.body != nil {
				_ = s.body.Close()
			}
			return &CollectedCodexSSE{
				Data:       append([]byte(nil), buf.Bytes()...),
				Account:    s.account,
				Attempts:   s.Attempts,
				BaseModel:  s.BaseModel,
				ConvertDur: s.ConvertDur,
				SendDur:    s.SendDur,
				Lines:      totalLines,
				CollectDur: time.Since(collectStart),
			}, nil
		}

		if s.body != nil {
			_ = s.body.Close()
		}
		if scanErr != nil {
			lastErr = wrapReadErr(scanErr)
		} else if buf.Len() == 0 {
			lastErr = fmt.Errorf("%w: responses sse 0 bytes", ErrEmptyResponse)
		} else {
			lastErr = fmt.Errorf("%w: 未收到 response.completed 事件 first_type=%q last_type=%q lines=%d", ErrEmptyResponse, firstType, lastType, localLines)
		}

		if s.account != nil {
			s.account.RecordFailure()
		}
		if round >= s.pumpRounds-1 || s.reopenFn == nil || !PumpShouldReopenNoClientBytes(lastErr) {
			return nil, lastErr
		}
		if s.account != nil && s.account.FilePath != "" {
			s.reopenExcluded[s.account.FilePath] = true
		}
		newBody, newMeta, rerr := s.reopenFn(ctx)
		if rerr != nil {
			return nil, rerr
		}
		failedEmail := ""
		if s.account != nil {
			failedEmail = s.account.GetEmail()
		}
		log.Warnf("nonstream 聚合未得到 completed，换号重试 (%d/%d) account=%s: %v", round+1, s.pumpRounds, failedEmail, lastErr)
		s.account = newMeta.Account
		s.Attempts += newMeta.Attempts
		s.SendDur = newMeta.SendDur
		s.body = newBody
		s.reverseTools = newMeta.ReverseTools
	}
	return nil, lastErr
}

// UpstreamBody 返回当前上游响应体，由调用方在读完后 Close（与 PumpChatCompletion 的 defer 语义一致）。
func (s *CodexResponsesStream) UpstreamBody() io.ReadCloser {
	if s == nil {
		return nil
	}
	return s.body
}

// StreamAccount 当前流绑定的账号（成功/失败记账）。
func (s *CodexResponsesStream) StreamAccount() *auth.Account {
	if s == nil {
		return nil
	}
	return s.account
}

// PumpChatCompletion 将 Codex SSE 转为 OpenAI Chat Completions 块写入 w（仅响应体；HTTP 响应头由 handler 事先设好）。
// 若 pump 遇可重试上游读错误且尚未向响应体写入任何 SSE 消息（chunkCount==0，不含响应头），则换号重连，次数与 max-retry 对齐。
func (s *CodexResponsesStream) PumpChatCompletion(w io.Writer, flush func()) error {
	defer func() { _ = s.body.Close() }()

	streamStart := time.Now()
	var firstChunkAt time.Time
	var completedAt time.Time
	chunkCount := 0
	pumpCtx := context.Background()

	var state *translator.StreamState
	var scanErr error
	var pumpScanLines int
	// 上一轮已在循环内因「空响应」完成换号 reopen，本轮开头勿再关 body
	var skipLeadingReopen bool

	for round := 0; round < s.pumpRounds; round++ {
		if round > 0 && !skipLeadingReopen {
			// 仅当响应体侧尚未写出任何 SSE chunk（HTTP 响应头不算）时可换号；除取消外不因「非 GOAWAY」而拒绝换号
			if chunkCount > 0 || s.reopenFn == nil || !PumpShouldReopenNoClientBytes(scanErr) {
				break
			}
			if fp := s.account.FilePath; fp != "" {
				s.reopenExcluded[fp] = true
			}
			_ = s.body.Close()
			newBody, newMeta, rerr := s.reopenFn(pumpCtx)
			if rerr != nil {
				break
			}
			s.account.RecordFailure()
			log.Warnf("stream pump 上游错误，换号重试 account=%s: %v", s.account.GetEmail(), wrapReadErr(scanErr))
			s.account = newMeta.Account
			s.Attempts += newMeta.Attempts
			s.SendDur = newMeta.SendDur
			s.body = newBody
			s.reverseTools = newMeta.ReverseTools
		}
		skipLeadingReopen = false

		state = translator.NewStreamState(s.BaseModel)
		reverseToolMap := s.reverseTools
		scanner := bufio.NewScanner(s.body)
		scanner.Buffer(make([]byte, scannerInitSize), scannerMaxSize)
		pumpScanLines = 0

		for scanner.Scan() {
			pumpScanLines++
			line := scanner.Bytes()
			if s.debugUpstreamStream {
				ae := ""
				if s.account != nil {
					ae = s.account.GetEmail()
				}
				logUpstreamStreamChunk("chat_sse_line", s.BaseModel, ae, line)
			}
			chunks := translator.ConvertStreamChunk(pumpCtx, line, state, reverseToolMap, s.IncludeUsage)
			for _, chunk := range chunks {
				if firstChunkAt.IsZero() {
					firstChunkAt = time.Now()
				}
				chunkCount++
				_, _ = w.Write(sseDataPrefix)
				_, _ = io.WriteString(w, chunk)
				_, _ = w.Write(sseDataSuffix)
				if flush != nil {
					flush()
				}
			}
			if state.Completed {
				if completedAt.IsZero() {
					completedAt = time.Now()
				}
				break
			}
		}

		scanErr = scanner.Err()
		if scanErr != nil {
			if errors.Is(scanErr, context.Canceled) {
				firstChunkDur := time.Duration(0)
				if !firstChunkAt.IsZero() {
					firstChunkDur = firstChunkAt.Sub(streamStart)
				}
				log.Infof("req summary stream model=%s account=%s attempts=%d convert=%v upstream_open_first_read=%v pump_first_client_chunk=%v pump_to_completed=%v tail_after_completed=%v pump_elapsed=%v chunks=%d e2e_est=%v (canceled)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, firstChunkDur, time.Duration(0), time.Duration(0), time.Since(streamStart), chunkCount, s.SendDur+time.Since(streamStart))
				/* 已向客户端承诺 SSE：无任何 data 时须返回错误，避免 200 + 空体 */
				if chunkCount == 0 {
					return fmt.Errorf("读取流式响应中断: %w", scanErr)
				}
				return nil
			}
			// 非 Canceled 错误：进入下一轮检查是否可换号重试
			continue
		}
		// 扫描正常结束：无正文/工具/思维（含仅有元数据/ error.failed / 空 response.completed），且未向客户端写 chunk → 换号再拉流
		contentEmpty := !state.HasText && !state.HasToolCall && !state.HasReasoning
		if contentEmpty && chunkCount == 0 && s.reopenFn != nil && round < s.pumpRounds-1 {
			if fp := s.account.FilePath; fp != "" {
				s.reopenExcluded[fp] = true
			}
			_ = s.body.Close()
			newBody, newMeta, rerr := s.reopenFn(pumpCtx)
			if rerr != nil {
				log.Warnf("stream pump 上游无有效 chunk（含 SSE error/failed），换号 reopen 失败 round=%d/%d account=%s: %v", round+1, s.pumpRounds, s.account.GetEmail(), rerr)
				break
			}
			failedEmail := s.account.GetEmail()
			s.account.RecordFailure()
			log.Warnf("stream pump 上游空响应/SSE 失败，换号重试 (%d/%d) account=%s", round+1, s.pumpRounds, failedEmail)
			s.account = newMeta.Account
			s.Attempts += newMeta.Attempts
			s.SendDur = newMeta.SendDur
			s.body = newBody
			s.reverseTools = newMeta.ReverseTools
			skipLeadingReopen = true
			continue
		}
		if contentEmpty && chunkCount == 0 && s.reopenFn != nil && round >= s.pumpRounds-1 {
			log.Warnf("stream pump 上游无有效 chunk，已达 pump 内换号上限 (pumpRounds=%d) account=%s %s", s.pumpRounds, s.account.GetEmail(), state.EmptyUpstreamDiag(pumpScanLines))
		}
		break
	}

	if scanErr != nil {
		log.Errorf("读取流式响应失败: %v", scanErr)
		firstChunkDur := time.Duration(0)
		completedDur := time.Duration(0)
		tailAfterCompleted := time.Duration(0)
		if !firstChunkAt.IsZero() {
			firstChunkDur = firstChunkAt.Sub(streamStart)
		}
		if !completedAt.IsZero() {
			completedDur = completedAt.Sub(streamStart)
			tailAfterCompleted = time.Since(completedAt)
		}
		pumpEl := time.Since(streamStart)
		log.Infof("req summary stream model=%s account=%s attempts=%d convert=%v upstream_open_first_read=%v pump_first_client_chunk=%v pump_to_completed=%v tail_after_completed=%v pump_elapsed=%v chunks=%d e2e_est=%v (ERR)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, firstChunkDur, completedDur, tailAfterCompleted, pumpEl, chunkCount, s.SendDur+pumpEl)
		return wrapReadErr(scanErr)
	}

	/* 无正文/工具/思维且未向客户端写任何 chunk：含「上游有 SSE 但空 completed」或仅元数据/失败事件 */
	if !state.HasText && !state.HasToolCall && !state.HasReasoning && chunkCount == 0 {
		firstChunkDur := time.Duration(0)
		completedDur := time.Duration(0)
		tailAfterCompleted := time.Duration(0)
		if !firstChunkAt.IsZero() {
			firstChunkDur = firstChunkAt.Sub(streamStart)
		}
		if !completedAt.IsZero() {
			completedDur = completedAt.Sub(streamStart)
			tailAfterCompleted = time.Since(completedAt)
		}
		pumpEl := time.Since(streamStart)
		log.Infof("req summary stream model=%s account=%s attempts=%d convert=%v upstream_open_first_read=%v pump_first_client_chunk=%v pump_to_completed=%v tail_after_completed=%v pump_elapsed=%v chunks=%d e2e_est=%v (empty)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, firstChunkDur, completedDur, tailAfterCompleted, pumpEl, chunkCount, s.SendDur+pumpEl)
		diag := state.EmptyUpstreamDiag(pumpScanLines)
		log.Warnf("chat stream 上游空响应: model=%s account=%s attempts=%d %s", s.BaseModel, s.account.GetEmail(), s.Attempts, diag)
		return fmt.Errorf("%w: %s", ErrEmptyResponse, diag)
	}

	if !state.Completed {
		finishReason := "stop"
		if state.FunctionCallIndex != -1 {
			finishReason = "tool_calls"
		}
		synth := `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
		synth, _ = sjson.Set(synth, "id", state.ResponseID)
		synth, _ = sjson.Set(synth, "created", state.CreatedAt)
		synth, _ = sjson.Set(synth, "model", state.Model)
		synth, _ = sjson.Set(synth, "choices.0.finish_reason", finishReason)
		chunkCount++
		_, _ = w.Write(sseDataPrefix)
		_, _ = io.WriteString(w, synth)
		_, _ = w.Write(sseDataSuffix)
		if flush != nil {
			flush()
		}
	}

	if s.IncludeUsage {
		usageChunk := translator.BuildChatCompletionStreamUsageOnlyChunk(state)
		chunkCount++
		_, _ = w.Write(sseDataPrefix)
		_, _ = io.WriteString(w, usageChunk)
		_, _ = w.Write(sseDataSuffix)
		if flush != nil {
			flush()
		}
	}

	doneWriteStart := time.Now()
	_, _ = w.Write(sseDoneMarker)
	if flush != nil {
		flush()
	}
	doneWriteDur := time.Since(doneWriteStart)

	if state.UsageInput > 0 || state.UsageOutput > 0 {
		s.account.RecordUsage(state.UsageInput, state.UsageOutput, state.UsageTotal)
	}
	s.account.RecordSuccess()
	firstChunkDur := time.Duration(0)
	completedDur := time.Duration(0)
	tailAfterCompleted := time.Duration(0)
	if !firstChunkAt.IsZero() {
		firstChunkDur = firstChunkAt.Sub(streamStart)
	}
	if !completedAt.IsZero() {
		completedDur = completedAt.Sub(streamStart)
		tailAfterCompleted = time.Since(completedAt)
	}
	pumpEl := time.Since(streamStart)
	log.Infof("req summary stream model=%s account=%s attempts=%d convert=%v upstream_open_first_read=%v pump_first_client_chunk=%v pump_to_completed=%v tail_after_completed=%v done_write=%v pump_elapsed=%v chunks=%d e2e_est=%v", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, firstChunkDur, completedDur, tailAfterCompleted, doneWriteDur, pumpEl, chunkCount, s.SendDur+pumpEl)
	return nil
}

// PumpRawSSE 转发上游 SSE 给客户端（Responses API，仅写 w 即响应体）。
// 若启用 dropPartialImage，会按 SSE 帧解析并丢弃 response.image_generation_call.partial_image（每帧 base64 数 MB），
// 其它事件原样下发；否则保持纯字节透传。
// 若尚未向 w 写入任何字节（已发的 HTTP 响应头不计入），遇读错误、io.EOF 且无字节等均换号重连，次数与 pumpRounds 对齐；除 context.Canceled 外不因「非 GOAWAY」拒绝换号。
func (s *CodexResponsesStream) PumpRawSSE(w io.Writer, flush func()) error {
	defer func() { _ = s.body.Close() }()
	streamStart := time.Now()
	// 仅统计经 w 写入的 SSE 响应体字节；与 fasthttp SetBodyStreamWriter 一致，状态行/响应头不在此 Writer 上。
	sseBodyBytes := 0
	var pumpErr error
	pumpCtx := context.Background()

	useFrameFilter := s.dropPartialImage
	buf := make([]byte, httpBufferSize)

	for round := 0; round < s.pumpRounds; round++ {
		pumpErr = nil
		if useFrameFilter {
			n, fErr := pumpResponsesSSEFiltered(s.body, w, flush, &sseBodyBytes, s.debugUpstreamStream, s.BaseModel, s.account)
			_ = n
			if fErr != nil {
				if errors.Is(fErr, context.Canceled) {
					log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v (canceled)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
					if sseBodyBytes == 0 {
						return fmt.Errorf("读取流式响应中断: %w", fErr)
					}
					return nil
				}
				if fErr == io.EOF {
					if sseBodyBytes > 0 {
						s.account.RecordSuccess()
						log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
						return nil
					}
					pumpErr = io.EOF
				} else {
					pumpErr = fErr
				}
			} else {
				if sseBodyBytes > 0 {
					s.account.RecordSuccess()
					log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
					return nil
				}
				pumpErr = io.EOF
			}
		} else {
			for {
				n, readErr := s.body.Read(buf)
				if n > 0 {
					if s.debugUpstreamStream {
						ae := ""
						if s.account != nil {
							ae = s.account.GetEmail()
						}
						logUpstreamStreamChunk("responses_raw_read", s.BaseModel, ae, buf[:n])
					}
					if _, werr := w.Write(buf[:n]); werr != nil {
						return werr
					}
					sseBodyBytes += n
					if flush != nil {
						flush()
					}
				}
				if readErr != nil {
					if readErr == io.EOF {
						if sseBodyBytes > 0 {
							s.account.RecordSuccess()
							log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
							return nil
						}
						pumpErr = io.EOF
						break
					}
					if errors.Is(readErr, context.Canceled) {
						log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v (canceled)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
						if sseBodyBytes == 0 {
							return fmt.Errorf("读取流式响应中断: %w", readErr)
						}
						return nil
					}
					pumpErr = readErr
					break
				}
			}
		}

		if pumpErr != nil && sseBodyBytes == 0 && s.reopenFn != nil && round < s.pumpRounds-1 {
			if fp := s.account.FilePath; fp != "" {
				s.reopenExcluded[fp] = true
			}
			_ = s.body.Close()
			newBody, newMeta, rerr := s.reopenFn(pumpCtx)
			if rerr != nil {
				log.Warnf("responses-stream raw 零字节，换号 reopen 失败 round=%d/%d account=%s: %v", round+1, s.pumpRounds, s.account.GetEmail(), rerr)
				break
			}
			failedEmail := s.account.GetEmail()
			s.account.RecordFailure()
			log.Warnf("responses-stream raw 尚未发往客户端，换号重试 (%d/%d) account=%s err=%v", round+1, s.pumpRounds, failedEmail, pumpErr)
			s.account = newMeta.Account
			s.Attempts += newMeta.Attempts
			s.SendDur = newMeta.SendDur
			s.body = newBody
			continue
		}

		if pumpErr == io.EOF && sseBodyBytes == 0 {
			log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v (empty)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
			return fmt.Errorf("%w: responses raw sse 0 bytes", ErrEmptyResponse)
		}

		if pumpErr != nil {
			break
		}
		break
	}

	if pumpErr != nil {
		log.Errorf("读取流式响应失败: %v", pumpErr)
		log.Infof("req summary responses-stream model=%s account=%s attempts=%d convert=%v upstream=%v total=%v (ERR)", s.BaseModel, s.account.GetEmail(), s.Attempts, s.ConvertDur, s.SendDur, time.Since(streamStart))
		return wrapReadErr(pumpErr)
	}
	return fmt.Errorf("%w: responses stream pump", ErrEmptyResponse)
}

// countingWriter 统计写入 w 的字节数（用于判断是否已向客户端承诺 SSE 体）
type countingWriter struct {
	w io.Writer
	n *int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	nw, err := c.w.Write(p)
	if nw > 0 {
		*c.n += int64(nw)
	}
	return nw, err
}

type CodexStreamPump func(s *CodexResponsesStream, w io.Writer, flush func()) error

func CodexStreamOpenBridgeMax(maxRetry int) int {
	n := 2 + maxRetry
	if n < 2 {
		return 2
	}
	return n
}
func (e *Executor) RunCodexStreamWithOpenBridges(octx context.Context, rc RetryConfig, requestBody []byte, model string, w io.Writer, flush func(), bridges int, pump CodexStreamPump) error {
	var written int64
	cw := &countingWriter{w: w, n: &written}
	var lastErr error
	for b := 0; b < bridges; b++ {
		ctx := octx
		if b > 0 {
			ctx = context.Background()
		}
		s, err := e.OpenCodexResponsesStream(ctx, rc, requestBody, model)
		if err != nil {
			lastErr = err
			if written == 0 && b < bridges-1 && IsRetryableOpenCodexError(err) {
				log.Warnf("codex stream 全量重连 open %d/%d: %v", b+1, bridges, err)
				continue
			}
			return err
		}
		err = pump(s, cw, flush)
		if err == nil {
			return nil
		}
		lastErr = err
		if written > 0 || !IsRetryableStreamPumpForBridge(err) {
			return err
		}
		if b >= bridges-1 {
			return err
		}
		log.Warnf("codex stream 全量重连 pump %d/%d（响应体尚无字节）: %v", b+1, bridges, err)
	}
	return lastErr
}

// CodexCompactStream /responses/compact 成功后的响应（含待透传头与 Body）。
type CodexCompactStream struct {
	Resp       *http.Response
	Account    *auth.Account
	Attempts   int
	BaseModel  string
	ConvertDur time.Duration
	SendDur    time.Duration
}

// PumpBody 透传 compact 响应体；成功读完时由调用方 RecordSuccess。
func (s *CodexCompactStream) PumpBody(w io.Writer, flush func()) error {
	defer func() { _ = s.Resp.Body.Close() }()
	buf := make([]byte, httpBufferSize)
	for {
		n, err := s.Resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if flush != nil {
				flush()
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return wrapReadErr(err)
		}
	}
}

/* sseFrameDropEventTypes 列出所有需要在转发时丢弃的 data.type；当前仅 partial_image 帧。 */
var sseFrameDropEventTypes = map[string]struct{}{
	"response.image_generation_call.partial_image": {},
}

/* pumpResponsesSSEFiltered 按 SSE 帧扫描上游字节，逐帧重组后写入 w；遇到 data.type 命中黑名单时整帧丢弃。
 * @returns 写出帧数与读取错误（io.EOF 或其它）。
 *
 * 协议简化：上游 /responses 每帧形如 `event: <name>\ndata: <json>\n\n`，且 data 一定是单行 JSON；
 * 因此按行扫描、遇空行结束当前帧即可，无需支持多行 data。
 */
func pumpResponsesSSEFiltered(body io.Reader, w io.Writer, flush func(), bytesWritten *int, debug bool, baseModel string, acc *auth.Account) (int, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, scannerInitSize), scannerMaxSize)
	frames := 0
	var frameBuf []byte
	var dataLine []byte
	flushFrame := func() error {
		if len(frameBuf) == 0 {
			frameBuf = frameBuf[:0]
			dataLine = nil
			return nil
		}
		drop := false
		if len(dataLine) > 0 {
			t := gjson.GetBytes(dataLine, "type").String()
			if _, ok := sseFrameDropEventTypes[t]; ok {
				drop = true
			}
		}
		if !drop {
			if debug {
				ae := ""
				if acc != nil {
					ae = acc.GetEmail()
				}
				logUpstreamStreamChunk("responses_raw_frame", baseModel, ae, frameBuf)
			}
			n, werr := w.Write(frameBuf)
			if n > 0 {
				*bytesWritten += n
			}
			if werr != nil {
				return werr
			}
			n, werr = w.Write([]byte("\n\n"))
			if n > 0 {
				*bytesWritten += n
			}
			if werr != nil {
				return werr
			}
			if flush != nil {
				flush()
			}
			frames++
		}
		frameBuf = frameBuf[:0]
		dataLine = nil
		return nil
	}
	for scanner.Scan() {
		raw := scanner.Bytes()
		// 空行 = 一帧结束
		if len(raw) == 0 {
			if err := flushFrame(); err != nil {
				return frames, err
			}
			continue
		}
		// 累计当前帧（保留原始换行结构，便于客户端按 SSE 解析）
		if len(frameBuf) > 0 {
			frameBuf = append(frameBuf, '\n')
		}
		frameBuf = append(frameBuf, raw...)
		if bytes.HasPrefix(raw, []byte("data:")) {
			dataLine = bytes.TrimSpace(raw[5:])
		}
	}
	if err := flushFrame(); err != nil {
		return frames, err
	}
	if err := scanner.Err(); err != nil {
		return frames, err
	}
	return frames, io.EOF
}
