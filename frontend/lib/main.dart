/// 应用入口：配置主题、提供 AppScope，并按登录态在登录页 / 主框架间切换。
import 'package:flutter/material.dart';
import 'app/scope.dart';
import 'app/theme.dart';
import 'screens/auth/login_screen.dart';
import 'screens/auth/await_invite_screen.dart';
import 'screens/shell/app_shell.dart';

void main() => runApp(const MyApp());

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) => AppScope(
        child: MaterialApp(
          title: '家庭记账',
          theme: AppTheme.light,
          debugShowCheckedModeBanner: false,
          home: const AuthGate(),
        ),
      );
}

/// 登录门禁：启动时恢复 token；已登录进主页，否则显示登录/注册页。
class AuthGate extends StatefulWidget {
  const AuthGate({super.key});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    AppScope.of(context).auth.init();
  }

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    return ListenableBuilder(
      listenable: auth,
      builder: (context, _) {
        if (!auth.ready) {
          return const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          );
        }
        if (auth.loggedIn) {
          // 已登录：是账本成员进主框架；尚未被邀请则显示等待邀请页（P1-B3）。
          if (auth.isMember) return const AppShell();
          return const AwaitInviteScreen();
        }
        return const LoginScreen();
      },
    );
  }
}
