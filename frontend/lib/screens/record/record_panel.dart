/// 记一笔 v2 共享面板（F-3）：支出 / 收入 / 转账三 Tab + 分类九宫格 +
/// 数字键盘 + 分录预览 + 资金账户描边强调。PC 弹窗与移动底部弹层共用本面板。
///
/// 逻辑层沿用原 RecordDialog（三模式 posting 构造、默认账户回写、离线提交），
/// **不触碰 sync 协议**：仍经 LedgerController.createTransaction 走在线/离线双路径。
/// 「高级模式」跳转 RecordScreen（双录分录编辑，熟悉 Beancount 的用户用）。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/posting.dart';
import '../../widgets/account_icon.dart';
import '../../widgets/category_grid.dart';
import '../../widgets/common.dart';
import '../../widgets/num_keyboard.dart';
import 'record_screen.dart';

class RecordV2Panel extends StatefulWidget {
  final VoidCallback? onClosed;
  const RecordV2Panel({super.key, this.onClosed});

  @override
  State<RecordV2Panel> createState() => _RecordV2PanelState();
}

class _RecordV2PanelState extends State<RecordV2Panel> {
  int _mode = 0; // 0 支出 / 1 收入 / 2 转账
  final _amountCtl = TextEditingController();
  final _descCtl = TextEditingController();
  final _tagsCtl = TextEditingController();
  DateTime _date = DateTime.now();
  final String _commodity = 'CNY';
  String? _categoryUuid;
  String? _assetUuid;
  String? _toUuid;
  bool _busy = false;
  Map<String, String> _defs = {};
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _loadDefaults();
  }

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

  List<LocalPosting> _buildPostings(Decimal amount) {
    final amt = amount.toString();
    if (_mode == 2) {
      return [
        LocalPosting(accountUuid: _assetUuid!, commodity: _commodity, amount: (-amount).toString()),
        LocalPosting(accountUuid: _toUuid!, commodity: _commodity, amount: amt),
      ];
    } else if (_mode == 0) {
      return [
        LocalPosting(accountUuid: _assetUuid!, commodity: _commodity, amount: (-amount).toString()),
        LocalPosting(accountUuid: _categoryUuid!, commodity: _commodity, amount: amt),
      ];
    } else {
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
    if (_assetUuid == null) return '请选择资金账户';
    if (_mode == 2 && _toUuid == null) return '请选择转入账户';
    if (_mode == 2 && _toUuid == _assetUuid) return '转出与转入账户不能相同';
    return null;
  }

  Future<void> _submit({required bool keepOpen}) async {
    final err = _validate();
    if (err != null) {
      showAppSnack(context, err, warn: true);
      return;
    }
    final amount = Decimal.parse(_amountCtl.text.trim());
    final postings = _buildPostings(amount);
    final ledger = AppScope.of(context).ledger;
    try {
      if (_mode == 0 && _categoryUuid != null) {
        await ledger.setDefaultAccount('expense', _categoryUuid!);
      } else if (_mode == 1 && _categoryUuid != null) {
        await ledger.setDefaultAccount('income', _categoryUuid!);
      } else if (_mode == 2) {
        if (_assetUuid != null) await ledger.setDefaultAccount('transferOut', _assetUuid!);
        if (_toUuid != null) await ledger.setDefaultAccount('transferIn', _toUuid!);
      }
    } catch (_) {}

    setState(() => _busy = true);
    try {
      await ledger.createTransaction(
        date: todayStr(_date),
        description: _descCtl.text.trim(),
        postings: postings,
        tags: _tags,
      );
      if (!mounted) return;
      if (keepOpen) {
        showAppSnack(context, '已记一笔，继续', warn: false);
        setState(() {
          _amountCtl.clear();
          _descCtl.clear();
          _tagsCtl.clear();
          _busy = false;
        });
      } else {
        widget.onClosed?.call();
        Navigator.pop(context, true);
      }
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '提交失败: $e', warn: true);
    } finally {
      if (mounted && _busy) setState(() => _busy = false);
    }
  }

  List<String> get _tags => _tagsCtl.text
      .split(RegExp(r'[\s,，]+'))
      .map((e) => e.trim())
      .where((e) => e.isNotEmpty)
      .toList();

  String _shortName(String? uuid) {
    if (uuid == null) return '';
    for (final a in AppScope.of(context).ledger.accounts) {
      if (a.uuid == uuid) {
        final d = a.display;
        return d.contains(':') ? d.split(':').last : d;
      }
    }
    return '';
  }

  /// 分录预览：随分类 / 账户 / 金额实时刷新；信息不全时返回 null 不显示。
  String? _preview() {
    final amt = Decimal.tryParse(_amountCtl.text.trim());
    if (amt == null || amt <= Decimal.zero) return null;
    final a = amt.toString();
    if (_mode == 2) {
      if (_assetUuid == null || _toUuid == null) return null;
      return '${_shortName(_toUuid)} +$a ← ${_shortName(_assetUuid)} −$a';
    }
    if (_categoryUuid == null || _assetUuid == null) return null;
    if (_mode == 0) {
      return '${_shortName(_categoryUuid)} +$a ← ${_shortName(_assetUuid)} −$a';
    }
    return '${_shortName(_assetUuid)} +$a ← ${_shortName(_categoryUuid)} −$a';
  }

  Future<void> _pickDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
    );
    if (picked != null) setState(() => _date = picked);
  }

  void _goAdvanced() {
    final nav = Navigator.of(context, rootNavigator: true);
    Navigator.pop(context);
    nav.push(MaterialPageRoute(builder: (_) => const RecordScreen()));
  }

  void _onToken(String t) {
    final cur = _amountCtl.text;
    if (t == '.') {
      if (cur.contains('.')) return;
      _amountCtl.text = cur.isEmpty ? '0.' : '$cur.';
    } else {
      if (cur == '0' && t == '0') return;
      _amountCtl.text = cur == '0' ? t : cur + t;
    }
    setState(() {});
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
    final preview = _preview();

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // 模式 Tab + 高级入口
        Row(
          children: [
            for (var m = 0; m < 3; m++)
              Padding(
                padding: const EdgeInsets.only(right: 22),
                child: InkWell(
                  onTap: () => setState(() {
                    _mode = m;
                    _applyModeDefault();
                  }),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(['支出', '收入', '转账'][m],
                          style: TextStyle(
                              fontSize: 15,
                              color: _mode == m
                                  ? AppColors.blue
                                  : const Color(0xFF9AA0AC),
                              fontWeight:
                                  _mode == m ? FontWeight.w700 : FontWeight.w400)),
                      const SizedBox(height: 5),
                      Container(
                        height: 2.5,
                        width: 34,
                        color: _mode == m ? AppColors.blue : Colors.transparent,
                      ),
                    ],
                  ),
                ),
              ),
            const Spacer(),
            TextButton(
              onPressed: _goAdvanced,
              style: TextButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 6),
                  minimumSize: const Size(0, 32)),
              child: const Text('高级 ›',
                  style: TextStyle(fontSize: 13, color: AppColors.sub)),
            ),
          ],
        ),
        const SizedBox(height: 4),

        // 金额大屏
        TextField(
          controller: _amountCtl,
          keyboardType:
              const TextInputType.numberWithOptions(decimal: true, signed: false),
          textAlign: TextAlign.center,
          onChanged: (_) => setState(() {}),
          style: const TextStyle(fontSize: 36, fontWeight: FontWeight.w800),
          decoration: InputDecoration(
            prefixText: '¥ ',
            prefixStyle: const TextStyle(fontSize: 20, color: Color(0xFF9AA0AC)),
            hintText: '0.00',
            border: InputBorder.none,
            filled: false,
          ),
        ),

        // 转账：转出 / 转入
        if (_mode == 2) ...[
          _acctCard(_assetUuid, (v) => setState(() => _assetUuid = v),
              assetAccounts, '🏦 转出 · 选择账户', outlined: false),
          const Center(
              child: Padding(
            padding: EdgeInsets.symmetric(vertical: 4),
            child: Text('↓',
                style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: AppColors.blue)),
          )),
          _acctCard(_toUuid, (v) => setState(() => _toUuid = v), assetAccounts,
              '＋ 转入 · 选择账户',
              outlined: false, dashed: _toUuid == null),
        ]
        // 支出 / 收入：分类九宫格 + 资金账户描边
        else ...[
          CategoryGrid(
            cats: cats,
            selectedUuid: _categoryUuid,
            onSelect: (a) => setState(() => _categoryUuid = a.uuid),
          ),
          const SizedBox(height: 10),
          _acctCard(_assetUuid, (v) => setState(() => _assetUuid = v),
              assetAccounts, _mode == 0 ? '🏦 资金账户（钱从这里出）' : '🏦 存入账户（钱进这里）',
              outlined: true),
        ],

        const SizedBox(height: 10),
        Row(children: [
          Expanded(
            child: InkWell(
              onTap: _pickDate,
              borderRadius: BorderRadius.circular(10),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 11),
                decoration: BoxDecoration(
                    color: const Color(0xFFF4F5F8),
                    borderRadius: BorderRadius.circular(10)),
                child: Row(children: [
                  const Text('📅', style: TextStyle(fontSize: 13)),
                  const SizedBox(width: 6),
                  Text(todayStr(_date).substring(5),
                      style: const TextStyle(fontSize: 12, color: AppColors.ink)),
                ]),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: TextField(
              controller: _tagsCtl,
              style: const TextStyle(fontSize: 12),
              decoration: InputDecoration(
                hintText: '#标签（可选）',
                prefixStyle: const TextStyle(fontSize: 13),
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 10, vertical: 13),
                filled: true,
                fillColor: const Color(0xFFF4F5F8),
                border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                    borderSide: BorderSide.none),
              ),
            ),
          ),
        ]),
        const SizedBox(height: 8),
        TextField(
          controller: _descCtl,
          style: const TextStyle(fontSize: 13),
          decoration: InputDecoration(
            hintText: _mode == 2 ? '备注（可选，如「还信用卡」）' : '备注（可选，如「早餐 · 公司楼下」）',
            prefixStyle: const TextStyle(fontSize: 13),
            contentPadding:
                const EdgeInsets.symmetric(horizontal: 10, vertical: 13),
            filled: true,
            fillColor: const Color(0xFFF4F5F8),
            border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10), borderSide: BorderSide.none),
          ),
        ),

        // 分录预览
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 10),
          child: Text(
            preview == null
                ? '记账即自动配平（复式分录在保存时生成）'
                : '分录预览：${_mode == 2 ? '' : '${_mode == 0 ? 'Expenses' : 'Income'} · '}$preview',
            style: const TextStyle(fontSize: 11, color: Color(0xFF9AA0AC)),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),

        NumKeyboard(
          onToken: _onToken,
          onBackspace: () {
            final t = _amountCtl.text;
            if (t.isNotEmpty) _amountCtl.text = t.substring(0, t.length - 1);
            setState(() {});
          },
          onClear: () {
            _amountCtl.clear();
            setState(() {});
          },
          onAgain: () => _submit(keepOpen: true),
          onSave: () => _submit(keepOpen: false),
          saveEnabled: !_busy,
        ),
        const SizedBox(height: 6),
        Text(
          '本地保存${ledger.online ? '，已联网将自动同步' : '，离线待同步'}',
          style: AppTheme.muted,
          textAlign: TextAlign.center,
        ),
      ],
    );
  }

  /// 账户卡片式下拉。[outlined] 时蓝描边强调（资金账户必选）。
  Widget _acctCard(String? value, ValueChanged<String?> onChanged,
      List<LocalAccount> items, String hint,
      {required bool outlined, bool dashed = false}) {
    return Container(
      decoration: BoxDecoration(
        color: outlined ? const Color(0xFFE7EFFF) : const Color(0xFFF4F5F8),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: outlined ? AppColors.blue : const Color(0xFFE3E5EC),
          width: outlined ? 1.5 : 1,
        ),
      ),
      child: DropdownButtonFormField<String>(
        value: (value != null && items.any((a) => a.uuid == value)) ? value : null,
        isExpanded: true,
        icon: const Icon(Icons.expand_more, size: 18),
        items: items
            .map((a) => DropdownMenuItem(
                  value: a.uuid,
                  child: Row(children: [
                    AccountIcon(a, size: 26),
                    const SizedBox(width: 8),
                    Expanded(
                        child: Text(a.display, overflow: TextOverflow.ellipsis)),
                  ]),
                ))
            .toList(),
        onChanged: onChanged,
        decoration: InputDecoration(
          hintText: hint,
          hintStyle: TextStyle(
              fontSize: 13,
              color: dashed ? const Color(0xFFC6CAD4) : const Color(0xFF9AA0AC)),
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
          border: InputBorder.none,
          filled: false,
        ),
      ),
    );
  }
}
