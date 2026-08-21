/// 分类管理页（设计稿「分类」）：分类本质就是 Expenses / Income 类型账户。
/// 展示二级分类（parentUuid 为空）及其三级子分类（parentUuid 指向二级）。
/// 新增子分类复用 AccountDialog（选父分类即可生成三级账户）。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/account_icon.dart';
import '../../widgets/common.dart';
import '../accounts/account_dialog.dart';

class CategoriesScreen extends StatelessWidget {
  const CategoriesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.accounts.isEmpty) {
          return const EmptyState('还没有分类，点右下角 + 添加');
        }
        return ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          children: [
            _TypeBlock(ledger: ledger, type: 'Expenses', label: '支出分类'),
            const SizedBox(height: 8),
            _TypeBlock(ledger: ledger, type: 'Income', label: '收入分类'),
          ],
        );
      },
    );
  }
}

class _TypeBlock extends StatelessWidget {
  final LedgerController ledger;
  final String type;
  final String label;
  const _TypeBlock({required this.ledger, required this.type, required this.label});

  List<LocalAccount> get _roots => ledger.accounts
      .where((a) => a.type == type && a.parentUuid == null)
      .toList();
  List<LocalAccount> _children(LocalAccount root) =>
      ledger.accounts.where((a) => a.parentUuid == root.uuid).toList();

  @override
  Widget build(BuildContext context) {
    final roots = _sortedRoots;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            SectionTitle(label),
            const Spacer(),
            TextButton.icon(
              onPressed: () => showDialog(
                context: context,
                builder: (_) => AccountDialog(defaultType: type),
              ),
              icon: const Icon(Icons.add, size: 18),
              label: const Text('添加'),
            ),
          ],
        ),
        if (roots.isEmpty)
          const Padding(
            padding: EdgeInsets.only(bottom: 8),
            child: Text('暂无分类', style: AppTheme.muted),
          ),
        if (roots.isNotEmpty)
          ReorderableListView(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            buildDefaultDragHandles: false,
            onReorder: (oldIndex, newIndex) => _onReorder(roots, oldIndex, newIndex),
            children: [
              for (var i = 0; i < roots.length; i++)
                AppCard(
                  key: ValueKey(roots[i].uuid),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          ReorderableDragStartListener(
                            index: i,
                            child: const Padding(
                              padding: EdgeInsets.only(right: 4),
                              child: Icon(Icons.drag_handle,
                                  size: 20, color: AppColors.sub),
                            ),
                          ),
                          AccountIcon(roots[i], size: 34),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(roots[i].display,
                                style: const TextStyle(
                                    fontSize: 15, fontWeight: FontWeight.w700)),
                          ),
                          TextButton(
                            onPressed: () => showDialog(
                              context: context,
                              builder: (_) => AccountDialog(defaultType: type),
                            ),
                            child: const Text('加子分类'),
                          ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: _children(roots[i])
                            .map((c) => Chip(
                                  avatar: AccountIcon(c, size: 22),
                                  label: Text(c.display),
                                  visualDensity: VisualDensity.compact,
                                ))
                            .toList(),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        const SizedBox(height: 12),
      ],
    );
  }

  /// 根分类按 sort_order 升序（未设置则按 name），保证拖拽顺序展示与持久化一致。
  List<LocalAccount> get _sortedRoots => _roots
    ..sort((a, b) {
      if (a.sortOrder != b.sortOrder) return a.sortOrder.compareTo(b.sortOrder);
      return a.name.compareTo(b.name);
    });

  /// 拖拽重排：先把视觉列表按 onReorder 规则调整，再调用 controller 持久化新顺序。
  void _onReorder(
      List<LocalAccount> roots, int oldIndex, int newIndex) async {
    final list = List<LocalAccount>.from(roots);
    if (oldIndex < newIndex) newIndex -= 1;
    final moved = list.removeAt(oldIndex);
    list.insert(newIndex, moved);
    await ledger.reorderAccounts(list);
  }
}
