package mcpx

// RiskAnnotations derives MCP ToolAnnotations from a tool's risk
// classification. readOnly maps directly to ReadOnlyHint. destructive is set
// explicitly on DestructiveHint via BoolPtr (rather than left nil) because
// go-sdk defaults a nil DestructiveHint to true, which would silently
// mislabel a non-destructive write tool; setting it explicitly keeps the
// value unambiguous regardless of go-sdk's default. RiskAnnotations is pure
// and always returns a freshly-allocated *ToolAnnotations.
func RiskAnnotations(readOnly, destructive bool) *ToolAnnotations {
	return &ToolAnnotations{
		ReadOnlyHint:    readOnly,
		DestructiveHint: BoolPtr(destructive),
	}
}
