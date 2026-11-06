/// API 异常与错误体解析（纯 Dart，不依赖 dart:html，便于 VM 单测）。
library;

import 'dart:convert';

import 'opening_models.dart';

/// 带状态码的 API 异常，便于 UI 按 401/403/409 分别处理。
///
/// [code] 为服务端错误码（如 `account_balance_not_zero`），
/// [details] 保留完整错误体（余额缺口在 `balances` 字段内）。
class ApiException implements Exception {
  final int statusCode;
  final String message;
  final String? code;
  final Map<String, dynamic>? details;

  ApiException(
    this.statusCode,
    this.message, {
    this.code,
    this.details,
  });

  /// 从错误响应体解析。无法解析 JSON 时退化为 `HTTP <status>`。
  factory ApiException.fromBody(int status, String body) {
    var message = 'HTTP $status';
    String? code;
    Map<String, dynamic>? details;
    try {
      final decoded = jsonDecode(body);
      if (decoded is Map) {
        details = Map<String, dynamic>.from(decoded);
        final err = decoded['error'];
        if (err is String && err.isNotEmpty) message = err;
        final c = decoded['code'];
        if (c is String && c.isNotEmpty) code = c;
      }
    } catch (_) {
      // 非 JSON 错误体（如网关 HTML）：保留默认文案。
    }
    return ApiException(status, message, code: code, details: details);
  }

  /// 409 关闭账户冲突时的按币种余额差额；非该场景返回空列表。
  List<BalanceDiff> get balanceDiffs => BalanceDiff.listFrom(details?['balances']);

  @override
  String toString() => message;
}
