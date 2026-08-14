/// 应用作用域：在组件树根部提供单例控制器（AuthController / LedgerController），
/// 各页面通过 AppScope.of(context) 取用，避免层层传参。
/// Auth / Ledger 共用同一 LedgerApi 与 LocalStore 实例（token 静态共享、store 为单例）。
import 'package:flutter/widgets.dart';
import '../core/api/ledger_api.dart';
import '../core/store/local_store.dart';
import '../viewmodels/auth_controller.dart';
import '../viewmodels/ledger_controller.dart';

class AppScope extends InheritedWidget {
  final LedgerApi api;
  final LocalStore store;
  final AuthController auth;
  final LedgerController ledger;

  // 初始化顺序：先建共享实例，再注入给两个控制器，保证 token/store 一致。
  // 注意：auth/ledger 必须在初始化列表中用新实例表达式构造，
  // 不能引用同列表中的 api/store（Dart 禁止在初始化列表里用 this 引用字段）。
  // LedgerApi 的 token 为静态共享，LocalStore 为工厂单例，所以多建实例是安全的。
  AppScope({super.key, required super.child})
      : api = LedgerApi(),
        store = LocalStore(),
        auth = AuthController(LedgerApi()),
        ledger = LedgerController(LedgerApi(), LocalStore());

  static AppScope of(BuildContext context) {
    final scope = context.dependOnInheritedWidgetOfExactType<AppScope>();
    assert(scope != null, 'AppScope 未在组件树根部提供');
    return scope!;
  }

  @override
  bool updateShouldNotify(AppScope oldWidget) => false;
}
