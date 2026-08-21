/// 本地账户模型（与后端 sync 协议对齐，uuid 为跨端身份键）。

class LocalAccount {
  final String uuid;
  final String name; // Beancount ASCII 账户名
  final String? displayName; // 中文显示名（可选）
  final String type; // Assets / Liabilities / Equity / Income / Expenses
  final String openDate;
  final String? closeDate; // 关闭日（YYYY-MM-DD）；非空表示已关闭（Beancount close，0.4-A）
  final String? restriction; // 限定币种（可选）
  final String? icon; // 图标（emoji，展示用）
  final String? color; // 颜色 hex（展示用）
  final String? parentUuid; // 父账户 uuid（三级分类）
  final String? subType; // 账户子类型（展示用）
  final int sortOrder; // 用户自定义排序（0.4-C）；同类型内升序，0 表示未设置

  LocalAccount({
    required this.uuid,
    required this.name,
    this.displayName,
    required this.type,
    required this.openDate,
    this.closeDate,
    this.restriction,
    this.icon,
    this.color,
    this.parentUuid,
    this.subType,
    this.sortOrder = 0,
  });

  /// 是否已关闭（close_date 非空）。
  bool get isClosed => closeDate != null && closeDate!.isNotEmpty;

  /// 展示名：优先中文 display_name，回退 ASCII name。
  String get display => (displayName?.isNotEmpty == true) ? displayName! : name;

  Map<String, dynamic> toMap() => {
        'uuid': uuid,
        'name': name,
        'display_name': displayName,
        'type': type,
        'open_date': openDate,
        'close_date': closeDate,
        'commodity_restriction': restriction,
        'icon': icon,
        'color': color,
        'parent_uuid': parentUuid,
        'sub_type': subType,
        'sort_order': sortOrder,
      };

  factory LocalAccount.fromMap(Map<String, dynamic> m) => LocalAccount(
        uuid: m['uuid'] as String,
        name: m['name'] as String,
        displayName: m['display_name'] as String?,
        type: m['type'] as String,
        openDate: m['open_date'] as String,
        closeDate: m['close_date'] as String?,
        restriction: m['commodity_restriction'] as String?,
        icon: m['icon'] as String?,
        color: m['color'] as String?,
        parentUuid: m['parent_uuid'] as String?,
        subType: m['sub_type'] as String?,
        sortOrder: m['sort_order'] as int? ?? 0,
      );
}

/// 账户展示名：优先中文 display_name，回退 ASCII name。
String accountDisplayName(LocalAccount a) => a.display;
