/// 首页 / 账单（设计稿「账单」）：渐变汇总头 + 日历/列表 + 支出排行 + 最近交易。
/// 0.4-B：四宫格指标 + 近 12 月趋势；批 4 / F-10：宽屏 ≥1200 排三栏（日历 / 排行 / 汇总+快捷）。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../widgets/calendar_view.dart';
import '../../widgets/charts.dart';
import '../../widgets/common.dart';
import '../../widgets/identity_banner.dart';
import '../../widgets/ledger_switcher.dart';
import '../../widgets/privacy.dart';
import '../../widgets/revamp.dart';
import '../record/record_dialog.dart';
import '../transactions/txn_detail_screen.dart';
import '../transactions/transactions_screen.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({super.key});

  /// 0.4-D 之前：>1024 走独立的 PC 仪表盘（净资产卡 + 四宫格 + 趋势 + 资产负债权益分组）。
  /// 批 4 / F-10 起：两端统一走「账单首页」——PC 在 ≥1200 时排成三栏，
  /// 避免同一份数据维护两套视觉。净资产/四宫格/趋势已并入宽屏右栏，功能不丢。
  @override
  Widget build(BuildContext context) => const _BillHome();
}


class _MetricChip extends StatelessWidget {
  final String label;
  final String value;
  final Color color;
  const _MetricChip({required this.label, required this.value, required this.color});

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: AppTheme.cardDecoration,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Row(
              children: [
                Container(
                  width: 4,
                  height: 14,
                  decoration: BoxDecoration(
                    color: color,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
                const SizedBox(width: 6),
                Text(label, style: AppTheme.muted),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              value,
              style: const TextStyle(
                fontSize: 17,
                fontWeight: FontWeight.w700,
                color: AppColors.ink,
                fontFeatures: [FontFeature.tabularFigures()],
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      );
}

/// 近 12 月收支趋势迷你折线卡。
class _MiniTrendCard extends StatelessWidget {
  final LedgerController ledger;
  final bool wide;
  const _MiniTrendCard({required this.ledger, required this.wide});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts) a.uuid: a,
    };
    final now = DateTime.now();
    final points = <TrendPoint>[];

    // 选最大币种
    final totals = <String, Decimal>{};
    for (final t in ledger.txns) {
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc == null) continue;
        // 金额大小展示，与账本符号无关（选展示币种用）。
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        totals[p.commodity] = (totals[p.commodity] ?? Decimal.zero) + amt;
      }
    }
    final commodity = totals.isEmpty
        ? 'CNY'
        : totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;

    for (var i = 11; i >= 0; i--) {
      final m = DateTime(now.year, now.month - i);
      final prefix = '${m.year}-${m.month.toString().padLeft(2, '0')}';
      var inc = Decimal.zero;
      var exp = Decimal.zero;
      for (final t in ledger.txns) {
        if (!t.date.startsWith(prefix)) continue;
        for (final p in t.postings) {
          final acc = accByUuid[p.accountUuid];
          if (acc == null) continue;
          if (p.commodity != commodity) continue;
          // 金额大小展示，与账本符号无关（收入/支出按月取大小）。
          final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
          if (acc.type == 'Income') {
            inc += amt;
          } else if (acc.type == 'Expenses') {
            exp += amt;
          }
        }
      }
      points.add(TrendPoint(label: '${m.month}月', income: inc, expense: exp));
    }

    return AppCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('近 12 月收支趋势', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
              const SizedBox(width: 16),
              _legendDot(AppColors.green, '收入'),
              const SizedBox(width: 12),
              _legendDot(AppColors.warn, '支出'),
              const Spacer(),
              Text('（$commodity）', style: const TextStyle(fontSize: 11, color: AppColors.sub)),
            ],
          ),
          const SizedBox(height: 8),
          TrendLineChart(points: points, commodity: commodity, height: wide ? 200 : 160),
        ],
      ),
    );
  }

  Widget _legendDot(Color c, String label) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(width: 8, height: 8, decoration: BoxDecoration(color: c, shape: BoxShape.circle)),
          const SizedBox(width: 4),
          Text(label, style: const TextStyle(fontSize: 11, color: AppColors.sub)),
        ],
      );
}

