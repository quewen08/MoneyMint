package export

import (
	"testing"

	"familyledger/internal/testutil"
)

// bean-check 的候选探测、严格模式与错误分类统一放在 internal/testutil，
// 本包只保留参数适配。此前 export 与 server 各有一份实现，server 那份是简化复制版，
// 在 Linux CI 上误把「可执行文件不存在」当成账本校验失败（见 internal/testutil 回归测试）。

// runBeanCheck 校验导出的 .beancount：路径相对当前测试工作目录，无需切换子进程工作目录。
func runBeanCheck(t *testing.T, beanFilePath string) {
	t.Helper()
	testutil.RunBeanCheck(t, beanFilePath, "")
}

// 以下两个薄委托保留原有护栏测试的行为锁定点，实现以 testutil 为准。
func strictBeanCheck() bool {
	return testutil.StrictBeanCheck()
}

func checkerUnavailable(out string) bool {
	return testutil.CheckerUnavailable(out)
}

// TestStrictBeanCheckEnv 锁定严格模式的环境变量解析规则。
// CI 依赖 BEAN_CHECK_STRICT=1 生效，任何解析回归都会导致护栏静默失效。
func TestStrictBeanCheckEnv(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{" True ", true},
		{"0", false},
		{"yes", false},
		{"false", false},
	}
	for _, c := range cases {
		t.Setenv("BEAN_CHECK_STRICT", c.val)
		if got := strictBeanCheck(); got != c.want {
			t.Fatalf("BEAN_CHECK_STRICT=%q 解析为 %v，期望 %v", c.val, got, c.want)
		}
	}
}

// TestCheckerUnavailable 锁定「校验器不可用」与「账本有错」的区分规则。
// 判错方向会带来两种事故：把环境问题当缺陷（误报）或把缺陷当环境问题（漏报）。
func TestCheckerUnavailable(t *testing.T) {
	cases := []struct {
		out  string
		want bool
	}{
		{"/usr/bin/python: No module named beancount", true},
		{"/usr/bin/python3: No module named 'beancount'", true},
		{`python: No module named "beancount.scripts.check"`, true},
		{"No module named 'beancount.scripts.check'", true},
		// 真实校验失败不应被误判为环境问题
		{"error: 账本不平衡\n  Assets:Cash 10.00 CNY", false},
		{"Syntax error at line 3", false},
		{"", false},
	}
	for _, c := range cases {
		if got := checkerUnavailable(c.out); got != c.want {
			t.Fatalf("checkerUnavailable(%q) = %v，期望 %v", c.out, got, c.want)
		}
	}
}
