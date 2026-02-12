package i18n

import "embed"

//go:embed messages/*.json
var messagesFS embed.FS