class _NetWorthCard extends StatelessWidget {
  final Map<String, String> netWorth;
  /// 隐私开关：true 时金额遮挡（与头部 / 列表一致）。
  final bool hidden;
  const _NetWorthCard({required this.netWorth, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    final entries = netWorth.entries.toList();
    return AppCard(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('净资产（资产 + 负债，负债为负）', style: AppTheme.muted),
          const SizedBox(height: 6),
          if (hidden)
            const Text('••••',
                style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800, letterSpacing: -0.5))
          else if (entries.isEmpty)
            const Text('¥ 0.00',
                style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800, letterSpacing: -0.5))
          else
            ...entries.map((e) => Padding(
                  padding: const EdgeInsets.only(bottom: 2),
                  child: Text(
                    formatMoney(e.value, e.key, signed: false),
                    style: const TextStyle(
                      fontSize: 34,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.5,
                    ),
                  ),
                )),
          const SizedBox(height: 4),
          const Text('数据来自本地账本，联网后自动同步', style: AppTheme.muted),
        ],
      ),
    );
  }
}

class _RecentTxns extends StatelessWidget {
  final LedgerController ledger;
  final bool hidden; // 首页隐私开关：true 时金额蒙层
  const _RecentTxns({required this.ledger, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    final recent = ledger.txns.take(5).toList();
    if (recent.isEmpty) {
      return const AppCard(child: EmptyState('还没有交易，点「记一笔」开始记账'));
    }
    return AppCard(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          for (var i = 0; i < recent.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: AppColors.line),
            _TxnTile(ledger: ledger, txn: recent[i], hidden: hidden),
          ],
        ],
      ),
    );
  }
}

