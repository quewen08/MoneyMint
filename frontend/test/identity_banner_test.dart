/// 首页恒等式告警横幅 + 关闭账户余额拦截弹窗的展示测试。
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/opening_models.dart';
import 'package:frontend/core/models/account.dart';
import 'package:frontend/widgets/balance_blocked_dialog.dart';
import 'package:frontend/widgets/identity_banner.dart';

void main() {
  group('IdentityBanner', () {
    testWidgets('不平衡时展示差额，点击关闭触发回调', (tester) async {
      var dismissed = false;
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: IdentityBanner(
            result: const IdentityResult(
              balanced: false,
              diffs: [
                IdentityDiff(commodity: 'CNY', sum: '0.01'),
                IdentityDiff(commodity: 'USD', sum: '-0.02'),
              ],
            ),
            warning: null,
            onDismiss: () => dismissed = true,
          ),
        ),
      ));
      expect(find.text('账本恒等式不一致'), findsOneWidget);
      expect(find.textContaining('CNY'), findsWidgets);
      expect(find.textContaining('USD'), findsWidgets);
      await tester.tap(find.byKey(const ValueKey('identity-banner-dismiss')));
      await tester.pump();
      expect(dismissed, isTrue);
    });

    testWidgets('本地净值告警文案透传展示', (tester) async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: IdentityBanner(
            result: const IdentityResult(balanced: true),
            warning: '净值双算不一致：100.00 vs 100.01',
            onDismiss: () {},
          ),
        ),
      ));
      expect(find.text('净值双算不一致：100.00 vs 100.01'), findsOneWidget);
    });
  });

  group('关闭账户余额拦截', () {
    testWidgets('展示按币种差额并提供转出入口', (tester) async {
      final account = LocalAccount(
        uuid: 'acc-1',
        name: 'Assets:Bank:CNY',
        displayName: '银行卡',
        type: 'Assets',
        openDate: '2026-01-01',
      );
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: BalanceBlockedDialog(
            account: account,
            diffs: const [
              BalanceDiff(commodity: 'CNY', amount: '-3200.00'),
            ],
            canTransfer: true,
          ),
        ),
      ));
      expect(find.text('账户余额未结清'), findsOneWidget);
      expect(find.textContaining('银行卡'), findsWidgets);
      expect(find.textContaining('CNY'), findsWidgets);
      expect(find.text('转到其他账户'), findsOneWidget);
      expect(find.text('取消'), findsOneWidget);
    });

    testWidgets('无可用转入账户时不展示转出按钮', (tester) async {
      final account = LocalAccount(
        uuid: 'acc-2',
        name: 'Liabilities:Card',
        displayName: '信用卡',
        type: 'Liabilities',
        openDate: '2026-01-01',
      );
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: BalanceBlockedDialog(
            account: account,
            diffs: const [BalanceDiff(commodity: 'CNY', amount: '-100.00')],
            canTransfer: false,
          ),
        ),
      ));
      expect(find.text('转到其他账户'), findsNothing);
      expect(find.textContaining('暂无可转入的资产账户'), findsOneWidget);
    });
  });
}
