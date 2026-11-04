/// 应用入口：配置主题、提供 AppScope，并按登录态在登录页 / 主框架间切换。
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'app/scope.dart';
import 'app/theme.dart';
import 'screens/auth/await_invite_screen.dart';
import 'screens/auth/login_screen.dart';
import 'screens/shell/app_shell.dart';
import 'viewmodels/auth_controller.dart';

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
  /// 已执行过 init 的控制器实例。
  ///
  /// 两个坑都不能踩：
  /// 1. 不能用一次性 bool 门闩：热重载会重建 AppScope，换新控制器后
  ///    旧门闩挡住 init，ready 永远 false。
  /// 2. 检查不能只放 didChangeDependencies：AppScope.updateShouldNotify
  ///    恒为 false，热重载更新 scope 时 didChangeDependencies 不会重跑，
  ///    而 build 必定重跑（整树从根重建）——所以检查放 build 里。
  AuthController? _initedAuth;

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    if (_initedAuth != auth) {
      _initedAuth = auth;
      // init 是 async 且首个 notifyListeners 在 await 之后，
      // 构建期调用不会同步触发 setState。
      auth.init();
    }
    return ListenableBuilder(
      listenable: auth,
      builder: (context, _) {
        // 诊断打点（debug）：观察控制器实例号与登录态。
        if (kDebugMode) {
          debugPrint(
              '[gate] auth=#${identityHashCode(auth)} ready=${auth.ready} '
              'loggedIn=${auth.loggedIn} member=${auth.isMember}');
        }
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
