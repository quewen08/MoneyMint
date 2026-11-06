/// 期初余额 / 恒等式检查 / 导出告警的传输模型。
///
/// 金额一律为定点十进制字符串（禁止浮点）。方向取值为 `hold`（持有）/
/// `owe`（欠款），账本符号由服务端按账户类型推导，本层只负责解析。
library;

/// 期初方向：持有。
const String openingDirectionHold = 'hold';

/// 期初方向：欠款。
const String openingDirectionOwe = 'owe';

/// 期初金额方向的中文文案（D-03：固定为「持有 / 欠款」）。
String openingDirectionLabel(String direction) =>
    direction == openingDirectionOwe ? '欠款' : '持有';

/// 期初读取结果（`GET /api/accounts/{uuid}/opening`）。
class OpeningView {
  final bool hasOpening;
  final String accountUuid;

  /// 绝对值（仅 [hasOpening] 为 true 时有意义）。
  final String amount;

  /// `hold` | `owe`。
  final String direction;

  /// 账本符号（欠款为负）。
  final String signedAmount;
  final String commodity;
  final int version;
  final String date;
  final String transactionUuid;

  const OpeningView({
    required this.hasOpening,
    required this.accountUuid,
    this.amount = '',
    this.direction = '',
    this.signedAmount = '',
    this.commodity = '',
    this.version = 0,
    this.date = '',
    this.transactionUuid = '',
  });

  factory OpeningView.fromJson(Map<String, dynamic> j) => OpeningView(
        hasOpening: j['has_opening'] as bool? ?? false,
        accountUuid: j['account_uuid'] as String? ?? '',
        amount: _str(j['amount']),
        direction: j['direction'] as String? ?? '',
        signedAmount: _str(j['signed_amount']),
        commodity: j['commodity'] as String? ?? '',
        version: (j['version'] as num?)?.toInt() ?? 0,
        date: j['date'] as String? ?? '',
        // 后端字段为 transaction_uuid；设计稿曾写作 txn_uuid，两者都兼容。
        transactionUuid:
            (j['transaction_uuid'] as String?) ?? (j['txn_uuid'] as String?) ?? '',
      );
}

/// 期初设置/修正/清空结果（`POST /api/accounts/{uuid}/opening`）。
class OpeningResult {
  /// created | updated | cleared | unchanged。
  final String status;
  final String accountUuid;
  final String transactionUuid;
  final String reversalUuid;
  final int version;
  final String signedAmount;
  final String commodity;
  final String date;

  const OpeningResult({
    required this.status,
    required this.accountUuid,
    this.transactionUuid = '',
    this.reversalUuid = '',
    this.version = 0,
    this.signedAmount = '',
    this.commodity = '',
    this.date = '',
  });

  factory OpeningResult.fromJson(Map<String, dynamic> j) => OpeningResult(
        status: j['status'] as String? ?? '',
        accountUuid: j['account_uuid'] as String? ?? '',
        transactionUuid: j['transaction_uuid'] as String? ?? '',
        reversalUuid: j['reversal_uuid'] as String? ?? '',
        version: (j['version'] as num?)?.toInt() ?? 0,
        signedAmount: _str(j['signed_amount']),
        commodity: j['commodity'] as String? ?? '',
        date: j['date'] as String? ?? '',
      );
}

/// 某币种恒等式差额（服务端 `diffs` 的一项）。
class IdentityDiff {
  final String commodity;

  /// 该币种全部 posting 的合计（非 0 即失衡）。
  final String sum;

  const IdentityDiff({required this.commodity, required this.sum});

  factory IdentityDiff.fromJson(Map<String, dynamic> j) => IdentityDiff(
        commodity: j['commodity'] as String? ?? '',
        sum: _str(j['sum']),
      );
}

/// 恒等式检查结果（`GET /api/identity/check`）。
class IdentityResult {
  final bool balanced;
  final List<IdentityDiff> diffs;
  final int postingCount;
  final String checkedAt;
  final int durationMs;

  const IdentityResult({
    required this.balanced,
    this.diffs = const [],
    this.postingCount = 0,
    this.checkedAt = '',
    this.durationMs = 0,
  });

  factory IdentityResult.fromJson(Map<String, dynamic> j) => IdentityResult(
        balanced: j['balanced'] as bool? ?? true,
        diffs: (j['diffs'] as List? ?? const [])
            .whereType<Map>()
            .map((e) => IdentityDiff.fromJson(Map<String, dynamic>.from(e)))
            .toList(),
        postingCount: (j['posting_count'] as num?)?.toInt() ?? 0,
        checkedAt: j['checked_at'] as String? ?? '',
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
      );
}

/// 关闭账户时某币种的余额差额（409 `account_balance_not_zero` 的 `balances`）。
class BalanceDiff {
  final String commodity;
  final String amount;

  const BalanceDiff({required this.commodity, required this.amount});

  factory BalanceDiff.fromJson(Map<String, dynamic> j) => BalanceDiff(
        commodity: j['commodity'] as String? ?? '',
        amount: _str(j['amount']),
      );

  /// 宽松解析：接受 `[{commodity,amount}]`，缺少字段或空值的项跳过。
  static List<BalanceDiff> listFrom(Object? raw) {
    if (raw is! List) return const [];
    final out = <BalanceDiff>[];
    for (final e in raw.whereType<Map>()) {
      final m = Map<String, dynamic>.from(e);
      final commodity = m['commodity'];
      final amount = _str(m['amount']);
      if (commodity is! String || commodity.isEmpty || amount.isEmpty) continue;
      out.add(BalanceDiff(commodity: commodity, amount: amount));
    }
    return out;
  }
}

/// 单文件导出的响应：正文 + 服务端恒等式告警头。
class ExportFileResult {
  final String text;
  final bool balanced;
  final List<IdentityDiff> diffs;

  const ExportFileResult({
    required this.text,
    this.balanced = true,
    this.diffs = const [],
  });
}

/// 多文件目录导出的响应：zip 字节 + 服务端恒等式告警头。
class ExportArchiveResult {
  final List<int> bytes;
  final bool balanced;
  final List<IdentityDiff> diffs;

  const ExportArchiveResult({
    required this.bytes,
    this.balanced = true,
    this.diffs = const [],
  });
}

/// 把 JSON 中可能是数字/字符串的金额统一转为字符串。
String _str(Object? v) {
  if (v == null) return '';
  if (v is String) return v;
  return v.toString();
}
