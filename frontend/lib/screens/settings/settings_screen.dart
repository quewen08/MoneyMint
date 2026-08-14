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
import 'sync_screen.dart';
import 'change_password_screen.dart';
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
                  AppCard(
                    child: Column(
                      children: [
                        _SetRow(
                          label: '账本管理',
                          trailing: Text(
                            '${auth.currentLedger?.name ?? '—'} ›',
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
                          label: '修改密码',
                          trailing: const Text('›', style: AppTheme.muted),
                          onTap: () => Navigator.push(
                            context,
                            MaterialPageRoute(
                                builder: (_) => const ChangePasswordScreen()),
                          ),
                        ),
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
