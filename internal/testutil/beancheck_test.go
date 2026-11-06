package testutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLaunchFailureIsNotLedgerError 锁定本次 CI 事故的根因：
// Linux 上 exec 一个不存在的绝对路径时 stdout/stderr 全为空，
// 旧实现靠「输出里有没有 no module named / not found」判断环境是否可用，
// 于是把「fork/exec ...: no such file or directory」误判成账本校验失败并直接 Fatal。
func TestLaunchFailureIsNotLedgerError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-python")
	_, err := exec.Command(missing, "-m", "beancount.scripts.check").CombinedOutput()
	if err == nil {
		t.Fatal("预期命令不存在，实际执行成功")
	}
	if !IsLaunchFailure(err) {
		t.Fatalf("命令无法启动应识别为环境问题，实际未识别：%T %v", err, err)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("启动失败不应被当成 ExitError（那会触发账本有错的 Fatal）: %v", err)
	}
	if CheckerUnavailable("") {
		t.Fatal("空输出不应被判定为 beancount 模块缺失")
	}
}

// TestRunBeanCheckSkipsMissingCandidate 验证：首个候选不存在时必须继续尝试后续候选，
// 而不是把测试打成失败。
//
// 开启严格模式，只有第二个候选真的执行成功才不会 Fatal —— 这就同时证明了「跳过」与「继续」。
// 占位命令会忽略自动追加的账本路径参数，只验证控制流，不依赖本机是否安装 beancount。
func TestRunBeanCheckSkipsMissingCandidate(t *testing.T) {
	t.Setenv("BEAN_CHECK_STRICT", "1")
	okCmd := []string{"sh", "-c", "true"}
	if runtime.GOOS == "windows" {
		okCmd = []string{"cmd", "/c", "echo ok"}
	}
	cands := [][]string{
		{filepath.Join(t.TempDir(), "no-such-python"), "-m", "beancount.scripts.check"},
		okCmd,
	}
	runBeanCheckWithCandidates(t, "dummy.bean", "", cands)
}

// TestRepoRootFindsGoMod 验证项目根定位与包深度无关，且 .venv 候选为绝对路径。
// 绝对路径是必须的：调用方会设置 cmd.Dir 指向导出根目录（解析 include），
// 相对路径在切换工作目录后会失效。
func TestRepoRootFindsGoMod(t *testing.T) {
	root := RepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("RepoRoot 返回的目录不含 go.mod: %s (%v)", root, err)
	}
	cands := BeanCheckCandidates(root)
	if len(cands) < 2 {
		t.Fatalf("候选列表至少应包含两种 venv 布局，实际 %d 条", len(cands))
	}
	for i := 0; i < 2; i++ {
		py := cands[i][0]
		if !filepath.IsAbs(py) {
			t.Fatalf("venv 候选必须是绝对路径，实际 %q", py)
		}
		if want := filepath.Join(root, ".venv"); !containsPath(want, py) {
			t.Fatalf("venv 候选 %q 不在 %q 下", py, want)
		}
	}
}

func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
