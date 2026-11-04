/// 我的 / 设置页（设计稿「我的」）：同步状态卡、导出 Beancount、成员管理、修改密码、主题、关于、退出。
import 'dart:convert';
import 'dart:html' as html;
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../viewmodels/auth_controller.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/common.dart';
import '../members/members_screen.dart';
import 'backup_card.dart';
import 'sync_screen.dart';
import 'change_password_screen.dart';
import 'sessions_screen.dart';
import 'ledgers_screen.dart';

class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final auth = AppScope.of(context).auth;
    final wide = isWide(context);
    return ListenableBuilder(
      listenable: Listenable.merge([ledger, auth]),
      builder: (context, _) => SingleChildScrollView(
        child: Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: wide ? 720 : double.infinity),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              child: Column(
                children: [
                  // 身份卡（F-8）：头像点 + 用户名 + 系统管理员 / 账本角色徽章。
                  _IdentityCard(auth: auth),
                  const SizedBox(height: 12),
                  AppCard(
                    onTap: () => Navigator.push(
                      context,
                      MaterialPageRoute(builder: (_) => const SyncScreen()),
                    ),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('同步状态', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16)),
                              const SizedBox(height: 4),
                              Text(
                                '上次同步：刚刚 · 待同步 ${ledger.pending} 条',
                                style: AppTheme.muted,
                              ),
                            ],
                          ),
                        ),
                        Icon(
                          ledger.online ? Icons.arrow_forward_ios : Icons.cloud_off,
                          color: AppColors.green,
                          size: 20,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                  const _DefaultAccountsCard(),
                  const SizedBox(height: 12),
                  // 数据备份（M1）：仅系统管理员可见，内部自行判断是否渲染
                  const BackupCard(),
                  const SizedBox(height: 12),
                  AppCard(
                    child: Column(
                      children: [
                        _SetRow(
                          label: '账本管理',
                          trailing: Text(
                            '${auth.currentLedger?.name ?? '—'}（当前） ›',
                            style: const TextStyle(color: AppColors.blue),
                          ),
                          onTap: () => Navigator.push(
                            context,
                            MaterialPageRoute(
                                builder: (_) => const LedgersScreen()),
                          ),
                        ),
                        _SetRow(
                          label: '导出 Beancount',
                          trailing: const Text('下载 ›', style: TextStyle(color: AppColors.blue)),
                          onTap: () => _export(context, ledger),
                        ),
                        _SetRow(
                          label: '成员管理',
                          trailing: Text(
                            auth.role == 'owner' ? '所有者 ›' : '仅 owner ›',
                            style: TextStyle(
                                color: auth.role == 'owner'
                                    ? AppColors.blue
                                    : AppColors.sub),
                          ),
                          enabled: auth.role == 'owner',
                          onTap: auth.role == 'owner'
                              ? () => Navigator.push(
                                    context,
                                    MaterialPageRoute(
                                        builder: (_) => const MembersScreen()),
                                  )
                              : null,
                        ),
                        _SetRow(
                          label: '登录设备',
                          trailing: const Text('查看 / 吊销 ›',
                              style: TextStyle(color: AppColors.blue)),
                          onTap: () => Navigator.push(
                            context,
                            MaterialPageRoute(builder: (_) => const SessionsScreen()),
                          ),
                        ),
                        _SetRow(
                          label: '修改密码',
                          trailing: const Text('›', style: AppTheme.muted),
                          onTap: () => Navigator.push(
                            context,
                            MaterialPageRoute(
                                builder: (_) => const ChangePasswordScreen()),
                          ),
                        ),
                        // 开放注册开关（B-4 / F-8）：仅系统管理员可见可操作，
                        // 切换持久化到服务端 app_settings，对所有端生效。
                        if (auth.isAdmin) const _RegistrationRow(),
                        _SetRow(
                          label: '主题',
                          trailing: const Text('跟随系统 ›', style: AppTheme.muted),
                        ),
                        _SetRow(
                          label: '关于',
                          trailing: const Text('v0.1 ›', style: AppTheme.muted),
                        ),
                        _SetRow(
                          label: '退出登录',
                          trailing: const Icon(Icons.chevron_right, color: AppColors.warn),
                          danger: true,
                          onTap: () => _logout(context, auth),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _export(BuildContext context, LedgerController ledger) async {
    try {
      final text = await ledger.exportText();
      final blob = html.Blob([utf8.encode(text)]);
      final url = html.Url.createObjectUrlFromBlob(blob);
      html.AnchorElement(href: url)
        ..download = 'family.beancount'
        ..click();
      html.Url.revokeObjectUrl(url);
      showAppSnack(context, '已导出 family.beancount');
    } catch (e) {
      showAppSnack(context, '导出失败（需联网）: $e', warn: true);
    }
  }

  Future<void> _logout(BuildContext context, AuthController auth) async {
    await auth.logout();
    if (!context.mounted) return;
    // AuthGate 监听 auth 状态，会自动切回登录页。
  }
}

/// 默认账户（0.4-C，纯前端记忆）查看与清除。按账本隔离。
class _DefaultAccountsCard extends StatefulWidget {
  const _DefaultAccountsCard();

  @override
  State<_DefaultAccountsCard> createState() => _DefaultAccountsCardState();
}

class _DefaultAccountsCardState extends State<_DefaultAccountsCard> {
  Map<String, String> _defs = {};
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _load();
  }

  Future<void> _load() async {
    final defs = await AppScope.of(context).ledger.defaultAccounts;
    if (!mounted) return;
    setState(() => _defs = defs);
  }

  String _name(String? uuid) {
    if (uuid == null) return '未设置';
    final found =
        AppScope.of(context).ledger.accounts.where((x) => x.uuid == uuid);
    return found.isEmpty ? '（已删除）' : found.first.display;
  }

  @override
  Widget build(BuildContext context) {
    const kinds = [
      ('expense', '默认支出'),
      ('income', '默认收入'),
      ('transferOut', '默认转账来源'),
      ('transferIn', '默认转账目标'),
    ];
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionTitle('默认账户'),
          const SizedBox(height: 4),
          for (final k in kinds)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 6),
              child: Row(
                children: [
                  Expanded(child: Text(k.$2, style: const TextStyle(fontSize: 14))),
                  Text(_name(_defs[k.$1]), style: AppTheme.muted),
                ],
              ),
            ),
          if (_defs.isNotEmpty)
            Align(
              alignment: Alignment.centerRight,
              child: TextButton(
                onPressed: () async {
                  await AppScope.of(context).ledger.clearDefaultAccounts();
                  await _load();
                },
                child: const Text('清除全部默认',
                    style: TextStyle(color: AppColors.warn)),
              ),
            ),
        ],
      ),
    );
  }
}