class _TxnTile extends StatelessWidget {
  final LedgerController ledger;
  final LocalTxn txn;
  final bool hidden;
  const _TxnTile({required this.ledger, required this.txn, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    final head = ledger.txnHeadline(txn);
    final headCommodity = txn.postings.isNotEmpty ? txn.postings.first.commodity : 'CNY';
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Text(txn.description.isEmpty ? '(无说明)' : txn.description,
          style: const TextStyle(fontWeight: FontWeight.w500)),
      subtitle: Text(
        '${txn.date} · ${ledger.txnFlow(txn)}${txn.createdByName != null && txn.createdByName!.isNotEmpty ? ' · ${txn.createdByName}' : ''}',
        style: AppTheme.muted,
      ),
      trailing: head != null ? AmountText(head, headCommodity, hidden: hidden) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}

/// 账单首页（F-4 改版）：HeroHeader 渐变汇总 + 日历/列表两视图 + 支出排行 + 最近交易。
/// 批 4 / F-10：宽屏复用同一套卡片排成两/三栏，不再维护独立 PC 仪表盘。
///
/// 能力迁移核对（重构前盘点，防丢功能）：
/// - 隐私眼睛开关（Privacy 持久化 + AmountText 蒙层）→ 移至头部右上
/// - 当月收支结余（主币种口径，避免跨币种直加）→ SummaryRow
/// - 月历（每日净值、点击跳当日流水）→ 日历 Tab（CalendarView 受控月份，与汇总联动）
/// - 最近交易（点击进详情）→ 保留
/// - 下拉刷新同步 → 保留
/// - 全量流水入口 → 头部右上 📋（落地后已移除 mobile_shell AppBar 的过渡期入口）
/// - 月份切换（原型 ‹ 2026年11月 ›）→ 新增，联动汇总/日历/列表/排行
/// - 支出排行（二级分类 TopN + 占比条）→ 新增
/// - 净资产/四宫格指标/近 12 月趋势（原 PC 仪表盘）→ 宽屏右栏
/// - 资产/负债/权益分组 → 收敛到「资产」页（PC 侧边栏直达，避免两处维护）
class _BillHome extends StatefulWidget {
  const _BillHome();

  @override
  State<_BillHome> createState() => _BillHomeState();
}

class _BillHomeState extends State<_BillHome> {
  bool _hide = false;
  bool _listMode = false; // false = 日历视图，true = 列表视图
  DateTime _month = DateTime.now();

  @override
  void initState() {
    super.initState();
    _hide = Privacy.load();
  }

  void _toggleHide() {
    setState(() => _hide = !_hide);
    Privacy.save(_hide);
  }

  void _shiftMonth(int months) {
    setState(() => _month = DateTime(_month.year, _month.month + months));
  }

  String get _monthPrefix =>
      '${_month.year}-${_month.month.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context) {
    final scope = AppScope.of(context);
    final ledger = scope.ledger;
    final auth = scope.auth;
    // 账本 pill 需要 auth 的当前账本名，切换后随之刷新。
    return ListenableBuilder(
      listenable: Listenable.merge([ledger, auth]),
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        final accByUuid = {
          for (final a in ledger.accounts) a.uuid: a,
        };
        // 选交易总额最大的币种作为展示口径（避免跨币种直接相加）。
        final totals = <String, Decimal>{};
        // 金额大小展示，与账本符号无关（选展示币种用）。
        for (final t in ledger.txns) {
          for (final p in t.postings) {
            totals[p.commodity] = (totals[p.commodity] ?? Decimal.zero) +
                (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
          }
        }
        final commodity = totals.isEmpty
            ? 'CNY'
            : totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;

        // 所选月份的收支结余 + 当月交易（按日倒序）+ 支出排行（二级分类）。
        var income = Decimal.zero;
        var expense = Decimal.zero;
        final monthTxns = <LocalTxn>[];
        final catTotals = <String, Decimal>{}; // key = 'Expenses:二级'
        for (final t in ledger.txns) {
          if (!t.date.startsWith(_monthPrefix)) continue;
          monthTxns.add(t);
          for (final p in t.postings) {
            if (p.commodity != commodity) continue;
            final acc = accByUuid[p.accountUuid];
            if (acc == null) continue;
            // 金额大小展示，与账本符号无关（当月收入/支出取大小）。
            final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
            if (acc.type == 'Income') {
              income += amt;
            } else if (acc.type == 'Expenses') {
              expense += amt;
              // 归并到二级分类（Expenses:X:Y 的分录记到 Expenses:X）；
              // 直接记在二级上的分录自然落位，三级及更深向上归并。
              final parts = acc.name.split(':');
              if (parts.length >= 2) {
                final key = parts.take(2).join(':');
                catTotals[key] = (catTotals[key] ?? Decimal.zero) + amt;
              }
            }
          }
        }
        final balance = income - expense;
        monthTxns.sort((a, b) => b.date.compareTo(a.date));

        final expenseStr =
            _hide ? '••••' : formatMoney(expense.toStringAsFixed(2), commodity, signed: false);
        final incomeStr =
            _hide ? '••••' : formatMoney(income.toStringAsFixed(2), commodity, signed: false);
        final balanceStr =
            _hide ? '••••' : formatMoney(balance.toStringAsFixed(2), commodity, signed: false);

        final wide = isWide(context);
        // F-10：≥1200 三栏（日历 / 汇总指标 / 快捷+最近），1024~1199 两栏，更窄纵向堆叠。
        final threeCol = MediaQuery.sizeOf(context).width >= 1200;

        final hero = HeroHeader(
          leading: _MonthSwitcher(
            month: _month,
            onShift: _shiftMonth,
            hidden: _hide,
          ),
          // 多账本全局化：四页头部统一账本 pill（F-4）。
          pill: LedgerPill(
            ledgerName: auth.currentLedger?.name ?? '账本',
            onTap: () => showLedgerSwitcher(context),
          ),
          actions: [
            HeroAction(
              icon: _hide
                  ? Icons.visibility_off_outlined
                  : Icons.visibility_outlined,
              tooltip: _hide ? '显示金额' : '隐藏金额',
              onTap: _toggleHide,
            ),
            HeroAction(
              icon: Icons.receipt_long_outlined,
              tooltip: '全部流水',
              onTap: () => Navigator.push(
                context,
                MaterialPageRoute(builder: (_) => const TransactionsScreen()),
              ),
            ),
          ],
          summary: SummaryRow(
            expense: expenseStr,
            income: incomeStr,
            balance: balanceStr,
            balancePositive: balance >= Decimal.zero,
            expenseLabel: '${_month.month}月支出',
            incomeLabel: '${_month.month}月收入',
          ),
          bottomGap: wide ? 0 : 34,
        );

        final calendarCard = UpliftCard(
          margin: wide ? EdgeInsets.zero : const EdgeInsets.fromLTRB(16, 0, 16, 0),
          child: Column(
            children: [
              _ViewSeg(listMode: _listMode, onChanged: (v) => setState(() => _listMode = v)),
              const SizedBox(height: 12),
              if (!_listMode)
                CalendarView(
                  txns: ledger.txns,
                  month: _month,
                  // 供日历区分「收支」与「账户间转账」，否则每日结余恒为 0。
                  accountTypes: {for (final a in ledger.accounts) a.uuid: a.type},
                  hidden: _hide,
                  onMonthChanged: (m) => setState(() => _month = m),
                  onSelectDate: (date) => Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) => TransactionsScreen(filterDate: date),
                    ),
                  ),
                )
              else
                _MonthList(
                  ledger: ledger,
                  txns: monthTxns,
                  commodity: commodity,
                  hidden: _hide,
                ),
            ],
          ),
        );

        final rankingCard = _ExpenseRanking(
          catTotals: catTotals,
          accounts: ledger.accounts,
          total: expense,
          commodity: commodity,
          hidden: _hide,
          flush: wide,
        );

        final recentCard = Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!wide)
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 16),
                child: SectionTitle('最近交易'),
              )
            else
              const SectionTitle('最近交易'),
            _RecentTxns(ledger: ledger, hidden: _hide),
          ],
        );

        // 四宫格指标 + 近 12 月趋势：原 PC 仪表盘的能力，收进宽屏右栏，功能不丢。
        final metricsCard = AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SectionTitle('资产概览'),
              const SizedBox(height: 10),
              GridView.count(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                crossAxisCount: 2,
                crossAxisSpacing: 8,
                mainAxisSpacing: 8,
                childAspectRatio: 1.75,
                children: [
                  _MetricChip(
                      label: '总资产', value: _moneyMap(ledger.byType['Assets'] ?? {}), color: AppColors.blue),
                  _MetricChip(
                      label: '总负债',
                      // 总负债按展示惯例取正数幅度；负债余额本身为负，显式 abs。
                      value: _moneyMap(ledger.byType['Liabilities'] ?? {},
                          magnitude: true),
                      color: AppColors.warn),
                  _MetricChip(
                      label: '收入',
                      value: _moneyMap(ledger.byType['Income'] ?? {}),
                      color: AppColors.green),
                  _MetricChip(
                      label: '支出',
                      value: _moneyMap(ledger.byType['Expenses'] ?? {}),
                      color: AppColors.expense),
                ],
              ),
            ],
          ),
        );

        final quickCard = AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SectionTitle('快捷记账'),
              const SizedBox(height: 10),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  style: FilledButton.styleFrom(
                    backgroundColor: AppColors.blue,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12)),
                  ),
                  onPressed: () => showDialog(
                    context: context,
                    builder: (_) => const RecordDialog(),
                  ),
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('记一笔'),
                ),
              ),
              const SizedBox(height: 8),
              const Text('支出 / 收入 / 转账 三 Tab 在弹层内切换',
                  style: TextStyle(fontSize: 11, color: AppColors.sub)),
            ],
          ),
        );

        final trendCard = _MiniTrendCard(ledger: ledger, wide: true);

        // 恒等式告警横幅：服务端/本地检查不平衡或净值双算不一致时展示；
        // 仅本次刷新内可关闭，下次 syncNow/init 重现。
        final identityBanner = ledger.hasIdentityAlert
            ? IdentityBanner(
                result: ledger.identityResult,
                warning: ledger.identityWarning,
                onDismiss: ledger.dismissIdentityBanner,
              )
            : null;

        Widget body;
        if (!wide) {
          body = ListView(
            padding: const EdgeInsets.only(bottom: 24),
            children: [
              hero,
              if (identityBanner != null) identityBanner,
              const SizedBox(height: 12),
              calendarCard,
              const SizedBox(height: 16),
              rankingCard,
              const SizedBox(height: 16),
              recentCard,
            ],
          );
        } else {
          final left = Column(
            children: [
              calendarCard,
              const SizedBox(height: 16),
              rankingCard,
            ],
          );
          final mid = Column(
            children: [
              _NetWorthCard(netWorth: ledger.netWorth, hidden: _hide),
              const SizedBox(height: 16),
              metricsCard,
              const SizedBox(height: 16),
              trendCard,
            ],
          );
          final right = Column(
            children: [
              quickCard,
              const SizedBox(height: 16),
              recentCard,
            ],
          );
          body = SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            child: Column(
              children: [
                hero,
                if (identityBanner != null)
                  Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 1440),
                      child: identityBanner,
                    ),
                  ),
                Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 1440),
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(20, 20, 20, 28),
                      child: threeCol
                          ? Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(flex: 5, child: left),
                                const SizedBox(width: 16),
                                Expanded(flex: 3, child: mid),
                                const SizedBox(width: 16),
                                Expanded(flex: 3, child: right),
                              ],
                            )
                          : Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(flex: 5, child: left),
                                const SizedBox(width: 16),
                                Expanded(
                                  flex: 5,
                                  child: Column(
                                    children: [
                                      _NetWorthCard(
                                          netWorth: ledger.netWorth, hidden: _hide),
                                      const SizedBox(height: 16),
                                      metricsCard,
                                      const SizedBox(height: 16),
                                      quickCard,
                                      const SizedBox(height: 16),
                                      recentCard,
                                    ],
                                  ),
                                ),
                              ],
                            ),
                    ),
                  ),
                ),
              ],
            ),
          );
        }

        return RefreshIndicator(
          onRefresh: () => ledger.syncNow().then((_) {}),
          child: body,
        );
      },
    );
  }

  /// 多币种余额拼接；隐私开启时统一遮挡。
  /// [magnitude] 为 true 时按金额大小展示（显式 abs），用于「总负债」这类正数幅度口径。
  String _moneyMap(Map<String, String> m, {bool magnitude = false}) {
    if (_hide) return '••••';
    if (m.isEmpty) return '—';
    if (!magnitude) {
      return m.entries.map((e) => formatMoney(e.value, e.key)).join('  ');
    }
    return m.entries
        .map((e) => formatMoney(
            (Decimal.tryParse(e.value) ?? Decimal.zero).abs().toString(), e.key))
        .join('  ');
  }
}

