/// 修改密码页（P1-B3）：凭旧密码设置新密码。
/// 成功服务端会作废全部旧会话并签发新 token，本端自动续用新 token。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/common.dart';

class ChangePasswordScreen extends StatefulWidget {
  const ChangePasswordScreen({super.key});

  @override
  State<ChangePasswordScreen> createState() => _ChangePasswordScreenState();
}

class _ChangePasswordScreenState extends State<ChangePasswordScreen> {
  final _oldCtl = TextEditingController();
  final _newCtl = TextEditingController();
  final _confirmCtl = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _oldCtl.dispose();
    _newCtl.dispose();
    _confirmCtl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final oldP = _oldCtl.text;
    final newP = _newCtl.text;
    if (oldP.isEmpty || newP.isEmpty) {
      showAppSnack(context, '请填写完整', warn: true);
      return;
    }
    if (newP.length < 6) {
      showAppSnack(context, '新密码至少 6 位', warn: true);
      return;
    }
    if (newP != _confirmCtl.text) {
      showAppSnack(context, '两次输入的新密码不一致', warn: true);
      return;
    }
    setState(() => _busy = true);
    try {
      await AppScope.of(context).auth.changePassword(oldP, newP);
      if (!mounted) return;
      showAppSnack(context, '密码已修改');
      Navigator.pop(context);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '修改失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('修改密码')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          AppCard(
            child: Column(
              children: [
                TextField(
                  controller: _oldCtl,
                  obscureText: true,
                  decoration: const InputDecoration(labelText: '旧密码'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _newCtl,
                  obscureText: true,
                  decoration: const InputDecoration(
                      labelText: '新密码', hintText: '至少 6 位'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _confirmCtl,
                  obscureText: true,
                  decoration: const InputDecoration(labelText: '确认新密码'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: _busy ? null : _submit,
            child: _busy
                ? const SizedBox(
                    height: 18,
                    width: 18,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : const Text('确认修改'),
          ),
          const SizedBox(height: 8),
          const Text('修改成功后，本机其他设备上已登录的会话将失效，需重新登录。',
              textAlign: TextAlign.center, style: AppTheme.muted),
        ],
      ),
    );
  }
}
