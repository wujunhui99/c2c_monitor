package domain

import (
	"slices"
	"strings"
)

var payMethodAliases = map[string]string{
	"bank":            "银行卡",
	"bank transfer":   "银行卡",
	"bank card":       "银行卡",
	"debit card":      "银行卡",
	"bank debit card": "银行卡",
	"银行卡":             "银行卡",
	"银行借记卡":           "银行卡",
	"银行转账":            "银行卡",
	"wechat":          "微信",
	"wechat pay":      "微信",
	"微信":              "微信",
	"alipay":          "支付宝",
	"支付宝":             "支付宝",
	"qq wallet":       "QQ 钱包",
	"qq 钱包":           "QQ 钱包",
	"qqwall":          "QQ 钱包",
	"qq钱包":            "QQ 钱包",
}

var payMethodAliasOrder = sortedPayMethodAliases()

func NormalizePayMethodName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if normalized, ok := payMethodAliases[strings.ToLower(trimmed)]; ok {
		return normalized
	}

	return trimmed
}

func NormalizePayMethods(methods []string) []string {
	seen := make(map[string]struct{}, len(methods))
	result := make([]string, 0, len(methods))

	for _, method := range methods {
		normalized := NormalizePayMethodName(method)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func JoinNormalizedPayMethods(methods []string) string {
	return strings.Join(NormalizePayMethods(methods), ", ")
}

func NormalizePayMethodsString(raw string) string {
	trimmed := strings.TrimSpace(strings.Trim(raw, "[]"))
	if trimmed == "" {
		return ""
	}

	if normalized, ok := payMethodAliases[strings.ToLower(trimmed)]; ok {
		return normalized
	}

	var parts []string
	switch {
	case strings.Contains(trimmed, ","):
		parts = strings.Split(trimmed, ",")
	case strings.Contains(trimmed, "/"):
		parts = strings.Split(trimmed, "/")
	case strings.Contains(trimmed, "|"):
		parts = strings.Split(trimmed, "|")
	case strings.Contains(trimmed, " "):
		parts = splitLegacyPayMethods(trimmed)
	default:
		return NormalizePayMethodName(trimmed)
	}

	normalized := JoinNormalizedPayMethods(parts)
	if normalized == "" {
		return trimmed
	}
	return normalized
}

func splitLegacyPayMethods(raw string) []string {
	lower := strings.ToLower(raw)
	parts := make([]string, 0, 4)

	for len(lower) > 0 {
		lower = strings.TrimLeft(lower, " \t")
		raw = strings.TrimLeft(raw, " \t")
		if lower == "" {
			break
		}

		matched := false
		for _, alias := range payMethodAliasOrder {
			if strings.HasPrefix(lower, alias) {
				parts = append(parts, raw[:len(alias)])
				raw = raw[len(alias):]
				lower = lower[len(alias):]
				matched = true
				break
			}
		}

		if matched {
			continue
		}

		nextSpace := strings.IndexAny(lower, " \t")
		if nextSpace == -1 {
			parts = append(parts, raw)
			break
		}

		parts = append(parts, raw[:nextSpace])
		raw = raw[nextSpace:]
		lower = lower[nextSpace:]
	}

	return parts
}

func sortedPayMethodAliases() []string {
	aliases := make([]string, 0, len(payMethodAliases))
	for alias := range payMethodAliases {
		aliases = append(aliases, alias)
	}

	slices.SortFunc(aliases, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(a, b)
		}
		if len(a) > len(b) {
			return -1
		}
		return 1
	})

	return aliases
}
