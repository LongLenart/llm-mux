package embedded

import _ "embed"

//go:embed config.example.yaml
var DefaultConfigTemplate []byte
