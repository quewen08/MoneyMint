/// 新建/编辑账户弹窗（设计稿「新建账户」）。
///
/// 0.4 期初重构口径：
/// - 仅 Assets / Liabilities 可设置期初，金额非负、方向为「持有 / 欠款」；
/// - 新建账户的期初走 `POST /api/accounts/{uuid}/opening`（不再拼凑通用交易）；
/// - 编辑态只改期初，保存时后端自动冲正旧期初并按当前值重建；
/// - 期初仅在线可用，离线输入区置灰并提示「联网后可设置期初」。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/opening_models.dart';
import '../../core/models/account.dart';
import '../../utils/opening.dart';
import '../../viewmodels/ledger_controller.dart';
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
  final LocalAccount? account; // 传入则为编辑（仅期初可改）
  final String defaultType;
  const AccountDialog({super.key, this.account, this.defaultType = 'Assets'});

  @override
  State<AccountDialog> createState() => _AccountDialogState();
}

class _AccountDialogState extends State<AccountDialog> {
  final _nameCtl = TextEditingController();
  final _dispCtl = TextEditingController();
  final _openBalCtl = TextEditingController();

  String _type = 'Assets';
  String? _icon;
  String? _color;
  String? _parentUuid;
  bool _busy = false;

  // ---- 期初状态 ----
  String _direction = openingDirectionHold;
  String _initialDirection = openingDirectionHold;
  String _openingCommodity = '';
  bool _directionTouched = false;
  bool _openingLoading = false;
  String? _openingError;

  // ---- 日期 ----
  String _openDate = '';
  String _ledgerOpeningDate = '';
  String _ledgerCommodity = 'CNY';
  bool _contextLoaded = false;

  bool get _isEdit => widget.account != null;
  bool get _isClosed => widget.account?.isClosed ?? false;

  /// 是否可设置期初（类型 + 编辑态关闭检查）。
  bool get _openingAllowed =>
      canSetOpening(_type) && !(_isEdit && _isClosed);

