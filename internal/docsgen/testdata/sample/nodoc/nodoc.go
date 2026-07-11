package nodoc

// Exported has no package doc comment above the package clause, so nodoc
// must be skipped by the discovery rule despite having an exported
// identifier.
func Exported() {}
