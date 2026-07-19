package utils

import (
	"fmt"
	"strings"
)

const (
	IPv6MulticastAddr = "ff02::1"
	IPv6LLPrefix      = "fe80"
	IPv6PrefixLength  = 64

	Red   = "\033[31m"
	Gray  = "\033[1;90m"
	Blue  = "\033[1;34m"
	Green = "\033[1;32m"
	Bold  = "\033[1m"
	White = "\033[0m"
)

var (
	Delim  = Red + strings.Repeat("=", 49) + White
	Banner = fmt.Sprintf("      %s%s____%s                 _ %s_   _%s      _   %s", Bold, Red, Gray, Bold+Red, Gray, White)
)
