package domain

import "testing"

func TestNormalizePayMethodName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "bank", raw: "bank", want: "银行卡"},
		{name: "bank transfer", raw: "Bank Transfer", want: "银行卡"},
		{name: "bank debit card", raw: "银行借记卡", want: "银行卡"},
		{name: "wechat", raw: "wechat", want: "微信"},
		{name: "alipay", raw: "Alipay", want: "支付宝"},
		{name: "qq", raw: "qqwall", want: "QQ 钱包"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePayMethodName(tt.raw); got != tt.want {
				t.Fatalf("NormalizePayMethodName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizePayMethodsString(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "single bank", raw: "bank", want: "银行卡"},
		{name: "exact qq wallet", raw: "QQ 钱包", want: "QQ 钱包"},
		{name: "space separated old slice format", raw: "支付宝 微信", want: "支付宝, 微信"},
		{name: "legacy bracketed multi word aliases", raw: "[Bank Transfer Alipay]", want: "银行卡, 支付宝"},
		{name: "legacy bracketed qq wallet", raw: "[QQ 钱包]", want: "QQ 钱包"},
		{name: "comma separated mixed names", raw: "bank, 微信, 支付宝", want: "银行卡, 微信, 支付宝"},
		{name: "dedupe", raw: "bank, 银行借记卡", want: "银行卡"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePayMethodsString(tt.raw); got != tt.want {
				t.Fatalf("NormalizePayMethodsString(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
