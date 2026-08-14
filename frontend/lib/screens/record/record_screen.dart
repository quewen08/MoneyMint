/// 记一笔页（设计稿「记一笔 ★」核心）：复式记账，实时借贷平衡指示，可增删分录。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/posting.dart';
import '../../utils/money.dart';
import '../../widgets/balance_meter.dart';
import '../../widgets/common.dart';

class RecordScreen extends StatefulWidget {
  const RecordScreen({super.key});

  @override
  State<RecordScreen> createState() => _RecordScreenState();
}

class _RecordScreenState extends State<RecordScreen> {
  final _descCtl = TextEditingController();
  final _dateCtl = TextEditingController(text: todayStr());
  final List<_PostingRow> _rows = [];
  bool _busy = false;
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    final accounts = AppScope.of(context).ledger.accounts;
    final first = accounts.isNotEmpty ? accounts.first.uuid : '';
    _rows
      ..add(_PostingRow(first))
      ..add(_PostingRow(first));
  }

  @override
  void dispose() {
    _descCtl.dispose();
    _dateCtl.dispose();
    for (final r in _rows) {
      r.dispose();
    }
    super.dispose();
  }

  Map<String, Decimal> _balances() {
    final m = <String, Decimal>{};
    for (final r in _rows) {
      final c = r.commodity.text.trim();
      final a = Decimal.tryParse(r.amount.text.trim());
      if (c.isEmpty || a == null) continue;
      m[c] = (m[c] ?? Decimal.zero) + a;
    }
    return m;
  }

  bool get _balanced => _balances().values.every((v) => v == Decimal.zero);

  String get _imbalanceDetail {
    final parts = _balances().entries
        .where((e) => e.value != Decimal.zero)
        .map((e) => formatMoney(e.value.toString(), e.key))
        .toList();
    return parts.isEmpty ? '' : parts.join('，');
  }

  Future<void> _submit() async {
    final ledger = AppScope.of(context).ledger;
    final valid = _rows
        .where((r) => r.amount.text.isNotEmpty && Decimal.tryParse(r.amount.text) != null)
        .toList();
    if (valid.length < 2) {
      showAppSnack(context, '至少需要 2 条有效分录', warn: true);
      return;
    }
    if (!_balanced) {
      showAppSnack(context, '借贷不平衡：$_imbalanceDetail', warn: true);
      return;
    }
    final postings = valid
        .map((r) => LocalPosting(
              accountUuid: r.accountUuid,
              commodity: r.commodity.text.trim(),
              amount: r.amount.text,
            ))
        .toList();
    setState(() => _busy = true);
    try {
      await ledger.createTransaction(
        date: _dateCtl.text,
        description: _descCtl.text,
        postings: postings,
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

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final balanced = _balanced;
    return Scaffold(
      appBar: AppBar(title: const Text('记一笔')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          AppCard(
            child: Column(
              children: [
                _Field(label: '日期', child: TextField(
                  controller: _dateCtl,
                  decoration: const InputDecoration(hintText: 'YYYY-MM-DD'),
                )),
                const SizedBox(height: 12),
                _Field(label: '摘要', child: TextField(
                  controller: _descCtl,
                  decoration: const InputDecoration(hintText: '如：超市采购'),
                )),
              ],
            ),
          ),
          const SizedBox(height: 12),
          SectionTitle('借贷分录（至少 2 条，必须平衡）'),
          for (var i = 0; i < _rows.length; i++)
            _PostingCard(
              key: ValueKey(_rows[i]),
              row: _rows[i],
              accounts: ledger.accounts,
              onRemove: _rows.length > 2
                  ? () => setState(() => _rows.removeAt(i))
                  : null,
            ),
          TextButton.icon(
            onPressed: () => setState(() {
              final first = ledger.accounts.isNotEmpty ? ledger.accounts.first.uuid : '';
              _rows.add(_PostingRow(first));
            }),
            icon: const Icon(Icons.add),
            label: const Text('添加分录'),
          ),
          const SizedBox(height: 8),
          BalanceMeter(balanced: balanced, detail: _imbalanceDetail),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: balanced && !_busy ? _submit : null,
            child: _busy
                ? const SizedBox(
                    height: 18,
                    width: 18,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                  )
                : const Text('保存交易'),
          ),
        ],
      ),
    );
  }
}

class _Field extends StatelessWidget {
  final String label;
  final Widget child;
  const _Field({required this.label, required this.child});

  @override
  Widget build(BuildContext context) => Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          SizedBox(width: 48, child: Text(label, style: AppTheme.muted)),
          Expanded(child: child),
        ],
      );
}

class _PostingCard extends StatelessWidget {
  final _PostingRow row;
  final List<dynamic> accounts; // List<LocalAccount>
  final VoidCallback? onRemove;
  const _PostingCard({super.key, required this.row, required this.accounts, this.onRemove});

  @override
  Widget build(BuildContext context) {
    final items = accounts
        .map<DropdownMenuItem<String>>(
            (a) => DropdownMenuItem<String>(value: a.uuid, child: Text(a.display)))
        .toList();
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.line),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Expanded(
                flex: 3,
                child: DropdownButtonFormField<String>(
                  value: items.any((it) => it.value == row.accountUuid)
                      ? row.accountUuid
                      : null,
                  items: items,
                  onChanged: (v) => row.accountUuid = v!,
                  decoration: const InputDecoration(hintText: '选择账户'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                flex: 2,
                child: TextField(
                  controller: row.commodity,
                  decoration: const InputDecoration(hintText: '币种'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                flex: 2,
                child: TextField(
                  controller: row.amount,
                  keyboardType: const TextInputType.numberWithOptions(signed: true),
                  textAlign: TextAlign.right,
                  decoration: const InputDecoration(hintText: '金额(可负)'),
                ),
              ),
              if (onRemove != null)
                IconButton(
                  icon: const Icon(Icons.remove_circle_outline, color: AppColors.warn),
                  onPressed: onRemove,
                )
              else
                const SizedBox(width: 40),
            ],
          ),
        ],
      ),
    );
  }
}

class _PostingRow {
  String accountUuid;
  final commodity = TextEditingController()..text = 'CNY';
  final amount = TextEditingController();
  _PostingRow(this.accountUuid);
  void dispose() {
    commodity.dispose();
    amount.dispose();
  }
}
