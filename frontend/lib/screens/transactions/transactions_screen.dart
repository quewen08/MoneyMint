/// 流水页（设计稿「流水」）：按日期分组列出交易，可跳转详情；支持多维筛选与搜索。
/// 0.4-D 增强：搜索框（描述/标签/账户名/金额）+ 标签多选 chip + 账户筛选 +
/// 日期区间 + 含已关闭开关 + 列表/月历双视图（移动端）。
/// 批 4 / F-10 增强：宽屏 ≥1024 走五列表格（日期/说明/账户·分类/标签/金额），
/// 日期与金额列可点表头排序；独立页（非 shell 内容）自带 AppBar 与账本 pill。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/common.dart';
import '../../widgets/calendar_view.dart';
import '../../widgets/ledger_switcher.dart';
import '../../widgets/revamp.dart';
import 'txn_detail_screen.dart';

class TransactionsScreen extends StatefulWidget {
  /// 预设账户筛选（账户详情「查看流水」进入时传入）。
  final String? filterAccountUuid;

  /// 单日筛选（月历点击进入时传入），非空则只显示该日交易。
  final String? filterDate;

  /// [embedded]=true 时不包 Scaffold（作为主框架 content 渲染，AppBar 由 shell 提供）；
  /// 默认 false：作为 Navigator.push 的独立页，自带 AppBar（含返回键与账本 pill）。
  final bool embedded;

  const TransactionsScreen({
    super.key,
    this.filterAccountUuid,
    this.filterDate,
    this.embedded = false,
  });

  @override
  State<TransactionsScreen> createState() => _TransactionsScreenState();
}

class _TransactionsScreenState extends State<TransactionsScreen> {
  final _searchCtl = TextEditingController();
  final Set<String> _tags = {};
  String? _account; // 账户筛选 uuid（null=全部）
  String? _member; // 成员筛选（F-6：按 createdByName，null=全部成员）
  String? _dateFrom;
  String? _dateTo;
  bool _includeClosed = false;
  bool _calendar = false;

  // PC 表格排序：列 0=日期 / 1=金额；默认按日期倒序（最新在前）。
  int _sortCol = 0;
  bool _asc = false;

  /// 表格视图专用排序（不影响移动端按日分组的倒序语义）。
  List<LocalTxn> _sortedForTable(LedgerController ledger, List<LocalTxn> list) {
    final out = [...list];
    out.sort((a, b) {
      int c;
      if (_sortCol == 1) {
        final da =
            (Decimal.tryParse(ledger.txnHeadline(a) ?? '0') ?? Decimal.zero).abs();
        final db =
            (Decimal.tryParse(ledger.txnHeadline(b) ?? '0') ?? Decimal.zero).abs();
        c = da.compareTo(db);
      } else {
        c = a.date.compareTo(b.date);
      }
      if (c != 0) return _asc ? c : -c;
      return b.uuid.compareTo(a.uuid); // 同值稳定，避免重排时行跳动
    });
    return out;
  }

  void _toggleSort(int col) {
    setState(() {
      if (_sortCol == col) {
        _asc = !_asc;
      } else {
        _sortCol = col;
        _asc = false;
      }
    });
  }

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
      _member != null ||
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
      _member = null;
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
    // F-6：成员筛选（记账人）。createdByName 为空的本地交易视作「我」记的，
    // 选择了成员后这些无名交易不再展示，避免误认为该成员漏记。
    if (_member != null) {
      final name = t.createdByName;
      if (name == null || name.isEmpty || name != _member) return false;
    }
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
        // 成员列表（F-6）：当前账本内出现过的记账人。
        final members = <String>{
          for (final t in ledger.txns)
            if (t.createdByName != null && t.createdByName!.isNotEmpty) t.createdByName!,
        }.toList()..sort();
        final accountOpts = ledger.accounts
            .where((a) => _includeClosed || !a.isClosed || a.uuid == _account)
            .toList();

        final filtered = ledger.txns.where((t) => _match(ledger, t)).toList();

