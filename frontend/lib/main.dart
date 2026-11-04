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
  /// 不能用一次性 bool 门闩：flutter run 的热重载会重跑 MyApp.build，
  /// AppScope 随之重建出全新的 AuthController——旧门闩挡住 init 后，
  /// 新控制器 ready 永远 false，页面表现为「先正常、刷新/热重载后永远转圈」。
  /// 按实例判断：同一个控制器只 init 一次，换了实例（热重载/热重启）自动重跑。
  AuthController? _initedAuth;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final auth = AppScope.of(context).auth;
    if (_initedAuth == auth) return;
    _initedAuth = auth;
    auth.init();
  }

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    return ListenableBuilder(
      listenable: auth,
      builder: (context, _) {
        // 诊断打点（debug）：观察控制器实例号与登录态。
        // 若刷新后出现「auth=#不同实例 ready=false」且不再有 init 日志，
        // 即热重载重建了 AppScope 但 AuthGate 的 _initialized 门闩挡住了 init。
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
