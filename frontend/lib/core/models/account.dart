/// 本地账户模型（与后端 sync 协议对齐，uuid 为跨端身份键）。

class LocalAccount {
  final String uuid;
  final String name; // Beancount ASCII 账户名
  final String? displayName; // 中文显示名（可选）
  final String type; // Assets / Liabilities / Equity / Income / Expenses
  final String openDate;
  final String? restriction; // 限定币种（可选）

  LocalAccount({
    required this.uuid,
    required this.name,
    this.displayName,
    required this.type,
    required this.openDate,
    this.restriction,
  });

  /// 展示名：优先中文 display_name，回退 ASCII name。
  String get display => (displayName?.isNotEmpty == true) ? displayName! : name;

  Map<String, dynamic> toMap() => {
        'uuid': uuid,
        'name': name,
        'display_name': displayName,
        'type': type,
        'open_date': openDate,
        'commodity_restriction': restriction,
      };

  factory LocalAccount.fromMap(Map<String, dynamic> m) => LocalAccount(
        uuid: m['uuid'] as String,
        name: m['name'] as String,
        displayName: m['display_name'] as String?,
        type: m['type'] as String,
        openDate: m['open_date'] as String,
        restriction: m['commodity_restriction'] as String?,
      );
}

/// 账户展示名：优先中文 display_name，回退 ASCII name。
String accountDisplayName(LocalAccount a) => a.display;