/// 身份卡（F-8）：头像点（名字散列色）+ 用户名 + 角色徽章。
/// 两层角色分开标注：系统管理员（全局）与 owner（账本级）。
class _IdentityCard extends StatelessWidget {
  final AuthController auth;
  const _IdentityCard({required this.auth});

  @override
  Widget build(BuildContext context) {
    final name = auth.username ?? '未登录';
    final role = auth.role;
    return AppCard(
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: AppColors.softOf(name),
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: Text(
              name.characters.first.toUpperCase(),
              style: const TextStyle(
                  fontSize: 18, fontWeight: FontWeight.w800, color: AppColors.ink),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name,
                    style: const TextStyle(
                        fontSize: 16, fontWeight: FontWeight.w700)),
                const SizedBox(height: 4),
                Row(
                  children: [
                    if (auth.isAdmin) ...[
                      _badge('系统管理员', AppColors.blue),
                      const SizedBox(width: 6),
                    ],
                    _badge(
                      role == 'owner'
                          ? '账本所有者'
                          : role == 'editor'
                              ? '可编辑'
                              : role == 'viewer'
                                  ? '只读'
                                  : '未加入账本',
                      role == 'owner'
                          ? AppColors.green
                          : role.isEmpty
                              ? AppColors.sub
                              : AppColors.ink,
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _badge(String text, Color color) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
        decoration: BoxDecoration(
          color: color.withValues(alpha: .1),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Text(text,
            style: TextStyle(
                fontSize: 11, fontWeight: FontWeight.w600, color: color)),
      );
}

/// 开放注册开关（B-4）：仅系统管理员渲染。切换即调服务端并持久化。
class _RegistrationRow extends StatefulWidget {
  const _RegistrationRow();

  @override
  State<_RegistrationRow> createState() => _RegistrationRowState();
}

class _RegistrationRowState extends State<_RegistrationRow> {
  bool? _enabled; // null = 加载中
  bool _busy = false;
  bool _checked = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // AppScope.of 依赖 InheritedWidget，不能在 initState 里取，放这里且仅一次。
    if (_checked) return;
    _checked = true;
    _load();
  }

  Future<void> _load() async {
    final v = await AppScope.of(context).auth.api.registrationEnabled();
    if (!mounted) return;
    setState(() => _enabled = v);
  }

  Future<void> _toggle(bool v) async {
    setState(() => _busy = true);
    try {
      await AppScope.of(context).auth.api.setRegistrationEnabled(v);
      if (!mounted) return;
      setState(() {
        _enabled = v;
        _busy = false;
      });
      showAppSnack(context, v ? '已开放注册' : '已关闭注册');
    } catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      showAppSnack(context, '切换失败（需系统管理员）: $e', warn: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final v = _enabled ?? false;
    return _SetRow(
      label: '开放注册',
      enabled: _enabled != null && !_busy,
      trailing: _busy
          ? const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2))
          : Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(_enabled == null ? '…' : (v ? '开放' : '关闭'),
                    style: AppTheme.muted),
                Switch(value: v, onChanged: _enabled == null ? null : _toggle),
              ],
            ),
    );
  }
}

class _SetRow extends StatelessWidget {
  final String label;
  final Widget trailing;
  final VoidCallback? onTap;
  final bool danger;
  final bool enabled;
  const _SetRow({
    required this.label,
    required this.trailing,
    this.onTap,
    this.danger = false,
    this.enabled = true,
  });

  @override
  Widget build(BuildContext context) => InkWell(
        onTap: enabled ? onTap : null,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 4),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: danger ? FontWeight.w700 : FontWeight.normal,
                    color: danger ? AppColors.warn : AppColors.ink,
                  ),
                ),
              ),
              trailing,
            ],
          ),
        ),
      );
}
