package hooks

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errWriter always fails, to exercise write's io error paths (Marshal
// never fails on these value types, so the only reachable error is the
// underlying io.Writer's).
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// TestResponseWrite_Zero proves the zero-value Response writes nothing —
// silent allow.
func TestResponseWrite_Zero(t *testing.T) {
	var buf bytes.Buffer

	err := Response{}.write(&buf, eventStop)

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// TestResponseWrite_Block proves the Block field's golden JSON shape.
func TestResponseWrite_Block(t *testing.T) {
	var buf bytes.Buffer

	err := Response{Block: "not now"}.write(&buf, eventPreToolUse)

	require.NoError(t, err)
	assert.Equal(t, `{"decision":"block","reason":"not now"}`+"\n", buf.String())
}

// TestResponseWrite_BlockWithSystemMessage proves SystemMessage merges as
// a sibling key on the Block shape.
func TestResponseWrite_BlockWithSystemMessage(t *testing.T) {
	var buf bytes.Buffer

	err := Response{Block: "not now", SystemMessage: "careful"}.write(&buf, eventPreToolUse)

	require.NoError(t, err)
	assert.Equal(t, `{"decision":"block","reason":"not now","systemMessage":"careful"}`+"\n", buf.String())
}

// TestResponseWrite_AdditionalContext proves the AdditionalContext golden
// JSON shape, including that hookEventName reflects the invoking event.
func TestResponseWrite_AdditionalContext(t *testing.T) {
	var buf bytes.Buffer

	err := Response{AdditionalContext: "relevant kb notes"}.write(&buf, eventUserPromptSubmit)

	require.NoError(t, err)
	assert.Equal(t,
		`{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"relevant kb notes"}}`+"\n",
		buf.String())
}

// TestResponseWrite_AdditionalContextWithSystemMessage proves
// SystemMessage merges as a sibling key on the AdditionalContext shape.
func TestResponseWrite_AdditionalContextWithSystemMessage(t *testing.T) {
	var buf bytes.Buffer

	err := Response{AdditionalContext: "ctx", SystemMessage: "note"}.write(&buf, eventSessionStart)

	require.NoError(t, err)
	assert.Equal(t,
		`{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"ctx"},"systemMessage":"note"}`+"\n",
		buf.String())
}

// TestResponseWrite_PlainText proves PlainText is written bare — no JSON
// wrapper at all.
func TestResponseWrite_PlainText(t *testing.T) {
	var buf bytes.Buffer

	err := Response{PlainText: "just some context"}.write(&buf, eventUserPromptSubmit)

	require.NoError(t, err)
	assert.Equal(t, "just some context\n", buf.String())
}

// TestResponseWrite_PlainTextIgnoredWhenBlockSet proves precedence:
// Block wins over PlainText even if both are set.
func TestResponseWrite_PlainTextIgnoredWhenBlockSet(t *testing.T) {
	var buf bytes.Buffer

	err := Response{Block: "nope", PlainText: "should not appear"}.write(&buf, eventStop)

	require.NoError(t, err)
	assert.Equal(t, `{"decision":"block","reason":"nope"}`+"\n", buf.String())
}

// TestResponseWrite_PlainTextIgnoredWhenAdditionalContextSet proves
// precedence: AdditionalContext wins over PlainText.
func TestResponseWrite_PlainTextIgnoredWhenAdditionalContextSet(t *testing.T) {
	var buf bytes.Buffer

	err := Response{AdditionalContext: "ctx", PlainText: "should not appear"}.write(&buf, eventSessionStart)

	require.NoError(t, err)
	assert.Equal(t,
		`{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"ctx"}}`+"\n",
		buf.String())
}

// TestResponseWrite_PlainTextWithSystemMessagePromotesToJSON proves the
// documented PlainText+SystemMessage assumption: bare text cannot carry a
// JSON systemMessage sibling, so SystemMessage wins as JSON and PlainText
// is dropped.
func TestResponseWrite_PlainTextWithSystemMessagePromotesToJSON(t *testing.T) {
	var buf bytes.Buffer

	err := Response{PlainText: "dropped", SystemMessage: "shown instead"}.write(&buf, eventUserPromptSubmit)

	require.NoError(t, err)
	assert.Equal(t, `{"systemMessage":"shown instead"}`+"\n", buf.String())
}

// TestResponseWrite_SystemMessageAlone proves SystemMessage with no other
// field set still produces JSON output (not silent allow).
func TestResponseWrite_SystemMessageAlone(t *testing.T) {
	var buf bytes.Buffer

	err := Response{SystemMessage: "heads up"}.write(&buf, eventPreCompact)

	require.NoError(t, err)
	assert.Equal(t, `{"systemMessage":"heads up"}`+"\n", buf.String())
}

// TestResponseWrite_JSONWriteErrorPropagates proves a write's underlying
// io error (not just a marshal error) propagates out of write.
func TestResponseWrite_JSONWriteErrorPropagates(t *testing.T) {
	err := Response{Block: "nope"}.write(errWriter{}, eventStop)
	assert.Error(t, err)
}

// TestResponseWrite_PlainTextWriteErrorPropagates proves the bare-text
// branch also propagates the underlying io error.
func TestResponseWrite_PlainTextWriteErrorPropagates(t *testing.T) {
	err := Response{PlainText: "hi"}.write(errWriter{}, eventUserPromptSubmit)
	assert.Error(t, err)
}

// TestResponseWrite_AllEightEvents proves the AdditionalContext shape's
// hookEventName field is correctly threaded for every documented event
// name, not just one or two spot-checked above.
func TestResponseWrite_AllEightEvents(t *testing.T) {
	events := []string{
		eventStop, eventSubagentStop, eventSubagentStart, eventUserPromptSubmit,
		eventPreToolUse, eventPostToolUse, eventPreCompact, eventSessionStart,
	}

	for _, ev := range events {
		t.Run(ev, func(t *testing.T) {
			var buf bytes.Buffer

			err := Response{AdditionalContext: "x"}.write(&buf, ev)

			require.NoError(t, err)
			assert.Contains(t, buf.String(), `"hookEventName":"`+ev+`"`)
		})
	}
}
