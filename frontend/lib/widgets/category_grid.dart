/// 分类九宫格：前 9 个分类 + 「更多」格；选中项即使被裁剪也会显示，保证反馈可见。
/// 「更多」弹出完整分类底部列表。
import 'package:flutter/material.dart';
import '../core/models/account.dart';
import 'account_icon.dart';

class CategoryGrid extends StatelessWidget {
  final List<LocalAccount> cats;
  final String? selectedUuid;
  final ValueChanged<LocalAccount> onSelect;
  const CategoryGrid({
    super.key,
    required this.cats,
    required this.selectedUuid,
    required this.onSelect,
  });

  /// 实际展示的分类（≤9）：前 9 个；选中项不在其中时占用第 9 位。
  List<LocalAccount> get shown {
    final head = cats.take(9).toList();
    if (selectedUuid != null && !head.any((a) => a.uuid == selectedUuid)) {
      final sel = cats.firstWhere((a) => a.uuid == selectedUuid);
      if (head.length == 9) {
        head[8] = sel;
      } else {
        head.add(sel);
      }
    }
    return head;
  }

  Future<void> _more(BuildContext context) async {
    final picked = await showModalBottomSheet<LocalAccount>(
      context: context,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (_) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          padding: const EdgeInsets.symmetric(vertical: 10),
          children: [
            const Padding(
              padding: EdgeInsets.fromLTRB(18, 8, 18, 8),
              child: Text('全部分类',
                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
            ),
            for (final a in cats)
              ListTile(
                leading: AccountIcon(a, size: 34),
                title: Text(_short(a.display),
                    style: const TextStyle(fontSize: 14)),
                subtitle: a.display.contains(':')
                    ? Text(a.display,
                        style: const TextStyle(
                            fontSize: 11, color: Color(0xFF9AA0AC)))
                    : null,
                trailing: a.uuid == selectedUuid
                    ? const Icon(Icons.check, color: Color(0xFF3B6EF6), size: 18)
                    : null,
                onTap: () => Navigator.pop(context, a),
              ),
          ],
        ),
      ),
    );
    if (picked != null) onSelect(picked);
  }

  @override
  Widget build(BuildContext context) {
    final cells = <Widget>[
      for (final a in shown)
        _CatCell(a: a, on: a.uuid == selectedUuid, onTap: () => onSelect(a)),
    ];
    while (cells.length < 9) {
      cells.add(const SizedBox());
    }
    cells.add(_MoreCell(onTap: () => _more(context)));

    return GridView.count(
      crossAxisCount: 5,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      childAspectRatio: .8,
      padding: EdgeInsets.zero,
      mainAxisSpacing: 6,
      children: cells,
    );
  }
}

String _short(String display) =>
    display.contains(':') ? display.split(':').last : display;

class _CatCell extends StatelessWidget {
  final LocalAccount a;
  final bool on;
  final VoidCallback onTap;
  const _CatCell({required this.a, required this.on, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          AccountIcon(a, size: 44),
          const SizedBox(height: 4),
          Text(_short(a.display),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                  fontSize: 11,
                  color: on ? const Color(0xFF3B6EF6) : const Color(0xFF6B7280),
                  fontWeight: on ? FontWeight.w600 : FontWeight.w400)),
        ],
      ),
    );
  }
}

class _MoreCell extends StatelessWidget {
  final VoidCallback onTap;
  const _MoreCell({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: const Color(0xFFF4F5F8),
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: const Color(0xFFC9CDD8), width: 1),
            ),
            alignment: Alignment.center,
            child: const Text('···',
                style: TextStyle(fontSize: 16, color: Color(0xFF6B7280))),
          ),
          const SizedBox(height: 4),
          const Text('更多',
              style: TextStyle(fontSize: 11, color: Color(0xFF6B7280))),
        ],
      ),
    );
  }
}
