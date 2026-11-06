/// API 错误码 / 余额差额解析测试（不依赖真实后端）。
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_error.dart';

void main() {
  group('ApiException 错误体解析', () {
    test('解析 code 与关闭账户差额 balances', () {
      final e = ApiException.fromBody(
        409,
        '{"error":"账户余额未结清","code":"account_balance_not_zero",'
        '"balances":[{"commodity":"CNY","amount":"-3200.00"}]}',
      );
      expect(e.statusCode, 409);
      expect(e.message, '账户余额未结清');
      expect(e.code, 'account_balance_not_zero');
      expect(e.balanceDiffs, hasLength(1));
      expect(e.balanceDiffs.single.commodity, 'CNY');
      expect(e.balanceDiffs.single.amount, '-3200.00');
    });

    test('期初错误码解析（account_not_found / account_type_not_allowed）', () {
      final e = ApiException.fromBody(
        409,
        '{"error":"仅资产/负债可设置期初","code":"account_type_not_allowed"}',
      );
      expect(e.code, 'account_type_not_allowed');
      expect(e.balanceDiffs, isEmpty);
      expect(e.details?['code'], 'account_type_not_allowed');
    });

    test('非 JSON 错误体退化为 HTTP 状态', () {
      final e = ApiException.fromBody(502, '<html>bad gateway</html>');
      expect(e.message, 'HTTP 502');
      expect(e.code, isNull);
      expect(e.balanceDiffs, isEmpty);
    });

    test('错误体缺少 balances 时安全返回空列表', () {
      final e = ApiException.fromBody(
        404,
        '{"error":"账户不存在","code":"account_not_found"}',
      );
      expect(e.balanceDiffs, isEmpty);
    });
  });
}
