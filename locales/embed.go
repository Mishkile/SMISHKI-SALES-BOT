// Package locales embeds the per-language translation files so the compiled
// binary is self-contained. Set LOCALES_DIR to read them from disk instead
// (for example to add a language without rebuilding).
package locales

import "embed"

// FS holds <lang>/common.json and <lang>/faq.json for every shipped locale.
//
//go:embed */common.json */faq.json
var FS embed.FS
