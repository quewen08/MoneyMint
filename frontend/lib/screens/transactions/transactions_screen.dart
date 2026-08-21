/// 流水页（设计稿「流水」）：按日期分组列出交易，可跳转详情；支持多维筛选与搜索。
/// 0.4-D 增强：搜索框（描述/标签/账户名/金额）+ 标签多选 chip + 账户筛选 +
/// 日期区间 + 含已关闭开关 + 列表/月历双视图（移动端）。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/common.dart';
import '../../widgets/calendar_view.dart';
import 'txn_detail_screen.dart';

class TransactionsScreen extends StatefulWidget {
  /// 预设账户筛选（账户详情「查看流水」进入时传入）。
  final String? filterAccountUuid;
  /// 单日筛选（月历点击进入时传入），非空则只显示该日交易。
  final String? filterDate;
  const TransactionsScreen({super.key, this.filterAccountUuid, this.filterDate});

  @override
  State<TransactionsScreen> createState() => _TransactionsScreenState();
}

class _TransactionsScreenState extends State<TransactionsScreen> {
  final _searchCtl = TextEditingController();
  final Set<String> _tags = {};
  String? _account; // 账户筛选 uuid（null=全部）
  String? _dateFrom;
  String? _dateTo;
  bool _includeClosed = false;
  bool _calendar = false;

  @override
  void initState() {
    super.initState();
    _account = widget.filterAccountUuid;
  }

  @override
  void dispose() {
    _searchCtl.dispose();
    super.dispose();
  }

  bool get _hasFilter =>
      _account != widget.filterAccountUuid ||
      _tags.isNotEmpty ||
      _dateFrom != null ||
      _dateTo != null ||
      _searchCtl.text.isNotEmpty ||
      widget.filterDate != null;

  Future<void> _pickDate(bool isFrom) async {
    final initial = DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
    );
    if (picked == null) return;
    setState(() {
      final s = '${picked.year}-${picked.month.toString().padLeft(2, '0')}-${picked.day.toString().padLeft(2, '0')}';
      if (isFrom) {
        _dateFrom = s;
      } else {
        _dateTo = s;
      }
    });
  }

  void _clearFilters() {
    setState(() {
      _tags.clear();
      _account = widget.filterAccountUuid;
      _dateFrom = null;
      _dateTo = null;
      _searchCtl.clear();
    });
  }

  bool _match(LedgerController ledger, LocalTxn t) {
    if (widget.filterAccountUuid != null &&
        !t.postings.any((p) => p.accountUuid == widget.filterAccountUuid)) {
      return false;
    }
    if (_account != null && !t.postings.any((p) => p.accountUuid == _account)) {
      return false;
    }
    if (widget.filterDate != null && t.date != widget.filterDate) return false;
    if (_dateFrom != null && t.date.compareTo(_dateFrom!) < 0) return false;
    if (_dateTo != null && t.date.compareTo(_dateTo!) > 0) return false;
    if (_tags.isNotEmpty && !t.tags.any((tg) => _tags.contains(tg))) return false;
    final q = _searchCtl.text.trim().toLowerCase();
    if (q.isNotEmpty) {
      final hay = [
        t.description.toLowerCase(),
        t.tags.join(' ').toLowerCase(),
        t.postings.map((p) => ledger.accountName(p.accountUuid).toLowerCase()).join(' '),
        t.postings.map((p) => p.amount).join(' '),
      ].join(' ');
      if (!hay.contains(q)) return false;
    }
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context);
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        final allTags = <String>{
          for (final t in ledger.txns) ...t.tags,
        }.toList()..sort();
        final accountOpts = ledger.accounts
            .where((a) => _includeClosed || !a.isClosed || a.uuid == _account)
            .toList();

        final filtered = ledger.txns.where((t) => _match(ledger, t)).toList();

        return Column(
          children: [
            _FilterBar(
              searchCtl: _searchCtl,
              onSearch: () => setState(() {}),
              allTags: allTags,
              selectedTags: _tags,
              onToggleTag: (tg) => setState(() {
                if (_tags.contains(tg)) {
                  _tags.remove(tg);
                } else {
                  _tags.add(tg);
                }
              }),
              accountOpts: accountOpts,
              account: _account,
              onAccount: (v) => setState(() => _account = v),
              dateFrom: _dateFrom,
              dateTo: _dateTo,
              onPickDate: _pickDate,
              includeClosed: _includeClosed,
              onIncludeClosed: (v) => setState(() => _includeClosed = v),
              calendar: _calendar,
              onToggleCalendar: (v) => setState(() => _calendar = v),
              hasFilter: _hasFilter,
              onClear: _clearFilters,
            ),
            Expanded(
              child: _calendar
                  ? ListView(
                      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                      children: [
                        CalendarView(
                          txns: filtered,
                          onSelectDate: (date) => Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) => TransactionsScreen(filterDate: date),
                            ),
                          ),
                        ),
                      ],
                    )
                  : _ListBody(ledger: ledger, list: filtered, wide: wide, onRefresh: () => ledger.syncNow().then((_) {})),
            ),
          ],
        );
      },
    );
  }
}

