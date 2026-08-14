/// 成员管理页（P1-B3）：owner 按用户名邀请已注册用户加入共享账本，
/// 调整角色（编辑/只读）、移除成员、重置成员密码。非 owner 只读提示。
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/sync_models.dart';
import '../../widgets/common.dart';

class MembersScreen extends StatefulWidget {
  const MembersScreen({super.key});

  @override
  State<MembersScreen> createState() => _MembersScreenState();
}

class _MembersScreenState extends State<MembersScreen> {
  bool _loaded = false;
  bool _busy = false;
  List<Member>? _members;
  String? _error;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_loaded) return;
    _loaded = true;
    _load();
  }

  Future<void> _load() async {
    final role = AppScope.of(context).auth.role;
    if (role != 'owner') {
      // 非 owner 直接返回（后端会 403，UI 已用 "仅账本所有者..." 兜底）。
      return;
    }
    final ledger = AppScope.of(context).ledger;
    try {
      final members = await ledger.loadMembers();
      if (!mounted) return;
      setState(() {
        _members = members;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() => _busy = true);
    try {
      await action();
      await _load();
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '操作失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _addMember() async {
    final usernameCtl = TextEditingController();
    var role = 'viewer';
    final result = await showDialog<_AddResult>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('邀请成员'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: usernameCtl,
              autofocus: true,
              decoration: const InputDecoration(
                labelText: '用户名（对方需先注册）',
                hintText: '如 alice',
              ),
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              value: role,
              decoration: const InputDecoration(labelText: '角色'),
              items: const [
                DropdownMenuItem(value: 'editor', child: Text('编辑：可记账改账')),
                DropdownMenuItem(value: 'viewer', child: Text('只读：仅查看')),
              ],
              onChanged: (v) => role = v!,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          ElevatedButton(
            onPressed: () {
              final name = usernameCtl.text.trim();
              if (name.isEmpty) return;
              Navigator.pop(ctx, _AddResult(name, role));
            },
            child: const Text('邀请'),
          ),
        ],
      ),
    );
    if (result == null) return;
    await _run(
      () => AppScope.of(context).ledger.addMember(result.username, result.role),
    );
  }

  Future<void> _changeRole(Member m) async {
    final current = m.role == 'editor' ? 'viewer' : 'editor';
    await _run(
      () => AppScope.of(context).ledger.updateMemberRole(m.userId, current),
    );
  }

  Future<void> _removeMember(Member m) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('移除成员？'),
        content: Text('移除后「${m.username}」将无法访问共享账本，其本地数据仍保留。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.warn),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('移除'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    await _run(() => AppScope.of(context).ledger.removeMember(m.userId));
  }

  Future<void> _resetPassword(Member m) async {
    final pwdCtl = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('重置「${m.username}」密码'),
        content: TextField(
          controller: pwdCtl,
          obscureText: true,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '新密码',
            hintText: '至少 6 位',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          ElevatedButton(
            onPressed: () {
              final p = pwdCtl.text;
              if (p.length < 6) return;
              Navigator.pop(ctx, p);
            },
            child: const Text('重置'),
          ),
        ],
      ),
    );
    if (result == null) return;
    await _run(
      () => AppScope.of(context).ledger.resetMemberPassword(m.userId, result),
    );
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context);
    final role = AppScope.of(context).auth.role;
    final isOwner = role == 'owner';
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (!isOwner) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Text('仅账本所有者（owner）可以管理成员。', style: AppTheme.muted),
            ),
          );
        }
        final list = _buildList();
        return SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
          child: wide
              ? Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 900),
                    child: list,
                  ),
                )
              : list,
        );
      },
    );
  }

  Widget _buildList() {
    // 注意：这里用 Column 而非 ListView。此前用 ListView 直接嵌套在 Center 内，
    // 在 PC（wide）首帧渲染时 SliverPadding 的子 SliverList 几何为 null，
    // 触发 "Unexpected null value"（sliver_padding.dart:147）。改为
    // SingleChildScrollView + Column（与 reports_screen 一致）可彻底规避。
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Text('加载失败: $_error', style: AppTheme.muted),
          ),
        AppCard(
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      '家庭账本',
                      style: TextStyle(
                        fontWeight: FontWeight.w700,
                        fontSize: 16,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text('成员通过用户名加入，需先注册账号', style: AppTheme.muted),
                  ],
                ),
              ),
              ElevatedButton.icon(
                onPressed: _busy ? null : _addMember,
                icon: const Icon(Icons.person_add, size: 18),
                label: const Text('邀请成员'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        if (_members == null)
          const Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: CircularProgressIndicator()),
          )
        else if (_members!.isEmpty)
          const Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: Text('还没有成员', style: AppTheme.muted)),
          )
        else
          AppCard(
            padding: EdgeInsets.zero,
            child: Column(
              children: [
                for (var i = 0; i < _members!.length; i++) ...[
                  if (i > 0) const Divider(height: 1, color: AppColors.line),
                  _MemberRow(
                    member: _members![i],
                    busy: _busy,
                    onToggleRole: () => _changeRole(_members![i]),
                    onRemove: () => _removeMember(_members![i]),
                    onResetPwd: () => _resetPassword(_members![i]),
                  ),
                ],
              ],
            ),
          ),
      ],
    );
  }
}

class _MemberRow extends StatelessWidget {
  final Member member;
  final bool busy;
  final VoidCallback onToggleRole;
  final VoidCallback onRemove;
  final VoidCallback onResetPwd;
  const _MemberRow({
    required this.member,
    required this.busy,
    required this.onToggleRole,
    required this.onRemove,
    required this.onResetPwd,
  });

  @override
  Widget build(BuildContext context) {
    final isOwner = member.role == 'owner';
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(
        children: [
          CircleAvatar(
            radius: 16,
            backgroundColor: isOwner
                ? AppColors.blue.withOpacity(0.12)
                : const Color(0xFFEEF1F6),
            child: Text(
              member.username.characters.first.toUpperCase(),
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: isOwner ? AppColors.blue : AppColors.sub,
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  member.username,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (member.displayName.isNotEmpty &&
                    member.displayName != member.username)
                  Text(member.displayName, style: AppTheme.muted),
              ],
            ),
          ),
          _RoleTag(role: member.role),
          if (!isOwner) ...[
            const SizedBox(width: 4),
            IconButton(
              tooltip: '切换角色',
              onPressed: busy ? null : onToggleRole,
              icon: const Icon(Icons.swap_horiz, size: 18),
              color: AppColors.sub,
            ),
            IconButton(
              tooltip: '重置密码',
              onPressed: busy ? null : onResetPwd,
              icon: const Icon(Icons.password, size: 18),
              color: AppColors.sub,
            ),
            IconButton(
              tooltip: '移除成员',
              onPressed: busy ? null : onRemove,
              icon: const Icon(Icons.person_remove_outlined, size: 18),
              color: AppColors.warn,
            ),
          ],
        ],
      ),
    );
  }
}

class _RoleTag extends StatelessWidget {
  final String role;
  const _RoleTag({required this.role});

  @override
  Widget build(BuildContext context) {
    final color = switch (role) {
      'owner' => AppColors.blue,
      'editor' => AppColors.green,
      _ => AppColors.sub,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withOpacity(0.12),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        Member.roleLabel(role),
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}

class _AddResult {
  final String username;
  final String role;
  _AddResult(this.username, this.role);
}
