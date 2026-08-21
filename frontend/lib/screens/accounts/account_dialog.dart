/// 新建/编辑账户弹窗（设计稿「新建账户」）：图标 + 颜色选择、类型、父分类、
/// 币种、期初余额（自动生成 Equity:Opening-Balances 对冲交易）。
/// 分类（支出/收入）本质就是选了父账户的 Expenses/Income 类型账户。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/posting.dart';
import '../../widgets/account_icon.dart';
import '../../widgets/common.dart';

const _emojiChoices = [
  '💰', '💵', '🏦', '💳', '🔵', '🟢', '📱', '🏠', '🏢', '🍜', '🛒', '🚌',
  '🛍️', '🎮', '💊', '👕', '📚', '⚽', '✈️', '💻', '🎁', '📈', '🧧', '⚖️', '💼', '➕',
];

const _colorChoices = [
  '#3B6EF6', '#2BA471', '#E8590C', '#9C36B5', '#1971C2', '#0CA678',
  '#E03131', '#F59F00', '#15AABF', '#4263EB', '#D6336C', '#868E96',
];

class AccountDialog extends StatefulWidget {
  final LocalAccount? account; // 传入则为编辑（仅预填，保存走同一创建逻辑）
  final String defaultType;
  const AccountDialog({super.key, this.account, this.defaultType = 'Assets'});

  @override
  State<AccountDialog> createState() => _AccountDialogState();
}

class _AccountDialogState extends State<AccountDialog> {
  final _nameCtl = TextEditingController();
  final _dispCtl = TextEditingController();
  final _openBalCtl = TextEditingController();
  String _type;
  final String _commodity = 'CNY';
  String? _icon;
  String? _color;
  String? _parentUuid;
  bool _busy = false;

  _AccountDialogState() : _type = 'Assets';

  @override
  void initState() {
    super.initState();
    final a = widget.account;
    _type = a?.type ?? widget.defaultType;
    _nameCtl.text = a?.name ?? '';
    _dispCtl.text = a?.displayName ?? '';
    _icon = a?.icon;
    _color = a?.color;
    _parentUuid = a?.parentUuid;
  }

  @override
  void dispose() {
    _nameCtl.dispose();
    _dispCtl.dispose();
    _openBalCtl.dispose();
    super.dispose();
  }

  List<LocalAccount> get _parentCandidates {
    // 仅支出/收入可挂父分类（三级结构）。
    final ledger = AppScope.of(context).ledger;
    if (_type != 'Expenses' && _type != 'Income') return const [];
    return ledger.accounts
        .where((a) => a.type == _type && a.parentUuid == null && a.uuid != widget.account?.uuid)
        .toList();
  }

  String _finalName() {
    final leaf = _nameCtl.text.trim();
    if (_parentUuid == null) return leaf;
    final parent = AppScope.of(context).ledger.accounts.firstWhere(
      (a) => a.uuid == _parentUuid,
      orElse: () => LocalAccount(uuid: '', name: '', type: _type, openDate: ''),
    );
    if (parent.name.isEmpty) return leaf;
    return '${parent.name}:$leaf';
  }

