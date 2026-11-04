/// 修改密码页（P1-B3）：凭旧密码设置新密码。
/// 成功服务端会作废全部旧会话并签发新 token，本端自动续用新 token。
///
/// [forced]=true 时（M3 强制改密）作为根页面由 AuthGate 展示：
/// 不给返回键（没有「稍后再说」），改密成功后 AuthGate 自动切回主框架。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/common.dart';

class ChangePasswordScreen extends StatefulWidget {
  /// 是否由登录门禁强制展示（服务端要求先改密）。
  final bool forced;
  const ChangePasswordScreen({super.key, this.forced = false});

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
      // 强制模式下本页是根页面，AuthGate 会因标记清除自动切回主框架，
      // 这里不能 pop（栈里没有上一页）。
      if (!widget.forced) Navigator.pop(context);
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
      appBar: AppBar(
        title: Text(widget.forced ? '设置新密码' : '修改密码'),
        // 强制模式不给返回键：这里没有「稍后再说」，不改完就用不了账本。
        automaticallyImplyLeading: !widget.forced,
        actions: [
          if (widget.forced)
            TextButton(
              onPressed: _busy ? null : () => AppScope.of(context).auth.logout(),
              child: const Text('退出登录'),
            ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (widget.forced) ...[
            AppCard(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(Icons.lock_reset, size: 18, color: AppColors.warn),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      '当前密码是他人代为设置的（预设账号或管理员重置），'
                      '请先设置一个只有你自己知道的新密码，设置完成后才能开始记账。',
                      style: TextStyle(fontSize: 12, height: 1.6, color: AppColors.sub),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
          ],
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
                const SizedBox(height: 12),
                Text(
                  '保存成功后本机自动换取新登录态，其他已登录设备将全部退出，需重新登录。',
                  style: TextStyle(fontSize: 12, height: 1.5, color: AppColors.sub),
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
          // 强制模式下卡片里已有同样说明，避免同一句话出现两遍。
          if (!widget.forced) ...[
            const SizedBox(height: 8),
            const Text('修改成功后，本机其他设备上已登录的会话将失效，需重新登录。',
                textAlign: TextAlign.center, style: AppTheme.muted),
          ],
        ],
      ),
    );
  }
}
