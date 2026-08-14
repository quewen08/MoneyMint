/// 新建账户页（设计稿「新建账户」）：账户名 / 类型 / 开户日期 / 币种限制。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/common.dart';

class AccountEditScreen extends StatefulWidget {
  const AccountEditScreen({super.key});

  @override
  State<AccountEditScreen> createState() => _AccountEditScreenState();
}

class _AccountEditScreenState extends State<AccountEditScreen> {
  final _nameCtl = TextEditingController();
  final _commCtl = TextEditingController();
  String _type = 'Assets';
  final _openDate = todayStr();
  bool _busy = false;

  @override
  void dispose() {
    _nameCtl.dispose();
    _commCtl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final name = _nameCtl.text.trim();
    if (name.isEmpty) {
      showAppSnack(context, '请输入账户名', warn: true);
      return;
    }
    setState(() => _busy = true);
    try {
      final ledger = AppScope.of(context).ledger;
      await ledger.createAccount(
        name: name,
        type: _type,
        openDate: _openDate,
        commodity: _commCtl.text.trim().isEmpty ? null : _commCtl.text.trim(),
      );
      if (!mounted) return;
      showAppSnack(context,
          '账户已创建（本地保存${ledger.online ? '并已同步' : '，离线待同步'}）');
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '创建失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('新建账户')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('账户名 *', style: AppTheme.sectionTitle),
                const SizedBox(height: 6),
                TextField(
                  controller: _nameCtl,
                  decoration: const InputDecoration(hintText: '如 支付宝 / Assets:Cash:CNY'),
                ),
                const SizedBox(height: 16),
                const Text('类型 *', style: AppTheme.sectionTitle),
                const SizedBox(height: 6),
                DropdownButtonFormField<String>(
                  value: _type,
                  items: const [
                    'Assets',
                    'Liabilities',
                    'Equity',
                    'Income',
                    'Expenses'
                  ].map((t) => DropdownMenuItem(value: t, child: Text(t))).toList(),
                  onChanged: (v) => setState(() => _type = v!),
                  decoration: const InputDecoration(),
                ),
                const SizedBox(height: 16),
                const Text('开户日期', style: AppTheme.sectionTitle),
                const SizedBox(height: 6),
                Text(_openDate, style: const TextStyle(fontWeight: FontWeight.w600)),
                const SizedBox(height: 16),
                const Text('币种限制（可选）', style: AppTheme.sectionTitle),
                const SizedBox(height: 6),
                TextField(
                  controller: _commCtl,
                  decoration: const InputDecoration(hintText: '如 CNY'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: _busy ? null : _save,
            child: _busy
                ? const SizedBox(
                    height: 18,
                    width: 18,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                  )
                : const Text('保存'),
          ),
        ],
      ),
    );
  }
}