  Future<void> _save() async {
    final name = _finalName();
    if (name.isEmpty) {
      showAppSnack(context, '请输入账户名（ASCII，如 Assets:Cash）', warn: true);
      return;
    }
    if (_parentUuid != null && !RegExp(r'^[A-Za-z0-9:_]+$').hasMatch(name)) {
      showAppSnack(context, '带分类的账户名只能含字母/数字/冒号', warn: true);
      return;
    }
    final ledger = AppScope.of(context).ledger;
    setState(() => _busy = true);
    try {
      final created = await ledger.createAccount(
        name: name,
        type: _type,
        openDate: todayStr(),
        commodity: null,
        icon: _icon,
        color: _color,
        parentUuid: _parentUuid,
        subType: null,
      );
      // 期初余额：自动生成 Equity:Opening-Balances 对冲交易（Equity 自身不生成）。
      final ob = Decimal.tryParse(_openBalCtl.text.trim());
      if (ob != null && ob > Decimal.zero && _type != 'Equity') {
        final equity = ledger.accounts.firstWhere(
          (a) => a.type == 'Equity' &&
              a.name.startsWith('Equity:Opening'),
          orElse: () => ledger.accounts.firstWhere(
            (a) => a.type == 'Equity',
            orElse: () => LocalAccount(uuid: '', name: '', type: 'Equity', openDate: ''),
          ),
        );
        if (equity.uuid.isNotEmpty) {
          await ledger.createTransaction(
            date: todayStr(),
            description: '期初余额',
            postings: [
              LocalPosting(accountUuid: created.uuid, commodity: _commodity, amount: ob.toString()),
              LocalPosting(accountUuid: equity.uuid, commodity: _commodity, amount: (-ob).toString()),
            ],
          );
        } else {
          showAppSnack(context, '未找到权益账户，期初余额未生成', warn: true);
        }
      }
      if (!mounted) return;
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
    final parents = _parentCandidates;
    return Dialog(
      insetPadding: const EdgeInsets.all(16),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(widget.account == null ? '新建账户' : '编辑账户',
                    style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
                const SizedBox(height: 16),
                Row(
                  children: [
                    InkWell(
                      onTap: () => _pickIcon(),
                      child: AccountIcon(
                        LocalAccount(
                          uuid: '',
                          name: '',
                          type: _type,
                          openDate: '',
                          icon: _icon,
                          color: _color,
                        ),
                        size: 52,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('图标 / 颜色', style: AppTheme.sectionTitle),
                          Wrap(
                            spacing: 6,
                            children: _colorChoices
                                .map((c) => InkWell(
                                      onTap: () => setState(() => _color = c),
                                      child: Container(
                                        width: 22,
                                        height: 22,
                                        decoration: BoxDecoration(
                                          color: parseHexColor(c),
                                          borderRadius: BorderRadius.circular(6),
                                          border: Border.all(
                                            color: _color == c
                                                ? AppColors.ink
                                                : Colors.transparent,
                                            width: 2,
                                          ),
                                        ),
                                      ),
                                    ))
                                .toList(),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 14),
                DropdownButtonFormField<String>(
                  value: _type,
                  items: const ['Assets', 'Liabilities', 'Equity', 'Income', 'Expenses']
                      .map((t) => DropdownMenuItem(value: t, child: Text(t)))
                      .toList(),
                  onChanged: (v) => setState(() => _type = v!),
                  decoration: const InputDecoration(labelText: '类型'),
                ),
                const SizedBox(height: 12),
                if (parents.isNotEmpty)
                  DropdownButtonFormField<String>(
                    value: _parentUuid,
                    isExpanded: true,
                    items: [
                      const DropdownMenuItem(value: null, child: Text('（无，作为二级分类）')),
                      ...parents.map((a) => DropdownMenuItem(value: a.uuid, child: Text(a.display))),
                    ],
                    onChanged: (v) => setState(() => _parentUuid = v),
                    decoration: const InputDecoration(labelText: '父分类'),
                  ),
                if (parents.isNotEmpty) const SizedBox(height: 12),
                TextField(
                  controller: _nameCtl,
                  decoration: InputDecoration(
                    labelText: _parentUuid == null ? '账户名(ASCII)' : '分类英文标识',
                    hintText: _parentUuid == null ? '如 Assets:Cash' : '如 Snack',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _dispCtl,
                  decoration: const InputDecoration(labelText: '显示名（中文，可选）'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _openBalCtl,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(
                    labelText: '期初余额（可选）',
                    hintText: '0.00',
                    suffixText: _commodity,
                  ),
                ),
                const SizedBox(height: 18),
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
          ),
        ),
      ),
    );
  }

  void _pickIcon() {
    showDialog(
      context: context,
      builder: (_) => Dialog(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Wrap(
            spacing: 10,
            runSpacing: 10,
            children: _emojiChoices
                .map((e) => InkWell(
                      onTap: () {
                        setState(() => _icon = e);
                        Navigator.pop(context);
                      },
                      child: Container(
                        width: 40,
                        height: 40,
                        decoration: BoxDecoration(
                          color: const Color(0xFFF3F4F6),
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: Center(child: Text(e, style: const TextStyle(fontSize: 22))),
                      ),
                    ))
                .toList(),
          ),
        ),
      ),
    );
  }
}