        final content = Column(
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
              members: members,
              member: _member,
              onMember: (v) => setState(() => _member = v),
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
                          accountTypes: {
                            for (final a in ledger.accounts) a.uuid: a.type
                          },
                          onSelectDate: (date) => Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) => TransactionsScreen(filterDate: date),
                            ),
                          ),
                        ),
                      ],
                    )
                  // F-10：宽屏走表格（可排序），窄屏保留按日分组的列表。
                  : wide
                      ? _TxnTable(
                          ledger: ledger,
                          list: _sortedForTable(ledger, filtered),
                          sortCol: _sortCol,
                          asc: _asc,
                          onSort: _toggleSort,
                          onRefresh: () => ledger.syncNow().then((_) {}),
                        )
                      : _ListBody(
                          ledger: ledger,
                          list: filtered,
                          wide: wide,
                          onRefresh: () => ledger.syncNow().then((_) {}),
                        ),
            ),
          ],
        );

        if (widget.embedded) return content;

        // 作为独立页被 push 时自带 AppBar：此前直接返回 Column，导致无返回键、
        // 也无法切换账本（从账单页 📋 / 账户详情进来时体验断层）。
        final auth = AppScope.of(context).auth;
        return Scaffold(
          appBar: AppBar(
            title: const Text('交易流水'),
            actions: [
              ListenableBuilder(
                listenable: auth,
                builder: (ctx, _) => Padding(
                  padding: const EdgeInsets.only(right: 12),
                  child: Center(
                    child: LedgerPill(
                      light: true,
                      ledgerName: auth.currentLedger?.name ?? '账本',
                      onTap: () => showLedgerSwitcher(context),
                    ),
                  ),
                ),
              ),
            ],
          ),
          body: content,
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
  final List<String> members;
  final String? member;
  final void Function(String?) onMember;
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
    required this.members,
    required this.member,
    required this.onMember,
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
          if (members.isNotEmpty) ...[
            const SizedBox(height: 8),
            // F-6：成员筛选 chips（彩色圆点标识记账人）。
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: [
                _memberChip('全部成员', null, member == null, () => onMember(null)),
                for (final m in members)
                  _memberChip(m, m, member == m, () => onMember(member == m ? null : m)),
              ],
            ),
          ],
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
  /// 成员 chip：彩色圆点标识记账人，色值按名字稳定散列。
  Widget _memberChip(String label, String? value, bool on, VoidCallback onTap) {
    final dotColor = value == null ? AppColors.blue : AppColors.softOf(value);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: on ? AppColors.blue.withValues(alpha: .12) : Colors.white,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: on ? AppColors.blue : AppColors.line),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (value != null) ...[
              Container(width: 8, height: 8,
                  decoration: BoxDecoration(color: dotColor, shape: BoxShape.circle)),
              const SizedBox(width: 5),
            ],
            Text(label,
                style: TextStyle(
                  fontSize: 12,
                  color: on ? AppColors.blue : AppColors.ink,
                  fontWeight: on ? FontWeight.w700 : FontWeight.w500,
                )),
          ],
        ),
      ),
    );
  }
}

/// PC 表格视图（F-10）：日期 / 说明 / 账户·分类 / 标签 / 金额 五列。
///
/// 用 ListView.separated 而非 DataTable：家庭账本动辄数千笔，
/// DataTable 会一次性构建全部行（无虚拟化），滚动会明显掉帧。
class _TxnTable extends StatelessWidget {
  final LedgerController ledger;
  final List<LocalTxn> list;
  final int sortCol;
  final bool asc;
  final ValueChanged<int> onSort;
  final Future<void> Function() onRefresh;

  const _TxnTable({
    required this.ledger,
    required this.list,
    required this.sortCol,
    required this.asc,
    required this.onSort,
    required this.onRefresh,
  });

  static const double _wDate = 96;
  static const double _wAmount = 132;

