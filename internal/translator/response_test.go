package translator

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertNonStreamResponsePassesNewAPICacheUsage(t *testing.T) {
	raw := []byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_1",
			"model":"gpt-test",
			"created_at":1710000000,
			"status":"completed",
			"usage":{
				"input_tokens":100,
				"output_tokens":20,
				"total_tokens":120,
				"cache_creation_input_tokens":12,
				"cache_read_input_tokens":34,
				"output_tokens_details":{"reasoning_tokens":5}
			},
			"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]
		}
	}`)

	out, ok := ConvertNonStreamResponse(raw, nil)
	if !ok {
		t.Fatal("expected converted response")
	}
	root := gjson.Parse(out)
	assertInt(t, root, "usage.prompt_tokens", 100)
	assertInt(t, root, "usage.completion_tokens", 20)
	assertInt(t, root, "usage.total_tokens", 120)
	assertInt(t, root, "usage.prompt_tokens_details.cached_tokens", 34)
	assertInt(t, root, "usage.prompt_tokens_details.cache_creation_tokens", 12)
	assertInt(t, root, "usage.prompt_tokens_details.cache_read_tokens", 34)
	assertInt(t, root, "usage.cache_creation_input_tokens", 12)
	assertInt(t, root, "usage.cache_read_input_tokens", 34)
	assertInt(t, root, "usage.completion_tokens_details.reasoning_tokens", 5)
}

func TestBuildChatCompletionStreamUsageOnlyChunkPassesCacheUsage(t *testing.T) {
	state := NewStreamState("gpt-test")
	ConvertStreamChunk(context.Background(), []byte(`data: {"type":"response.created","response":{"id":"resp_1","created_at":1710000000,"model":"gpt-test"}}`), state, nil, true)
	ConvertStreamChunk(context.Background(), []byte(`data: {"type":"response.output_text.delta","delta":"ok"}`), state, nil, true)
	chunks := ConvertStreamChunk(context.Background(), []byte(`data: {"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120,"input_cached_tokens":34,"cache_creation_input_tokens":12,"output_tokens_details":{"reasoning_tokens":5}}}}`), state, nil, true)
	if len(chunks) != 1 {
		t.Fatalf("expected finish chunk, got %d", len(chunks))
	}

	usageChunk := BuildChatCompletionStreamUsageOnlyChunk(state)
	root := gjson.Parse(usageChunk)
	assertInt(t, root, "usage.prompt_tokens", 100)
	assertInt(t, root, "usage.completion_tokens", 20)
	assertInt(t, root, "usage.total_tokens", 120)
	assertInt(t, root, "usage.prompt_tokens_details.cached_tokens", 34)
	assertInt(t, root, "usage.prompt_tokens_details.cache_creation_tokens", 12)
	assertInt(t, root, "usage.prompt_tokens_details.cache_read_tokens", 34)
	assertInt(t, root, "usage.cache_creation_input_tokens", 12)
	assertInt(t, root, "usage.cache_read_input_tokens", 34)
	assertInt(t, root, "usage.completion_tokens_details.reasoning_tokens", 5)
}

func TestConvertStreamSSEToNonStreamResponseKeepsCacheUsage(t *testing.T) {
	data := []byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"created_at\":1710000000,\"model\":\"gpt-test\"}}\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":100,\"output_tokens\":20,\"total_tokens\":120,\"cache_creation_input_tokens\":12,\"cache_read_input_tokens\":34}}}\n")

	out, ok := ConvertStreamSSEToNonStreamResponse(data, "gpt-test", nil)
	if !ok {
		t.Fatal("expected converted response")
	}
	root := gjson.Parse(out)
	assertInt(t, root, "usage.prompt_tokens", 100)
	assertInt(t, root, "usage.completion_tokens", 20)
	assertInt(t, root, "usage.total_tokens", 120)
	assertInt(t, root, "usage.prompt_tokens_details.cached_tokens", 34)
	assertInt(t, root, "usage.prompt_tokens_details.cache_creation_tokens", 12)
	assertInt(t, root, "usage.prompt_tokens_details.cache_read_tokens", 34)
	assertInt(t, root, "usage.cache_creation_input_tokens", 12)
	assertInt(t, root, "usage.cache_read_input_tokens", 34)
}

func assertInt(t *testing.T, root gjson.Result, path string, want int64) {
	t.Helper()
	got := root.Get(path)
	if !got.Exists() {
		t.Fatalf("missing %s in %s", path, root.Raw)
	}
	if got.Int() != want {
		t.Fatalf("%s = %d, want %d", path, got.Int(), want)
	}
}