/// 头部月份切换器：‹ 2026年11月 ›（白色，渐变头上）。
class _MonthSwitcher extends StatelessWidget {
  final DateTime month;
  final void Function(int months) onShift;
  final bool hidden;
  const _MonthSwitcher({required this.month, required this.onShift, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _iconBtn(Icons.chevron_left, '上一月', () => onShift(-1)),
        Expanded(
          child: Text(
            '${month.year}年${month.month}月',
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 17,
              fontWeight: FontWeight.w700,
              color: Colors.white,
            ),
          ),
        ),
        _iconBtn(Icons.chevron_right, '下一月', () => onShift(1)),
      ],
    );
  }

  Widget _iconBtn(IconData icon, String tip, VoidCallback onTap) => IconButton(
        visualDensity: VisualDensity.compact,
        icon: Icon(icon, color: Colors.white.withValues(alpha: .85), size: 22),
        tooltip: tip,
        onPressed: onTap,
      );
}

/// 日历/列表两段切换（原型 seg，紧凑版内嵌卡顶）。
class _ViewSeg extends StatelessWidget {
  final bool listMode;
  final void Function(bool) onChanged;
  const _ViewSeg({required this.listMode, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    Widget seg(String label, bool value) {
      final on = listMode == value;
      return Expanded(
        child: GestureDetector(
          onTap: () => onChanged(value),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 7),
            decoration: BoxDecoration(
              color: on ? AppColors.blue : Colors.transparent,
              borderRadius: BorderRadius.circular(9),
            ),
            alignment: Alignment.center,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: on ? FontWeight.w700 : FontWeight.w500,
                color: on ? Colors.white : AppColors.sub,
              ),
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(3),
      decoration: BoxDecoration(
        color: AppColors.bg,
        borderRadius: BorderRadius.circular(11),
      ),
      child: Row(children: [seg('日历', false), seg('列表', true)]),
    );
  }
}

/// 列表视图：当月交易按日分组（当日合计带符号染色）。
class _MonthList extends StatelessWidget {
  final LedgerController ledger;
  final List<LocalTxn> txns;
  final String commodity;
  final bool hidden;
  const _MonthList({
    required this.ledger,
    required this.txns,
    required this.commodity,
    this.hidden = false,
  });

