/// 登录 / 注册页（F-9 改版）：渐变头部 + 白卡上浮表单。
/// 注册前先查询服务端注册开关（B-4）：关闭时注册入口禁用并说明原因。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../viewmodels/auth_controller.dart';
import '../../widgets/common.dart';

class LoginScreen extends StatelessWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: ListenableBuilder(
                listenable: auth,
                builder: (ctx, _) => _Form(auth: auth),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _Form extends StatefulWidget {
  final AuthController auth;
  const _Form({required this.auth});

  @override
  State<_Form> createState() => _FormState();
}

class _FormState extends State<_Form> {
  final _userCtl = TextEditingController();
  final _passCtl = TextEditingController();
  bool _isRegister = false;
  bool? _regAllowed; // null = 未查询；false = 注册已关闭
  bool _checked = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // 注意：AppScope.of 依赖 InheritedWidget， initState 阶段调用会抛异常，
    // 必须放到 didChangeDependencies（仅执行一次）。
    if (_checked) return;
    _checked = true;
    _checkRegistration();
  }

  @override
  void dispose() {
    _userCtl.dispose();
    _passCtl.dispose();
    super.dispose();
  }

  /// 查询注册开关（公开端点，无需登录）。失败按开放处理（api 内已兜底）。
  Future<void> _checkRegistration() async {
    final v = await AppScope.of(context).auth.api.registrationEnabled();
    if (!mounted) return;
    setState(() => _regAllowed = v);
    if (!v && _isRegister) setState(() => _isRegister = false);
  }

  void _switchMode() {
    if (!_isRegister && _regAllowed == false) {
      showAppSnack(context, '注册已关闭，请联系管理员在设置中开放，或由 owner 邀请加入', warn: true);
      return;
    }
    setState(() => _isRegister = !_isRegister);
  }

  Future<void> _submit() async {
    final user = _userCtl.text.trim();
    final pass = _passCtl.text;
    if (user.isEmpty || pass.isEmpty) {
      showAppSnack(context, '请输入用户名和密码', warn: true);
      return;
    }
    if (_isRegister && pass.length < 6) {
      showAppSnack(context, '密码至少 6 位', warn: true);
      return;
    }
    if (_isRegister) {
      await widget.auth.register(user, pass);
    } else {
      await widget.auth.login(user, pass);
    }
    if (!mounted) return;
    if (widget.auth.error != null) {
      showAppSnack(context, widget.auth.error!, warn: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final busy = widget.auth.busy;
    final regClosed = _isRegister && _regAllowed == false;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // 渐变头部：logo + 标题（圆角下摆，与账单/资产页同一视觉语言）。
        Container(
          padding: const EdgeInsets.fromLTRB(24, 36, 24, 46),
          decoration: const BoxDecoration(
            gradient: AppColors.heroGradient,
            borderRadius: BorderRadius.vertical(bottom: Radius.circular(30)),
          ),
          child: Column(
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: .16),
                  borderRadius: BorderRadius.circular(20),
                ),
                alignment: Alignment.center,
                child: const Icon(Icons.account_balance_wallet,
                    size: 34, color: Colors.white),
              ),
              const SizedBox(height: 14),
              const Text('家庭记账',
                  style: TextStyle(
                      fontSize: 24,
                      fontWeight: FontWeight.w800,
                      color: Colors.white)),
              const SizedBox(height: 6),
              Text(
                _isRegister ? '注册并加入共享账本' : '登录你的共享账本',
                style: TextStyle(
                    fontSize: 13, color: Colors.white.withValues(alpha: .78)),
              ),
            ],
          ),
        ),
        // 白卡上浮叠在渐变头下摆上。
        Transform.translate(
          offset: const Offset(0, -28),
          child: Container(
            margin: const EdgeInsets.symmetric(horizontal: 4),
            padding: const EdgeInsets.all(22),
            decoration: BoxDecoration(
              color: AppColors.card,
              borderRadius: BorderRadius.circular(18),
              boxShadow: const [
                BoxShadow(
                    color: Color(0x14172044),
                    blurRadius: 16,
                    offset: Offset(0, 4)),
              ],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _userCtl,
                  decoration: const InputDecoration(
                      labelText: '用户名', hintText: '如 alice'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _passCtl,
                  obscureText: true,
                  decoration: const InputDecoration(
                      labelText: '密码', hintText: '••••••••'),
                ),
                const SizedBox(height: 20),
                ElevatedButton(
                  onPressed: busy ? null : _submit,
                  child: busy
                      ? const SizedBox(
                          height: 18,
                          width: 18,
                          child: CircularProgressIndicator(
                              strokeWidth: 2, color: Colors.white),
                        )
                      : Text(_isRegister ? '注册并登录' : '登录'),
                ),
                TextButton(
                  onPressed: busy ? null : _switchMode,
                  child: Text(_isRegister ? '已有账号？去登录' : '没有账号？注册'),
                ),
                // 注册语义提示 + 开关状态（B-4）：首个账号自动成为系统管理员。
                if (_isRegister) ...[
                  const SizedBox(height: 4),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: regClosed
                          ? AppColors.warn.withValues(alpha: .08)
                          : AppColors.blue.withValues(alpha: .07),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(
                      regClosed
                          ? '注册已关闭：请由管理员在「更多 → 开放注册」中开启，'
                              '或由账本 owner 通过成员管理邀请你加入。'
                          : '首个注册的账号自动成为系统管理员；'
                              '之后的账号需管理员开放注册或由 owner 邀请加入账本。',
                      style: TextStyle(
                        fontSize: 12,
                        height: 1.5,
                        color: regClosed ? AppColors.warn : AppColors.sub,
                      ),
                    ),
                  ),
                ],
                if (!_isRegister) ...[
                  const SizedBox(height: 10),
                  Text(
                    '忘记密码？请联系账本所有者重置；\n'
                    '服务器管理员可在 NAS 上执行命令行找回（见 README）。',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                        fontSize: 11,
                        height: 1.5,
                        color: AppColors.sub.withValues(alpha: .9)),
                  ),
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }
}
