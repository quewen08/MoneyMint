/// 等待邀请页（P1-B3）：账号已注册但尚未加入共享账本（role 为空），
/// 提示等待 owner 在成员管理中邀请加入。
/// 也用于「本地 token 失效但前端仍显示 loggedIn」的兜底（见 auth_controller.init）。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';

class AwaitInviteScreen extends StatelessWidget {
  const AwaitInviteScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    final name = auth.username;
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: ListenableBuilder(
                listenable: auth,
                builder: (ctx, _) => Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Icon(Icons.group_add_outlined,
                        size: 64, color: AppColors.blue),
                    const SizedBox(height: 12),
                    const Text('账号已注册',
                        textAlign: TextAlign.center,
                        style:
                            TextStyle(fontSize: 24, fontWeight: FontWeight.w800)),
                    const SizedBox(height: 8),
                    Text(
                      name == null
                          ? '你还没有加入任何账本。\n'
                              '如果是刚清空了后端数据库，请直接点下方「返回登录」后重新注册；'
                              '否则请等待账本所有者（owner）在「成员管理」中按用户名邀请你加入。'
                          : '你好，$name。\n'
                              '你还没有加入任何账本，请等待账本所有者（owner）'
                              '在「成员管理」中按用户名邀请你加入。',
                      textAlign: TextAlign.center,
                      style: const TextStyle(fontSize: 14, height: 1.6),
                    ),
                    const SizedBox(height: 24),
                    OutlinedButton(
                      onPressed: () async {
                        await auth.logout();
                      },
                      child: const Text('返回登录'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