  @override
  Widget build(BuildContext context) {
    if (txns.isEmpty) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 28),
        child: EmptyState('这个月还没有交易'),
      );
    }
    // 按日分组（txns 已按日期倒序）。
    final groups = <String, List<LocalTxn>>{};
    for (final t in txns) {
      groups.putIfAbsent(t.date, () => []).add(t);
    }
    final accByUuid = {for (final a in ledger.accounts) a.uuid: a};
    return Column(
      children: [
        for (final entry in groups.entries) ...[
          Row(
            children: [
              Text('${entry.key.substring(5)} 日',
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700)),
              const SizedBox(width: 8),
              Expanded(
                child: Divider(height: 1, color: AppColors.line.withValues(alpha: .7)),
              ),
              const SizedBox(width: 8),
              Builder(builder: (ctx) {
                // 当日结余 = 收入 − 支出。收入分录是贷方（负数）、支出是借方（正数），
                // 直接相加会把符号弄反，必须按类型取正/取负。
                var net = Decimal.zero;
                for (final t in entry.value) {
                  for (final p in t.postings) {
                    if (p.commodity != commodity) continue;
                    final acc = accByUuid[p.accountUuid];
                    if (acc == null) continue;
                    final v = Decimal.tryParse(p.amount) ?? Decimal.zero;
                    if (acc.type == 'Income') {
                      // 金额大小展示，与账本符号无关（收入按大小记为正）。
                      net += v.abs();
                    } else if (acc.type == 'Expenses') {
                      // 金额大小展示，与账本符号无关（支出按大小记为负）。
                      net -= v.abs();
                    }
                  }
                }
                final pos = net >= Decimal.zero;
                return Text(
                  hidden
                      ? '••••'
                      : formatMoney(net.toStringAsFixed(2), commodity, signed: pos),
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    color: hidden ? AppColors.sub : (pos ? AppColors.green : AppColors.warn),
                    fontFeatures: const [FontFeature.tabularFigures()],
                  ),
                );
              }),
            ],
          ),
          const SizedBox(height: 2),
          for (final t in entry.value)
            _TxnTile(ledger: ledger, txn: t, hidden: hidden),
          const SizedBox(height: 6),
        ],
      ],
    );
  }
}

