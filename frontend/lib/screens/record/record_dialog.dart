/// 记一笔弹窗（设计稿「记一笔」简化模式）：支出 / 收入 / 转账 三段切换，
/// 大金额输入 + 分类/账户下拉 + 日期/标签/描述，自动生成标准复式 posting。
/// 「高级」按钮跳转双录分录页（熟悉 Beancount 的用户）。
/// 底层仍调用 LedgerController.createTransaction，复用借贷平衡校验。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/posting.dart';
import '../../widgets/account_icon.dart';
import '../../widgets/common.dart';
import 'record_screen.dart';

class RecordDialog extends StatefulWidget {
  const RecordDialog({super.key});

  @override
  State<RecordDialog> createState() => _RecordDialogState();
}

class _RecordDialogState extends State<RecordDialog> {
  int _mode = 0; // 0 支出 / 1 收入 / 2 转账
  final _amountCtl = TextEditingController();
  final _descCtl = TextEditingController();
  final _tagsCtl = TextEditingController();
  final _dateCtl = TextEditingController(text: todayStr());
  final String _commodity = 'CNY';
  String? _categoryUuid; // 支出/收入：分类账户
  String? _assetUuid; // 支出/收入：资产/负债账户；转账：来源
  String? _toUuid; // 转账：目标账户
  bool _busy = false;
  Map<String, String> _defs = {}; // 默认账户（kind -> uuid），0.4-C
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _loadDefaults();
  }

  /// 加载默认账户并按当前模式预选（0.4-C）。
  Future<void> _loadDefaults() async {
    final defs = await AppScope.of(context).ledger.defaultAccounts;
    if (!mounted) return;
    setState(() {
      _defs = defs;
      _applyModeDefault();
    });
  }

  bool _exists(String? uuid) =>
      uuid != null &&
      AppScope.of(context).ledger.accounts.any((a) => a.uuid == uuid && !a.isClosed);
  String? _validDefault(String? uuid) => _exists(uuid) ? uuid : null;

  /// 按当前模式把对应默认账户填入下拉（默认账户不存在/已关闭则留空）。
  void _applyModeDefault() {
    if (_mode == 0) {
      _categoryUuid = _validDefault(_defs['expense']);
    } else if (_mode == 1) {
      _categoryUuid = _validDefault(_defs['income']);
    } else {
      _assetUuid = _validDefault(_defs['transferOut']);
      _toUuid = _validDefault(_defs['transferIn']);
    }
  }

  @override
  void dispose() {
    _amountCtl.dispose();
    _descCtl.dispose();
    _tagsCtl.dispose();
    _dateCtl.dispose();
    super.dispose();
  }

  List<String> get _tags => _tagsCtl.text
      .split(RegExp(r'[\s,，]+'))
      .map((e) => e.trim())
      .where((e) => e.isNotEmpty)
      .toList();

  List<LocalPosting> _buildPostings(Decimal amount) {
    final amt = amount.toString();
    if (_mode == 2) {
      // 转账：来源 -X，目标 +X
      return [
        LocalPosting(accountUuid: _assetUuid!, commodity: _commodity, amount: (-amount).toString()),
        LocalPosting(accountUuid: _toUuid!, commodity: _commodity, amount: amt),
      ];
    } else if (_mode == 0) {
      // 支出：资产/负债 -X，分类 +X
      return [
        LocalPosting(accountUuid: _assetUuid!, commodity: _commodity, amount: (-amount).toString()),
        LocalPosting(accountUuid: _categoryUuid!, commodity: _commodity, amount: amt),
      ];
    } else {
      // 收入：分类 -X，资产/负债 +X
      return [
        LocalPosting(accountUuid: _categoryUuid!, commodity: _commodity, amount: (-amount).toString()),
        LocalPosting(accountUuid: _assetUuid!, commodity: _commodity, amount: amt),
      ];
    }
  }

  String? _validate() {
    final amt = Decimal.tryParse(_amountCtl.text.trim());
    if (amt == null || amt <= Decimal.zero) return '请输入大于 0 的金额';
    if (_mode != 2 && _categoryUuid == null) return '请选择分类';
    if (_assetUuid == null) return '请选择账户';
    if (_mode == 2 && _toUuid == null) return '请选择目标账户';
    if (_mode == 2 && _toUuid == _assetUuid) return '来源与目标账户不能相同';
    return null;
  }

  Future<void> _submit() async {
    final err = _validate();
    if (err != null) {
      showAppSnack(context, err, warn: true);
      return;
    }
    final amount = Decimal.parse(_amountCtl.text.trim());
    final postings = _buildPostings(amount);
    final ledger = AppScope.of(context).ledger;
    // 0.4-C：回写「上次选择」为默认账户。
    try {
      if (_mode == 0 && _categoryUuid != null) {
        await ledger.setDefaultAccount('expense', _categoryUuid!);
      } else if (_mode == 1 && _categoryUuid != null) {
        await ledger.setDefaultAccount('income', _categoryUuid!);
      } else if (_mode == 2) {
        if (_assetUuid != null) await ledger.setDefaultAccount('transferOut', _assetUuid!);
        if (_toUuid != null) await ledger.setDefaultAccount('transferIn', _toUuid!);
      }
    } catch (_) {
      // 默认账户写入失败不阻断记账。
    }
    setState(() => _busy = true);
    try {
      await ledger.createTransaction(
        date: _dateCtl.text,
        description: _descCtl.text.trim(),
        postings: postings,
        tags: _tags,
      );
      if (!mounted) return;
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '提交失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Widget _accountDropdown(String? value, ValueChanged<String?> onChanged,
      List<LocalAccount> items, String hint) {
    return DropdownButtonFormField<String>(
      value: value,
      isExpanded: true,
      items: items
          .map((a) => DropdownMenuItem(
                value: a.uuid,
                child: Row(children: [
                  AccountIcon(a, size: 26),
                  const SizedBox(width: 8),
                  Expanded(child: Text(a.display, overflow: TextOverflow.ellipsis)),
                ]),
              ))
          .toList(),
      onChanged: onChanged,
      decoration: InputDecoration(hintText: hint),
    );
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final expenseCats = ledger.accounts.where((a) => a.type == 'Expenses' && !a.isClosed).toList();
    final incomeCats = ledger.accounts.where((a) => a.type == 'Income' && !a.isClosed).toList();
    final assetAccounts = ledger.accounts
        .where((a) => (a.type == 'Assets' || a.type == 'Liabilities') && !a.isClosed)
        .toList();
    final cats = _mode == 1 ? incomeCats : expenseCats;
    return Dialog(
      insetPadding: const EdgeInsets.all(16),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    const Text('记一笔', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
                    const Spacer(),
                    TextButton(
                      onPressed: () {
                        Navigator.pop(context);
                        Navigator.push(
                          context,
                          MaterialPageRoute(builder: (_) => const RecordScreen()),
                        );
                      },
                      child: const Text('高级'),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                Container(
                  decoration: BoxDecoration(
                    color: const Color(0xFFF3F4F6),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Row(
                    children: [
                      _ModeTab('支出', 0, _mode, (m) => setState(() {
                            _mode = m;
                            _applyModeDefault();
                          })),
                      _ModeTab('收入', 1, _mode, (m) => setState(() {
                            _mode = m;
                            _applyModeDefault();
                          })),
                      _ModeTab('转账', 2, _mode, (m) => setState(() {
                            _mode = m;
                            _applyModeDefault();
                          })),
                    ],
                  ),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _amountCtl,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  textAlign: TextAlign.right,
                  style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w800),
                  decoration: InputDecoration(
                    prefixText: '¥ ',
                    hintText: '0.00',
                    border: InputBorder.none,
                    filled: false,
                  ),
                ),
                const Divider(height: 1),
                const SizedBox(height: 12),
                if (_mode != 2)
                  _accountDropdown(_categoryUuid, (v) => setState(() => _categoryUuid = v),
                      cats, _mode == 1 ? '选择收入分类' : '选择支出分类'),
                if (_mode != 2) const SizedBox(height: 10),
                _accountDropdown(_assetUuid, (v) => setState(() => _assetUuid = v),
                    assetAccounts, _mode == 2 ? '来源账户' : '选择账户'),
                if (_mode == 2) const SizedBox(height: 10),
                if (_mode == 2)
                  _accountDropdown(_toUuid, (v) => setState(() => _toUuid = v),
                      assetAccounts, '目标账户'),
                const SizedBox(height: 10),
                TextField(
                  controller: _dateCtl,
                  readOnly: true,
                  onTap: () async {
                    final d = DateTime.tryParse(_dateCtl.text) ?? DateTime.now();
                    final picked = await showDatePicker(
                      context: context,
                      initialDate: d,
                      firstDate: DateTime(2000),
                      lastDate: DateTime(2100),
                    );
                    if (picked != null) {
                      _dateCtl.text = todayStr(picked);
                    }
                  },
                  decoration: const InputDecoration(hintText: '日期', suffixIcon: Icon(Icons.calendar_today)),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _tagsCtl,
                  decoration: const InputDecoration(hintText: '标签（空格或逗号分隔，可选）'),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _descCtl,
                  decoration: const InputDecoration(hintText: '备注（可选）'),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: _busy ? null : _submit,
                  child: _busy
                      ? const SizedBox(
                          height: 18,
                          width: 18,
                          child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                        )
                      : const Text('保存'),
                ),
                const SizedBox(height: 6),
                Text(
                  '本地保存${ledger.online ? '，已联网将自动同步' : '，离线待同步'}',
                  style: AppTheme.muted,
                  textAlign: TextAlign.center,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ModeTab extends StatelessWidget {
  final String label;
  final int mode;
  final int selected;
  final ValueChanged<int> onTap;
  const _ModeTab(this.label, this.mode, this.selected, this.onTap);

  @override
  Widget build(BuildContext context) {
    final active = mode == selected;
    return Expanded(
      child: InkWell(
        onTap: () => onTap(mode),
        borderRadius: BorderRadius.circular(8),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 10),
          decoration: BoxDecoration(
            color: active ? AppColors.blue : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Center(
            child: Text(
              label,
              style: TextStyle(
                color: active ? Colors.white : AppColors.sub,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
