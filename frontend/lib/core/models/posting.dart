/// 一条借贷分录（posting）。amount 为有符号定点十进制字符串（禁止浮点）。

class LocalPosting {
  final String accountUuid; // 引用账户 uuid（跨端稳定），非服务端自增 id
  final String commodity;
  final String amount; // 有符号定点十进制字符串，如 "-38.50"

  LocalPosting({
    required this.accountUuid,
    required this.commodity,
    required this.amount,
  });

  Map<String, dynamic> toMap() => {
        'account_uuid': accountUuid,
        'commodity': commodity,
        'amount': amount,
      };

  factory LocalPosting.fromMap(Map<String, dynamic> m) => LocalPosting(
        accountUuid: m['account_uuid'] as String,
        commodity: m['commodity'] as String,
        amount: (m['amount'] as Object).toString(),
      );
}