  @override
  Widget build(BuildContext context) {
    if (list.isEmpty) {
      return Center(
        child: Text(
          ledger.txns.isEmpty ? '还没有交易，点「记一笔」开始记账' : '没有匹配的交易',
          style: const TextStyle(color: AppColors.sub),
        ),
      );
    }
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 1280),
        child: Column(
          children: [
            Container(
              color: AppColors.bg,
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 9),
              child: Row(
                children: [
                  _head('日期', 0, width: _wDate),
                  _head('说明', null, flex: 4),
                  _head('账户 / 分类', null, flex: 4),
                  _head('标签', null, flex: 3),
                  _head('金额', 1, width: _wAmount, right: true),
                ],
              ),
            ),
            const Divider(height: 1, color: AppColors.line),
            Expanded(
              child: RefreshIndicator(
                onRefresh: onRefresh,
                child: ListView.separated(
                  physics: const AlwaysScrollableScrollPhysics(),
                  itemCount: list.length,
                  separatorBuilder: (_, __) =>
                      const Divider(height: 1, color: AppColors.line),
                  itemBuilder: (context, i) => _row(context, list[i]),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _head(String label, int? col,
      {double? width, int flex = 1, bool right = false}) {
    final on = col != null && sortCol == col;
    final row = Row(
      mainAxisSize: right ? MainAxisSize.min : MainAxisSize.max,
      mainAxisAlignment: right ? MainAxisAlignment.end : MainAxisAlignment.start,
      children: [
        Text(label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w700,
              color: on ? AppColors.ink : AppColors.sub,
            )),
        if (col != null) ...[
          const SizedBox(width: 2),
          Icon(
            on
                ? (asc ? Icons.arrow_upward : Icons.arrow_downward)
                : Icons.unfold_more,
            size: 13,
            color: on ? AppColors.ink : AppColors.sub,
          ),
        ],
      ],
    );
    final body = col == null
        ? row
        : InkWell(
            onTap: () => onSort(col),
            child: Padding(padding: const EdgeInsets.symmetric(vertical: 2), child: row),
          );
    return width != null
        ? SizedBox(width: width, child: body)
        : Expanded(flex: flex, child: body);
  }

  Widget _row(BuildContext context, LocalTxn t) {
    final head = ledger.txnHeadline(t);
    final commodity = t.postings.isNotEmpty ? t.postings.first.commodity : 'CNY';
    final isReversal = t.reversedOf != null;
    final isReversed = ledger.reversedUuids.contains(t.uuid);
    return InkWell(
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: t)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 9),
        child: Row(
          children: [
            SizedBox(
              width: _wDate,
              child: Text(
                t.date,
                style: const TextStyle(
                  fontSize: 12.5,
                  color: AppColors.ink,
                  fontFeatures: [FontFeature.tabularFigures()],
                ),
              ),
            ),
            Expanded(
              flex: 4,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          t.description.isEmpty ? '(无说明)' : t.description,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 13, fontWeight: FontWeight.w500),
                        ),
                      ),
                      if (isReversal)
                        _badge('冲正', AppColors.warn),
                      if (isReversed)
                        _badge('已冲正', Colors.grey),
                    ],
                  ),
                  if (t.createdByName != null && t.createdByName!.isNotEmpty)
                    Text(t.createdByName!,
                        style: const TextStyle(fontSize: 10.5, color: AppColors.sub)),
                ],
              ),
            ),
            Expanded(
              flex: 4,
              child: Text(
                ledger.txnFlow(t),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTheme.muted,
              ),
            ),
            Expanded(
              flex: 3,
              child: Text(
                t.tags.isEmpty ? '—' : t.tags.map((e) => '#$e').join(' '),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11.5,
                  color: t.tags.isEmpty ? AppColors.sub : AppColors.blue,
                ),
              ),
            ),
            SizedBox(
              width: _wAmount,
              child: Align(
                alignment: Alignment.centerRight,
                child: head != null
                    ? AmountText(head, commodity, size: 13.5)
                    : const SizedBox.shrink(),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _badge(String text, Color color) => Container(
        margin: const EdgeInsets.only(left: 6),
        padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
        decoration: BoxDecoration(
          color: color.withValues(alpha: .14),
          borderRadius: BorderRadius.circular(4),
        ),
        child: Text(text, style: TextStyle(fontSize: 10, color: color)),
      );
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
            ledger.txns.isEmpty ? '还没有交易，点「记一笔」开始记账' : '没有匹配的交易',
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
