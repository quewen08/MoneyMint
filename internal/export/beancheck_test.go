package export

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// beanCheckCandidates 返回 bean-check 的候选调用方式，按优先级排列。
//
// 优先使用项目内 .venv（Windows / POSIX 两种布局），保证本地与 CI 一致；
// 其次回退系统 python 与 PATH 上的 bean-check（CI 环境用 pip 安装 beancount）。
func beanCheckCandidates() [][]string {
	return [][]string{
		{filepath.Join("..", "..", ".venv", "Scripts", "python.exe"), "-m", "beancount.scripts.check"},
		{filepath.Join("..", "..", ".venv", "bin", "python"), "-m", "beancount.scripts.check"},
		{"python3", "-m", "beancount.scripts.check"},
		{"python", "-m", "beancount.scripts.check"},
		{"bean-check"},
	}
}

// strictBeanCheck 判断是否启用严格模式：严格模式下「找不到校验器」视为失败。
//
// CI 必须设 BEAN_CHECK_STRICT=1 —— 否则缺少 beancount 时校验会被静默跳过，
// 测试依然全绿，但账芯兼容性实际从未被验证（假绿灯）。
func strictBeanCheck() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("BEAN_CHECK_STRICT")))
	return v == "1" || v == "true"
}

// checkerUnavailable 判断输出是否表示「校验器本身不可用」而非「账本有错」。
//
// 典型场景：python 存在但没装 beancount → 报 ModuleNotFoundError。
// 这种情况应继续尝试下一个候选，而不是判定导出文件有问题。
// 匹配用小写前缀（不带结束引号），以同时覆盖
//
//	No module named 'beancount'
//	No module named 'beancount.scripts.check'
func checkerUnavailable(out string) bool {
	low := strings.ToLower(out)
	for _, s := range []string{
		"no module named 'beancount",
		`no module named "beancount`,
		"no module named beancount",
	} {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
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

// runBeanCheck 用外部 bean-check 校验导出的 .beancount 文件（退出码 0 即通过）。
//
// 严格模式：全部候选都不可用 → t.Fatalf（附安装指引）；
// 宽松模式：仅告警并跳过校验，保证本地缺 Python 环境时仍可验证其余断言。
// 注意：校验器可用但账本有错（退出码非 0 且非「模块缺失」）一律 t.Fatalf，
// 这在两种模式下都成立——那是真实的兼容性缺陷，不是环境问题。
func runBeanCheck(t *testing.T, beanFilePath string) {
	t.Helper()

	var tried []string
	for _, cand := range beanCheckCandidates() {
		// 带路径的候选（.venv）先确认文件存在，避免无谓的 exec 尝试与噪音错误。
		if strings.ContainsAny(cand[0], `/\`) {
			if _, err := os.Stat(cand[0]); err != nil {
				tried = append(tried, cand[0]+"（不存在）")
				continue
			}
		}

		args := append(append([]string{}, cand[1:]...), beanFilePath)
		out, err := exec.Command(cand[0], args...).CombinedOutput()
		if err == nil {
			t.Logf("bean-check 通过（%v）", cand)
			return
		}

		outStr := string(out)
		var execErr *exec.Error
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &execErr) || errors.Is(err, exec.ErrNotFound):
			// 命令本身不存在
			tried = append(tried, cand[0]+"（不可用）")
			continue
		case errors.As(err, &exitErr) && checkerUnavailable(outStr):
			// 命令存在但校验器模块未安装
			tried = append(tried, cand[0]+"（未安装 beancount）")
			continue
		case errors.As(err, &exitErr):
			// 校验器正常工作，但账本未通过校验：这是真实缺陷
			t.Fatalf("bean-check 校验未通过（%v）: %v\n输出:\n%s", cand, err, outStr)
		default:
			tried = append(tried, cand[0]+"（执行异常）")
			continue
		}
	}

	if strictBeanCheck() {
		t.Fatalf("严格模式：未找到可用的 bean-check，导出兼容性未经校验。\n"+
			"安装方式：python -m venv .venv && .venv/Scripts/python.exe -m pip install beancount\n"+
			"已尝试：%v", tried)
	}
	t.Logf("⚠️ bean-check 不可用，跳过导出兼容性校验（非严格模式）。已尝试：%v", tried)
}
