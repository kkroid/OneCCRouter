package translate

import (
	"encoding/json"
	"testing"
)

// TestStreamChunk_SingleToolMultiChunkArgs reproduces the exact bug that caused
// "Content block not found": a single tool call whose arguments arrive across
// MULTIPLE chunks. The old code closed the content block on every tool chunk,
// so the 2nd args chunk's delta referenced a stale/closed block index.
func TestStreamChunk_SingleToolMultiChunkArgs(t *testing.T) {
	ctx := &StreamContext{
		MessageID: "msg_test",
		Model:     "claude-opus-4.8",
		ToolCalls: make(map[int]*ToolCallState),
	}

	chunks := []*OpenAIStreamChunk{
		// chunk 1: id + name + first args fragment (typical OpenAI first delta)
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 0, ID: "call_1", Function: &OpenAIToolFunctionDelta{Name: "Read", Arguments: `{"file_`}},
			},
		}}}},
		// chunk 2: more args
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 0, Function: &OpenAIToolFunctionDelta{Arguments: `path":"/tmp/`}},
			},
		}}}},
		// chunk 3: final args
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 0, Function: &OpenAIToolFunctionDelta{Arguments: `x.go"}`}},
			},
		}}}},
	}

	var allEvents []SSEEvent
	for _, c := range chunks {
		evs, err := TranslateStreamChunk(c, ctx)
		if err != nil {
			t.Fatal(err)
		}
		allEvents = append(allEvents, evs...)
	}
	// finish
	finish := "tool_calls"
	evs, _ := TranslateStreamChunk(&OpenAIStreamChunk{
		Choices: []OpenAIStreamChoice{{Index: 0, FinishReason: &finish}},
	}, ctx)
	allEvents = append(allEvents, evs...)

	// --- Assertions ---
	var blockStartIdx = -99
	var deltaIndices []int
	var blockStopIdx = -99
	startCount, stopCount := 0, 0
	var accumulatedArgs string

	for _, ev := range allEvents {
		switch ev.Type {
		case "content_block_start":
			startCount++
			if ev.Index != nil {
				blockStartIdx = *ev.Index
			}
		case "content_block_delta":
			if ev.Index != nil {
				deltaIndices = append(deltaIndices, *ev.Index)
			}
			if ev.Delta != nil && ev.Delta.Type == "input_json_delta" {
				accumulatedArgs += ev.Delta.PartialJSON
			}
		case "content_block_stop":
			stopCount++
			if ev.Index != nil {
				blockStopIdx = *ev.Index
			}
		}
	}

	// Exactly ONE block_start and ONE block_stop
	if startCount != 1 {
		t.Errorf("expected exactly 1 content_block_start, got %d", startCount)
	}
	if stopCount != 1 {
		t.Errorf("expected exactly 1 content_block_stop, got %d", stopCount)
	}

	// CRITICAL: every delta index MUST match the block_start index
	for _, di := range deltaIndices {
		if di != blockStartIdx {
			t.Errorf("delta index %d != block_start index %d (Content block not found bug!)", di, blockStartIdx)
		}
	}
	if blockStopIdx != blockStartIdx {
		t.Errorf("block_stop index %d != block_start index %d", blockStopIdx, blockStartIdx)
	}

	// All args fragments must accumulate into valid JSON
	if accumulatedArgs != `{"file_path":"/tmp/x.go"}` {
		t.Errorf("args not accumulated correctly: %q", accumulatedArgs)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(accumulatedArgs), &parsed); err != nil {
		t.Errorf("accumulated args not valid JSON: %v", err)
	}
	if parsed["file_path"] != "/tmp/x.go" {
		t.Errorf("file_path lost: got %v", parsed["file_path"])
	}
}

// TestStreamChunk_TwoToolsMultiChunk verifies two sequential tool calls, each
// with args spread across chunks, get distinct content block indices.
func TestStreamChunk_TwoToolsMultiChunk(t *testing.T) {
	ctx := &StreamContext{
		MessageID: "msg_test",
		Model:     "claude-opus-4.8",
		ToolCalls: make(map[int]*ToolCallState),
	}

	chunks := []*OpenAIStreamChunk{
		// tool 0: name + partial args
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 0, ID: "call_0", Function: &OpenAIToolFunctionDelta{Name: "Read", Arguments: `{"p":`}},
			},
		}}}},
		// tool 0: more args
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 0, Function: &OpenAIToolFunctionDelta{Arguments: `"a"}`}},
			},
		}}}},
		// tool 1: name + partial args (different index → new block)
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 1, ID: "call_1", Function: &OpenAIToolFunctionDelta{Name: "Grep", Arguments: `{"q":`}},
			},
		}}}},
		// tool 1: more args
		{Choices: []OpenAIStreamChoice{{Index: 0, Delta: OpenAIStreamDelta{
			ToolCalls: []OpenAIToolCallDelta{
				{Index: 1, Function: &OpenAIToolFunctionDelta{Arguments: `"b"}`}},
			},
		}}}},
	}

	var allEvents []SSEEvent
	for _, c := range chunks {
		evs, _ := TranslateStreamChunk(c, ctx)
		allEvents = append(allEvents, evs...)
	}
	finish := "tool_calls"
	evs, _ := TranslateStreamChunk(&OpenAIStreamChunk{
		Choices: []OpenAIStreamChoice{{Index: 0, FinishReason: &finish}},
	}, ctx)
	allEvents = append(allEvents, evs...)

	// Map block index → accumulated args
	startCount, stopCount := 0, 0
	argsByIndex := map[int]string{}
	startIndices := map[int]bool{}

	for _, ev := range allEvents {
		switch ev.Type {
		case "content_block_start":
			startCount++
			if ev.Index != nil {
				startIndices[*ev.Index] = true
			}
		case "content_block_delta":
			if ev.Index != nil && ev.Delta != nil && ev.Delta.Type == "input_json_delta" {
				argsByIndex[*ev.Index] += ev.Delta.PartialJSON
			}
		case "content_block_stop":
			stopCount++
		}
	}

	// Two distinct blocks, two stops
	if startCount != 2 {
		t.Errorf("expected 2 content_block_start, got %d", startCount)
	}
	if stopCount != 2 {
		t.Errorf("expected 2 content_block_stop, got %d", stopCount)
	}
	if len(startIndices) != 2 {
		t.Errorf("expected 2 distinct block indices, got %d: %v", len(startIndices), startIndices)
	}

	// Every delta index must correspond to a started block
	for idx := range argsByIndex {
		if !startIndices[idx] {
			t.Errorf("delta on index %d but no block_start for it", idx)
		}
	}
}
