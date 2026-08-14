// 轻量冒烟测试：验证应用能正常构建并进入登录/主框架。
// 注意：前端依赖 dart:html（仅 Web 目标可用），请用 `flutter test -d chrome` 运行。
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:frontend/main.dart';

void main() {
  testWidgets('应用可构建并显示账本标题', (WidgetTester tester) async {
    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();
    // 未登录态下登录页或主框架都会出现「家庭记账」标题。
    expect(find.text('家庭记账'), findsWidgets);
  });
}
