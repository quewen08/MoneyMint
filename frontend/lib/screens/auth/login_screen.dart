/// 登录 / 注册页：未登录用户首屏入口（设计稿「登录/注册」）。
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

  @override
  void dispose() {
    _userCtl.dispose();
    _passCtl.dispose();
    super.dispose();
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
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Icon(Icons.account_balance_wallet, size: 64, color: AppColors.blue),
        const SizedBox(height: 12),
        const Text('家庭记账',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800)),
        const SizedBox(height: 4),
        Text(_isRegister ? '首个用户自动创建家庭账本，后续用户需等 owner 邀请' : '登录你的共享账本',
            textAlign: TextAlign.center, style: AppTheme.muted),
        const SizedBox(height: 24),
        TextField(
          controller: _userCtl,
          decoration: const InputDecoration(labelText: '用户名', hintText: '如 alice'),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _passCtl,
          obscureText: true,
          decoration: const InputDecoration(labelText: '密码', hintText: '••••••••'),
        ),
        const SizedBox(height: 20),
        ElevatedButton(
          onPressed: busy ? null : _submit,
          child: busy
              ? const SizedBox(
                  height: 18,
                  width: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : Text(_isRegister ? '注册并登录' : '登录'),
        ),
        TextButton(
          onPressed: busy ? null : () => setState(() => _isRegister = !_isRegister),
          child: Text(_isRegister ? '已有账号？去登录' : '没有账号？注册'),
        ),
      ],
    );
  }
}
