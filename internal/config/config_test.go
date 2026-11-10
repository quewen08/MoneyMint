package config

import (
	"reflect"
	"testing"
)

// validBaseConfig 返回一份可通过 Validate 的最小配置，供 CORS 用例复用。
func validBaseConfig() Config {
	return Config{
		DBPath:              "data/ledger.db",
		ListenAddr:          ":8080",
		BackupHour:          3,
		BackupKeepDaily:     14,
		BackupKeepMonthly:   12,
		LoginMaxAttempts:    5,
		LoginWindowMinutes:  15,
		LoginLockoutMinutes: 10,
		AuthCookie:          true,
		AuthExposeToken:     true,
	}
}

// TestCORSOriginsParsing 覆盖白名单解析与合法性校验。
func TestCORSOriginsParsing(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "未配置仅同源", raw: "", want: nil},
		{name: "通配", raw: "*", want: []string{"*"}},
		{name: "单个来源", raw: "http://app.local", want: []string{"http://app.local"}},
		{
			name: "多个来源去空白去重并去掉末尾斜杠",
			raw:  " http://app.local , https://a.example:8443/ ,http://app.local ",
			want: []string{"http://app.local", "https://a.example:8443"},
		},
		{name: "通配不能混用", raw: "*,http://app.local", wantErr: true},
		{name: "缺少协议", raw: "app.local", wantErr: true},
		{name: "非 http 协议", raw: "ftp://app.local", wantErr: true},
		{name: "不允许路径", raw: "http://app.local/path", wantErr: true},
		{name: "不允许查询", raw: "http://app.local?x=1", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaseConfig()
			cfg.CORSOrigins = tc.raw
			got := cfg.CORSWhitelist()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("CORSWhitelist(%q) = %v，期望 %v", tc.raw, got, tc.want)
			}
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate 应拒绝 %q", tc.raw)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate 不应拒绝 %q: %v", tc.raw, err)
			}
		})
	}
}

// TestAuthEnvDefaults 校验 D-7 四个新环境变量的默认值。
func TestAuthEnvDefaults(t *testing.T) {
	t.Setenv("LEDGER_AUTH_COOKIE", "")
	t.Setenv("LEDGER_COOKIE_SECURE", "")
	t.Setenv("LEDGER_CORS_ORIGINS", "")
	t.Setenv("LEDGER_AUTH_EXPOSE_TOKEN", "")

	cfg := Load()
	if !cfg.AuthCookie {
		t.Fatalf("LEDGER_AUTH_COOKIE 默认应为 true")
	}
	if cfg.CookieSecure {
		t.Fatalf("LEDGER_COOKIE_SECURE 默认应为 false")
	}
	if cfg.CORSOrigins != "" {
		t.Fatalf("LEDGER_CORS_ORIGINS 默认应为空（仅同源）")
	}
	if !cfg.AuthExposeToken {
		t.Fatalf("LEDGER_AUTH_EXPOSE_TOKEN 默认应为 true（兼容旧版）")
	}
}
