// 轻量冒烟测试：验证 UI 基础组件可构建。
//
// 注意：应用入口 `main.dart` 依赖 dart:html（仅 Web 目标可用），
// 默认的 `flutter test`（VM）无法编译它；完整应用冒烟需使用
// `flutter test --platform chrome`。这里改为测试不依赖 dart:html 的
// 基础组件，保证 VM 测试套件始终可运行。
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:frontend/core/models/account.dart';
import 'package:frontend/widgets/account_icon.dart';

void main() {
  testWidgets('账户图标组件可构建并回退默认 emoji', (WidgetTester tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: AccountIcon(
          LocalAccount(
              uuid: 'a1', name: 'Assets:Cash', type: 'Assets', openDate: '2026-01-01'),
        ),
      ),
    ));

    expect(find.byType(AccountIcon), findsOneWidget);
    expect(find.text('💰'), findsOneWidget);
  });

  testWidgets('无图标时按账户类型回退默认颜色', (WidgetTester tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: AccountIcon(
          LocalAccount(
              uuid: 'l1',
              name: 'Liabilities:Card',
              type: 'Liabilities',
              openDate: '2026-01-01'),
        ),
      ),
    ));
    expect(defaultAccountColor('Liabilities'), const Color(0xFFE8590C));
    expect(parseHexColor('#3B6EF6'), const Color(0xFF3B6EF6));
    expect(parseHexColor('zzzz'), isNull);
  });
}
