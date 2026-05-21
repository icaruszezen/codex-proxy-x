package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

const defaultImageGenerationModel = "gpt-5.5-image"

type imageGenerationParams struct {
	Prompt         string
	RequestModel   string
	ExecutorModel  string
	ResponseFormat string
	OutputFormat   string
}

func (h *ProxyHandler) handleImageGenerations(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	params, ok := h.parseImageGenerationParams(ctx, body)
	if !ok {
		return
	}

	requestBody, err := buildImageGenerationResponsesRequest(params)
	if err != nil {
		sendError(ctx, fasthttp.StatusInternalServerError, "构造图片生成请求失败", "server_error")
		return
	}

	log.Debugf("收到 Images Generations 请求: model=%s upstream_model=%s output_format=%s", params.RequestModel, params.ExecutorModel, params.OutputFormat)

	result, execErr := h.executor.ExecuteResponsesNonStream(ctx, h.buildRetryConfig(), requestBody, params.ExecutorModel)
	if execErr != nil {
		handleExecutorError(ctx, execErr)
		return
	}

	responseBody, err := convertCodexResponseToImageGenerationResponse(result, params.ResponseFormat)
	if err != nil {
		sendError(ctx, fasthttp.StatusBadGateway, err.Error(), "bad_gateway")
		return
	}

	RecordRequest()
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(responseBody)
}

func (h *ProxyHandler) parseImageGenerationParams(ctx *fasthttp.RequestCtx, body []byte) (imageGenerationParams, bool) {
	var params imageGenerationParams
	if len(body) == 0 {
		sendError(ctx, fasthttp.StatusBadRequest, "读取请求体失败", "invalid_request_error")
		return params, false
	}
	if !gjson.ValidBytes(body) {
		sendError(ctx, fasthttp.StatusBadRequest, "请求体不是有效 JSON", "invalid_request_error")
		return params, false
	}

	prompt := strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	if prompt == "" {
		sendError(ctx, fasthttp.StatusBadRequest, "缺少 prompt 字段", "invalid_request_error")
		return params, false
	}

	if nNode := gjson.GetBytes(body, "n"); nNode.Exists() {
		n := int(nNode.Int())
		if n != 1 {
			sendError(ctx, fasthttp.StatusBadRequest, "当前 /v1/images/generations 仅支持 n=1", "invalid_request_error")
			return params, false
		}
	}

	responseFormat := strings.TrimSpace(gjson.GetBytes(body, "response_format").String())
	if responseFormat == "" {
		responseFormat = "b64_json"
	}
	if responseFormat != "b64_json" {
		sendError(ctx, fasthttp.StatusBadRequest, "当前 /v1/images/generations 仅支持 response_format=b64_json", "invalid_request_error")
		return params, false
	}

	outputFormat := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "output_format").String()))
	if outputFormat == "" {
		outputFormat = "png"
	}
	if outputFormat == "jpg" {
		outputFormat = "jpeg"
	}
	switch outputFormat {
	case "png", "jpeg", "webp":
	default:
		sendError(ctx, fasthttp.StatusBadRequest, "output_format 仅支持 png、jpeg、webp", "invalid_request_error")
		return params, false
	}

	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	requestModel, executorModel, hasFast, explicitImage := normalizeImageGenerationModels(model)
	if hasFast && !h.enableModelFast {
		sendError(ctx, fasthttp.StatusBadRequest, "模型后缀 -fast 已禁用", "invalid_request_error")
		return params, false
	}
	if explicitImage && !h.enableModelImage {
		sendError(ctx, fasthttp.StatusBadRequest, "模型后缀 -image 已禁用", "invalid_request_error")
		return params, false
	}

	params.Prompt = prompt
	params.RequestModel = requestModel
	params.ExecutorModel = executorModel
	params.ResponseFormat = responseFormat
	params.OutputFormat = outputFormat
	return params, true
}

func normalizeImageGenerationModels(model string) (requestModel string, executorModel string, hasFast bool, explicitImage bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultImageGenerationModel
	}

	base := model
	for {
		lower := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lower, "-fast") && len(base) > len("-fast"):
			hasFast = true
			base = base[:len(base)-len("-fast")]
		case strings.HasSuffix(lower, "-image") && len(base) > len("-image"):
			explicitImage = true
			base = base[:len(base)-len("-image")]
		default:
			requestModel = strings.TrimSpace(base)
			if requestModel == "" {
				requestModel = strings.TrimSuffix(defaultImageGenerationModel, "-image")
			}
			executorModel = requestModel + "-image"
			if hasFast {
				executorModel += "-fast"
			}
			return requestModel, executorModel, hasFast, explicitImage
		}
	}
}

func buildImageGenerationResponsesRequest(params imageGenerationParams) ([]byte, error) {
	payload := map[string]any{
		"model": params.RequestModel,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": params.Prompt},
				},
			},
		},
		"tools": []map[string]string{
			{"type": "image_generation", "output_format": params.OutputFormat},
		},
		"tool_choice": "auto",
		"store":       false,
	}
	return json.Marshal(payload)
}

func convertCodexResponseToImageGenerationResponse(resp []byte, responseFormat string) ([]byte, error) {
	if responseFormat != "b64_json" {
		return nil, fmt.Errorf("当前仅支持 response_format=b64_json")
	}

	output := gjson.GetBytes(resp, "output")
	if !output.IsArray() {
		return nil, fmt.Errorf("上游响应缺少图片输出")
	}

	data := make([]map[string]string, 0, 1)
	for _, item := range output.Array() {
		if item.Get("type").String() != "image_generation_call" {
			continue
		}
		result := item.Get("result").String()
		if result == "" {
			continue
		}
		entry := map[string]string{"b64_json": result}
		if revised := item.Get("revised_prompt").String(); revised != "" {
			entry["revised_prompt"] = revised
		}
		data = append(data, entry)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("上游响应未包含 image_generation_call.result")
	}

	return json.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	})
}