/// 支出排行（二级分类 Top5 + 占比条），隐藏金额时只显示结构与占比。
class _ExpenseRanking extends StatelessWidget {
  final Map<String, Decimal> catTotals;
  final List<LocalAccount> accounts;
  final Decimal total;
  final String commodity;
  final bool hidden;
  /// [flush]=true 时不加左右外边距（宽屏分栏里由栏间距负责留白）。
  final bool flush;
  const _ExpenseRanking({
    required this.catTotals,
    required this.accounts,
    required this.total,
    required this.commodity,
    this.hidden = false,
    this.flush = false,
  });

  @override
  Widget build(BuildContext context) {
    if (catTotals.isEmpty) return const SizedBox.shrink();
    final ranked = catTotals.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    final top = ranked.take(5).toList();

    // 找二级分类账户本身（或其子账户）取 icon/color 展示。
    LocalAccount? findAcc(String key) {
      for (final a in accounts) {
        if (a.name == key) return a;
      }
      for (final a in accounts) {
        if (a.name.startsWith('$key:')) return a;
      }
      return null;
    }

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: flush ? 0 : 16),
      child: AppCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Text('支出排行',
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
                const Spacer(),
                Text('Top ${top.length} · $commodity',
                    style: const TextStyle(fontSize: 11, color: AppColors.sub)),
              ],
            ),
            const SizedBox(height: 12),
            for (final e in top) ...[
              Builder(builder: (ctx) {
                final acc = findAcc(e.key);
                // Decimal 的 `/` 在 dart:math 语境下推断不稳，改用 double 计算占比。
                final pct = total.toDouble() > 0
                    ? e.value.toDouble() / total.toDouble()
                    : 0.0;
                return Row(
                  children: [
                    CategoryIcon(
                      emoji: (acc?.icon?.isNotEmpty == true) ? acc!.icon! : '🏷️',
                      color: AppColors.softOf(e.key, hexColor: acc?.color),
                      size: 34,
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Expanded(
                                child: Text(
                                  accountDisplayName(acc ?? LocalAccount(uuid: e.key, name: e.key, type: '', openDate: '')),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                                ),
                              ),
                              Text(
                                hidden ? '••••' : formatMoney(e.value.toStringAsFixed(2), commodity, signed: false),
                                style: const TextStyle(
                                  fontSize: 13,
                                  fontWeight: FontWeight.w700,
                                  fontFeatures: [FontFeature.tabularFigures()],
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 5),
                          ClipRRect(
                            borderRadius: BorderRadius.circular(3),
                            child: LinearProgressIndicator(
                              value: pct.clamp(0.0, 1.0),
                              minHeight: 5,
                              backgroundColor: AppColors.line.withValues(alpha: .5),
                              valueColor: AlwaysStoppedAnimation(AppColors.blue),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 10),
                    Text(
                      '${(pct * 100).toStringAsFixed(0)}%',
                      style: const TextStyle(
                        fontSize: 11,
                        color: AppColors.sub,
                        fontFeatures: [FontFeature.tabularFigures()],
                      ),
                    ),
                  ],
                );
              }),
              const SizedBox(height: 12),
            ],
          ],
        ),
      ),
    );
  }
}