  @override
  void initState() {
    super.initState();
    final a = widget.account;
    _type = a?.type ?? widget.defaultType;
    _direction = defaultOpeningDirection(_type);
    _initialDirection = _direction;
    _openDate = a?.openDate ?? '';
    _nameCtl.text = a?.name ?? '';
    _dispCtl.text = a?.displayName ?? '';
    _icon = a?.icon;
    _color = a?.color;
    _parentUuid = a?.parentUuid;
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_contextLoaded) return;
    _contextLoaded = true;
    final scope = AppScope.of(context);
    final ledger = scope.ledger;
    final current = scope.auth.currentLedger;
    final today = todayStr();
    final ledgerDate = current?.openingDate ?? '';
    final dc = current?.defaultCommodity ?? '';
    _ledgerCommodity = dc.isEmpty ? 'CNY' : dc;
    _ledgerOpeningDate = ledgerDate.isNotEmpty ? ledgerDate : today;
    if (_openDate.isEmpty) {
      _openDate =
          _ledgerOpeningDate.compareTo(today) <= 0 ? _ledgerOpeningDate : today;
    }
    if (ledger.online && _isEdit && _openingAllowed) {
      _loadOpening();
    }
  }

  @override
  void dispose() {
    _nameCtl.dispose();
    _dispCtl.dispose();
    _openBalCtl.dispose();
    super.dispose();
  }

  List<LocalAccount> get _parentCandidates {
    final ledger = AppScope.of(context).ledger;
    if (_type != 'Expenses' && _type != 'Income') return const [];
    return ledger.accounts
        .where((a) =>
            a.type == _type && a.parentUuid == null && a.uuid != widget.account?.uuid)
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

  String get _displayCommodity {
    if (_openingCommodity.isNotEmpty) return _openingCommodity;
    return widget.account?.restriction ?? _ledgerCommodity;
  }

  /// 编辑态读取当前期初（仅在线）。
  Future<void> _loadOpening() async {
    final acc = widget.account;
    if (acc == null) return;
    setState(() => _openingLoading = true);
    try {
      final view = await AppScope.of(context).ledger.loadOpening(acc.uuid);
      if (!mounted) return;
      setState(() {
        _openingLoading = false;
        _openingError = null;
        if (view.hasOpening) {
          _openBalCtl.text = openingAbsoluteFromSigned(view.signedAmount);
          _direction = view.direction;
          _initialDirection = view.direction;
          _openingCommodity = view.commodity;
        } else {
          _openBalCtl.text = '';
          _direction = defaultOpeningDirection(_type);
          _initialDirection = _direction;
          _openingCommodity = view.commodity;
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _openingLoading = false;
        _openingError = '读取期初失败：$e';
      });
    }
  }

  Future<bool> _confirmDirectionChange() async {
    if (_direction == _initialDirection) return true;
    final label = openingDirectionLabel(_direction);
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('确认期初方向'),
        content: Text('期初将按「$label」记录，与当前方向不同。确认继续吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('确认'),
          ),
        ],
      ),
    );
    return ok ?? false;
  }

  Future<void> _pickOpenDate() async {
    final today = todayStr();
    final latest = _ledgerOpeningDate.isEmpty ? today : _ledgerOpeningDate;
    final last = latest.compareTo(today) <= 0 ? latest : today;
    final first = DateTime.tryParse('2000-01-01') ?? DateTime(2000);
    final initial = DateTime.tryParse(_openDate) ?? DateTime.tryParse(last) ?? DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: initial.isAfter(DateTime.parse(last)) ? DateTime.parse(last) : initial,
      firstDate: first,
      lastDate: DateTime.parse(last),
      helpText: '开户日不得晚于账本期初日（$last）',
    );
    if (picked == null || !mounted) return;
    setState(() {
      _openDate = picked.toIso8601String().substring(0, 10);
    });
  }

  Future<void> _save() async {
    final ledger = AppScope.of(context).ledger;
    if (_busy) return;

    if (_isEdit) {
      await _saveEditOpening(ledger);
      return;
    }

    final name = _finalName();
    if (name.isEmpty) {
      showAppSnack(context, '请输入账户名（ASCII，如 Assets:Cash）', warn: true);
      return;
    }
    if (_parentUuid != null && !RegExp(r'^[A-Za-z0-9:_]+$').hasMatch(name)) {
      showAppSnack(context, '带分类的账户名只能含字母/数字/冒号', warn: true);
      return;
    }

    final rawAmount = _openBalCtl.text.trim();
    var wantsOpening = false;
    String amount = '0';
    if (_openingAllowed && rawAmount.isNotEmpty) {
      final normalized = normalizeOpeningAmount(rawAmount);
      if (normalized == null) {
        showAppSnack(context, '期初金额需为非负数字（如 1000.00）', warn: true);
        return;
      }
      amount = normalized;
      // 输入 0 视为不设置期初（新建账户无需冲正）。
      wantsOpening = normalized != '0';
      if (wantsOpening && !await _confirmDirectionChange()) return;
    }

    setState(() => _busy = true);
    try {
      final created = await ledger.createAccount(
        name: name,
        type: _type,
        openDate: _openDate.isEmpty ? todayStr() : _openDate,
        displayName: _dispCtl.text.trim().isEmpty ? null : _dispCtl.text.trim(),
        commodity: null,
        icon: _icon,
        color: _color,
        parentUuid: _parentUuid,
        subType: null,
      );

      var openingSet = false;
      if (wantsOpening) {
        if (created.uuid.startsWith('loc-')) {
          // 离线创建或账户推送失败：服务器还没有该 uuid，无法设置期初。
          if (!mounted) return;
          showAppSnack(context, '账户尚未同步到服务器，期初未设置；联网后可再编辑设置', warn: true);
        } else if (amount != '0') {
          try {
            final r = await ledger.saveOpening(
              created.uuid,
              amount: amount,
              direction: _direction,
              commodity: created.restriction ?? '',
            );
            openingSet = r.status == 'created' || r.status == 'updated';
          } catch (e) {
            // 账户已创建成功，期初失败不回滚账户，提示用户编辑重试。
            if (!mounted) return;
            showAppSnack(context, '账户已创建，期初设置失败：$e', warn: true);
            Navigator.pop(context, true);
            return;
          }
        }
      }
      if (!mounted) return;
      if (openingSet) {
        showAppSnack(context, '账户已创建，期初已设置');
      } else if (_openingAllowed && _openBalCtl.text.trim().isNotEmpty) {
        showAppSnack(context, '账户已创建（期初未生效）', warn: true);
      } else if (_openingAllowed && !ledger.online) {
        showAppSnack(context, '账户已创建；联网后可设置期初');
      } else {
        showAppSnack(context, '账户已创建');
      }
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '创建失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// 编辑态：仅提交期初（后端自动冲正旧期初并重建）。
  Future<void> _saveEditOpening(LedgerController ledger) async {
    final acc = widget.account!;
    if (_isClosed) {
      showAppSnack(context, '账户已关闭，期初不可修改', warn: true);
      return;
    }
    if (!ledger.online) {
      showAppSnack(context, '期初仅在线可修改，请联网后重试', warn: true);
      return;
    }
    if (_openingLoading || _openingError != null) {
      // 未读到当前期初时禁止保存，避免把「读取失败」误当成清空期初。
      showAppSnack(context, '期初尚未读取成功，请关闭后重试', warn: true);
      return;
    }
    if (acc.uuid.startsWith('loc-')) {
      showAppSnack(context, '账户尚未同步到服务器，暂不可设置期初', warn: true);
      return;
    }
    final rawAmount = _openBalCtl.text.trim();
    final amount = rawAmount.isEmpty ? '0' : normalizeOpeningAmount(rawAmount);
    if (amount == null) {
      showAppSnack(context, '期初金额需为非负数字（如 1000.00）', warn: true);
      return;
    }
    if (!await _confirmDirectionChange()) return;

    setState(() => _busy = true);
    try {
      final r = await ledger.saveOpening(
        acc.uuid,
        amount: amount,
        direction: _direction,
        commodity: acc.restriction ?? '',
      );
      if (!mounted) return;
      switch (r.status) {
        case 'updated':
          showAppSnack(context, '已重建期初交易');
          break;
        case 'cleared':
          showAppSnack(context, '期初已清空（冲正旧期初）');
          break;
        case 'unchanged':
          showAppSnack(context, '期初未变化');
          break;
        default:
          showAppSnack(context, '期初已设置');
      }
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '保存期初失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final parents = _parentCandidates;
    final scope = AppScope.of(context);
    final ledgerOnline = scope.ledger.online;
    final today = todayStr();
    final latestOpenDate = _ledgerOpeningDate.isEmpty ? today : _ledgerOpeningDate;

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
                Text(_isEdit ? '编辑账户（仅期初）' : '新建账户',
                    style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
                const SizedBox(height: 16),
                Row(
                  children: [
                    InkWell(
                      onTap: _isEdit ? null : _pickIcon,
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
                                      onTap: _isEdit
                                          ? null
                                          : () => setState(() => _color = c),
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
                  onChanged: _isEdit
                      ? null
                      : (v) => setState(() {
                            _type = v!;
                            if (!_directionTouched) {
                              _direction = defaultOpeningDirection(_type);
                              _initialDirection = _direction;
                            }
                          }),
                  decoration: const InputDecoration(labelText: '类型'),
                ),
                const SizedBox(height: 12),
                if (parents.isNotEmpty)
                  DropdownButtonFormField<String>(
                    value: _parentUuid,
                    isExpanded: true,
                    items: [
                      const DropdownMenuItem(value: null, child: Text('（无，作为二级分类）')),
                      ...parents.map((a) =>
                          DropdownMenuItem(value: a.uuid, child: Text(a.display))),
                    ],
                    onChanged: _isEdit
                        ? null
                        : (v) => setState(() => _parentUuid = v),
                    decoration: const InputDecoration(labelText: '父分类'),
                  ),
                if (parents.isNotEmpty) const SizedBox(height: 12),
                TextField(
                  controller: _nameCtl,
                  enabled: !_isEdit,
                  decoration: InputDecoration(
                    labelText: _parentUuid == null ? '账户名(ASCII)' : '分类英文标识',
                    hintText: _parentUuid == null ? '如 Assets:Cash' : '如 Snack',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _dispCtl,
                  enabled: !_isEdit,
                  decoration: const InputDecoration(labelText: '显示名（中文，可选）'),
                ),
                const SizedBox(height: 12),
                InkWell(
                  onTap: _isEdit ? null : _pickOpenDate,
                  child: InputDecorator(
                    decoration: InputDecoration(
                      labelText: '开户日',
                      helperText: _isEdit
                          ? null
                          : '不得晚于账本期初日（$latestOpenDate）',
                    ),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(_openDate.isEmpty ? '—' : _openDate),
                        if (!_isEdit)
                          const Icon(Icons.calendar_today_outlined, size: 16),
                      ],
                    ),
                  ),
                ),
                if (_openingAllowed) ...[
                  const SizedBox(height: 14),
                  Row(
                    children: [
                      const Text('期初方向', style: AppTheme.sectionTitle),
                      const SizedBox(width: 12),
                      ChoiceChip(
                        label: const Text('持有'),
                        selected: _direction == openingDirectionHold,
                        onSelected: !ledgerOnline || _openingLoading || _isClosed
                            ? null
                            : (_) => setState(() {
                                  _direction = openingDirectionHold;
                                  _directionTouched = true;
                                }),
                      ),
                      const SizedBox(width: 8),
                      ChoiceChip(
                        label: const Text('欠款'),
                        selected: _direction == openingDirectionOwe,
                        onSelected: !ledgerOnline || _openingLoading || _isClosed
                            ? null
                            : (_) => setState(() {
                                  _direction = openingDirectionOwe;
                                  _directionTouched = true;
                                }),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  TextField(
                    controller: _openBalCtl,
                    enabled: ledgerOnline && !_openingLoading && !_isClosed,
                    keyboardType: const TextInputType.numberWithOptions(decimal: true),
                    decoration: InputDecoration(
                      labelText: '期初余额（可选）',
                      hintText: '0.00',
                      suffixText: _displayCommodity,
                      helperText: _isClosed
                          ? '账户已关闭，期初不可修改'
                          : (!ledgerOnline
                              ? '联网后可设置期初'
                              : (_openingError ??
                                  '金额为非负值，方向决定账本符号')),
                      helperStyle: TextStyle(
                        color: (_isClosed || !ledgerOnline)
                            ? AppColors.warn
                            : AppColors.sub,
                      ),
                    ),
                  ),
                  if (_openingLoading)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: LinearProgressIndicator(minHeight: 2),
                    ),
                  if (_direction != defaultOpeningDirection(_type))
                    Padding(
                      padding: const EdgeInsets.only(top: 6),
                      child: Text(
                        '当前方向与${_type == 'Assets' ? '资产' : '负债'}账户默认方向不一致，保存前会二次确认。',
                        style: const TextStyle(color: AppColors.sub, fontSize: 12),
                      ),
                    ),
                ],
                const SizedBox(height: 18),
                ElevatedButton(
                  onPressed: _busy ? null : _save,
                  child: _busy
                      ? const SizedBox(
                          height: 18,
                          width: 18,
                          child: CircularProgressIndicator(
                              strokeWidth: 2, color: Colors.white),
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
                        child:
                            Center(child: Text(e, style: const TextStyle(fontSize: 22))),
                      ),
                    ))
                .toList(),
          ),
        ),
      ),
    );
  }
}