/// 筛选栏：搜索 + 标签 chip + 账户下拉 + 日期区间 + 含已关闭 + 视图切换。
class _FilterBar extends StatelessWidget {
  final TextEditingController searchCtl;
  final VoidCallback onSearch;
  final List<String> allTags;
  final Set<String> selectedTags;
  final void Function(String) onToggleTag;
  final List<LocalAccount> accountOpts;
  final String? account;
  final void Function(String?) onAccount;
  final String? dateFrom;
  final String? dateTo;
  final void Function(bool) onPickDate;
  final bool includeClosed;
  final void Function(bool) onIncludeClosed;
  final bool calendar;
  final void Function(bool) onToggleCalendar;
  final bool hasFilter;
  final VoidCallback onClear;
  const _FilterBar({
    required this.searchCtl,
    required this.onSearch,
    required this.allTags,
    required this.selectedTags,
    required this.onToggleTag,
    required this.accountOpts,
    required this.account,
    required this.onAccount,
    required this.dateFrom,
    required this.dateTo,
    required this.onPickDate,
    required this.includeClosed,
    required this.onIncludeClosed,
    required this.calendar,
    required this.onToggleCalendar,
    required this.hasFilter,
    required this.onClear,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
      color: AppColors.bg,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: searchCtl,
                  onChanged: (_) => onSearch(),
                  decoration: InputDecoration(
                    hintText: '搜索描述 / 标签 / 账户 / 金额',
                    prefixIcon: const Icon(Icons.search, size: 20),
                    suffixIcon: searchCtl.text.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear, size: 18),
                            onPressed: () {
                              searchCtl.clear();
                              onSearch();
                            },
                          )
                        : null,
                    contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(10),
                      borderSide: BorderSide.none,
                    ),
                    filled: true,
                    fillColor: Colors.white,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              SegmentedButton<bool>(
                showSelectedIcon: false,
                segments: const [
                  ButtonSegment(value: false, icon: Icon(Icons.view_list, size: 18)),
                  ButtonSegment(value: true, icon: Icon(Icons.calendar_month, size: 18)),
                ],
                selected: {calendar},
                onSelectionChanged: (s) => onToggleCalendar(s.first),
              ),
            ],
          ),
          if (allTags.isNotEmpty) ...[
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: allTags.map((tg) {
                final on = selectedTags.contains(tg);
                return FilterChip(
                  label: Text('#$tg', style: const TextStyle(fontSize: 12)),
                  selected: on,
                  onSelected: (_) => onToggleTag(tg),
                  visualDensity: VisualDensity.compact,
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                );
              }).toList(),
            ),
          ],
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              DropdownButton<String?>(
                value: account,
                hint: const Text('全部账户', style: TextStyle(fontSize: 13)),
                underline: const SizedBox.shrink(),
                items: [
                  const DropdownMenuItem(value: null, child: Text('全部账户')),
                  for (final a in accountOpts)
                    DropdownMenuItem(value: a.uuid, child: Text(a.display)),
                ],
                onChanged: onAccount,
              ),
              TextButton.icon(
                onPressed: () => onPickDate(true),
                icon: const Icon(Icons.date_range, size: 16),
                label: Text(dateFrom ?? '起始', style: const TextStyle(fontSize: 13)),
              ),
              const Text('~', style: TextStyle(color: AppColors.sub)),
              TextButton(
                onPressed: () => onPickDate(false),
                child: Text(dateTo ?? '截止', style: const TextStyle(fontSize: 13)),
              ),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Checkbox(
                    value: includeClosed,
                    onChanged: (v) => onIncludeClosed(v ?? false),
                    visualDensity: VisualDensity.compact,
                    materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  const Text('含已关闭', style: TextStyle(fontSize: 13)),
                ],
              ),
              if (hasFilter)
                TextButton.icon(
                  onPressed: onClear,
                  icon: const Icon(Icons.filter_alt_off, size: 16),
                  label: const Text('清除', style: TextStyle(fontSize: 13)),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

/// 列表主体：按日期分组，点击进详情。
class _ListBody extends StatelessWidget {
  final LedgerController ledger;
  final List<LocalTxn> list;
  final bool wide;
  final Future<void> Function() onRefresh;
  const _ListBody({required this.ledger, required this.list, required this.wide, required this.onRefresh});

  @override
  Widget build(BuildContext context) {
    if (list.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            ledger.txns.isEmpty ? '还没有交易，点右下角 记一笔' : '没有匹配的交易',
            style: const TextStyle(color: AppColors.sub),
          ),
        ),
      );
    }
    final groups = <String, List<LocalTxn>>{};
    for (final t in list) {
      groups.putIfAbsent(t.date, () => []).add(t);
    }
    final dates = groups.keys.toList();
    return RefreshIndicator(
      onRefresh: onRefresh,
      child: Center(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: wide ? 900 : double.infinity),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
            children: [
              for (final date in dates) ...[
                SectionTitle(date),
                AppCard(
                  padding: EdgeInsets.zero,
                  child: Column(
                    children: [
                      for (var i = 0; i < groups[date]!.length; i++) ...[
                        if (i > 0) const Divider(height: 1, color: AppColors.line),
                        _TxnTile(ledger: ledger, txn: groups[date]![i]),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 12),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _TxnTile extends StatelessWidget {
  final LedgerController ledger;
  final LocalTxn txn;
  const _TxnTile({required this.ledger, required this.txn});

  /// 记账人后缀：有 createdByName 时追加「· 记账人」。
  String _byLine(LocalTxn t) {
    if (t.createdByName == null || t.createdByName!.isEmpty) return '';
    return ' · ${t.createdByName}';
  }

  @override
  Widget build(BuildContext context) {
    final head = ledger.txnHeadline(txn);
    final headCommodity = txn.postings.isNotEmpty ? txn.postings.first.commodity : 'CNY';
    final isReversal = txn.reversedOf != null;
    final isReversed = ledger.reversedUuids.contains(txn.uuid);
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Row(
        children: [
          Expanded(
            child: Text(txn.description.isEmpty ? '(无说明)' : txn.description,
                style: const TextStyle(fontWeight: FontWeight.w500)),
          ),
          if (isReversal)
            Container(
              margin: const EdgeInsets.only(left: 6),
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: AppColors.warn.withOpacity(0.12),
                borderRadius: BorderRadius.circular(4),
              ),
              child: const Text('冲正',
                  style: TextStyle(fontSize: 11, color: AppColors.warn)),
            ),
          if (isReversed)
            Container(
              margin: const EdgeInsets.only(left: 6),
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: Colors.grey.shade300,
                borderRadius: BorderRadius.circular(4),
              ),
              child: const Text('已冲正',
                  style: TextStyle(fontSize: 11, color: Colors.black54)),
            ),
        ],
      ),
      subtitle: Text(
        ledger.txnFlow(txn) + _byLine(txn),
        style: AppTheme.muted,
      ),
      trailing: head != null ? AmountText(head, headCommodity) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}
