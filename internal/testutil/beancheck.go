// Package testutil 提供跨测试包共享的辅助能力，仅被 *_test.go 引用，不参与生产代码路径。
//
// 背景：internal/export 与 internal/server 各有一份 bean-check 调用逻辑，
// 后者是简化复制版，在 Linux CI 上把「可执行文件不存在」误判为「账本有错」直接 Fatal。
// 现在统一收敛到本包，避免再次漂移。
package testutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// RepoRoot 从当前测试工作目录逐级向上查找 go.mod，返回项目根目录。
//
// 不同测试包的工作目录深度可能不同（internal/export、internal/server、internal/testutil），
// 硬编码 "../../.venv" 只对其中的一个深度成立；按 go.mod 定位与包深度无关。
func RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("未找到项目根目录（go.mod），起始目录：%s", dir)
		}
		dir = parent
	}
}

// BeanCheckCandidates 返回 bean-check 的候选调用方式，按优先级排列。
//
// 优先项目内 .venv（Windows / POSIX 两种布局），其次系统 python 与 PATH 上的 bean-check。
// .venv 路径一律用绝对路径：调用方可能设置 cmd.Dir（导出根目录用于解析 include），
// 相对路径会在切换工作目录后失效。
func BeanCheckCandidates(repoRoot string) [][]string {
	return [][]string{
		{filepath.Join(repoRoot, ".venv", "Scripts", "python.exe"), "-m", "beancount.scripts.check"},
		{filepath.Join(repoRoot, ".venv", "bin", "python"), "-m", "beancount.scripts.check"},
		{"python3", "-m", "beancount.scripts.check"},
		{"python", "-m", "beancount.scripts.check"},
		{"bean-check"},
	}
}

// StrictBeanCheck 判断是否启用严格模式：严格模式下「找不到校验器」视为失败。
//
// CI 必须设 BEAN_CHECK_STRICT=1 —— 否则缺少 beancount 时校验会被静默跳过，
// 测试依然全绿，但账芯兼容性实际从未被验证（假绿灯）。
func StrictBeanCheck() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("BEAN_CHECK_STRICT")))
	return v == "1" || v == "true"
}

// CheckerUnavailable 判断输出是否表示「校验器模块未安装」而非「账本有错」。
//
// 典型场景：python 存在但没装 beancount → 报 ModuleNotFoundError。
// 这种情况应继续尝试下一个候选，而不是判定导出文件有问题。
func CheckerUnavailable(out string) bool {
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

// IsLaunchFailure 判断错误是否属于「命令根本没能启动」。
//
// 关键：Linux 上 exec 一个不存在的绝对路径时，CombinedOutput 的 stdout/stderr 全为空，
// 不能靠输出文本判断环境是否可用，必须看错误类型。
func IsLaunchFailure(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	// 绝对路径不存在时 os.StartProcess 返回 *os.PathError（错误文本 "fork/exec ...:
	// no such file or directory"），不是 *exec.Error，必须一并覆盖。
	return errors.As(err, &execErr) ||
		errors.Is(err, exec.ErrNotFound) ||
		errors.Is(err, os.ErrNotExist)
}

// RunBeanCheck 用外部 bean-check 校验导出的 .beancount 文件（退出码 0 即通过）。
//
//   - beanArg：传给校验器的账本路径，相对 workDir 或用绝对路径。
//   - workDir：子进程工作目录，空则继承当前目录。多文件导出需指向导出根以解析 include。
//
// 严格模式：全部候选都不可用 → t.Fatalf（附安装指引）；宽松模式：仅告警并跳过。
// 校验器可用但账本有错（退出码非 0 且非「模块缺失」）一律 t.Fatalf，
// 这在两种模式下都成立——那是真实的兼容性缺陷，不是环境问题。
func RunBeanCheck(t *testing.T, beanArg, workDir string) {
	t.Helper()
	runBeanCheckWithCandidates(t, beanArg, workDir, BeanCheckCandidates(RepoRoot(t)))
}

// runBeanCheckWithCandidates 允许注入候选列表，便于回归测试锁定「首个候选不可用」路径。
func runBeanCheckWithCandidates(t *testing.T, beanArg, workDir string, cands [][]string) {
	t.Helper()

	var tried []string
	for _, cand := range cands {
		// 带路径的候选先确认存在，跳过本机平台不匹配的 venv 布局（如 Linux 上的 Scripts/python.exe）。
		if strings.ContainsAny(cand[0], `/\`) {
			if _, err := os.Stat(cand[0]); err != nil {
				tried = append(tried, cand[0]+"（不存在）")
				continue
			}
		}

		args := append(append([]string{}, cand[1:]...), beanArg)
		cmd := exec.Command(cand[0], args...)
		if workDir != "" {
			cmd.Dir = workDir
		}
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Logf("bean-check 通过（%v）", cand)
			return
		}

		outStr := string(out)
		var exitErr *exec.ExitError
		switch {
		case IsLaunchFailure(err):
			// 命令本身不存在 / 无法启动，继续试下一个候选。
			tried = append(tried, cand[0]+"（不可用）")
			continue
		case errors.As(err, &exitErr) && CheckerUnavailable(outStr):
			// 命令存在但校验器模块未安装。
			tried = append(tried, cand[0]+"（未安装 beancount）")
			continue
		case errors.As(err, &exitErr):
			// 校验器正常运行且判定账本有错：这是真实缺陷，任何模式下都必须失败。
			t.Fatalf("bean-check 校验未通过（%v）: %v\n输出:\n%s", cand, err, outStr)
		default:
			tried = append(tried, cand[0]+"（执行异常）")
			continue
		}
	}

	if StrictBeanCheck() {
		t.Fatalf("严格模式：未找到可用的 bean-check，导出兼容性未经校验。\n"+
			"安装方式：python -m venv .venv && .venv/Scripts/python.exe -m pip install beancount\n"+
			"已尝试：%v", tried)
	}
	t.Logf("⚠️ bean-check 不可用，跳过导出兼容性校验（非严格模式）。已尝试：%v", tried)
}
