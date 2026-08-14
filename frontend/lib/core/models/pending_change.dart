/// 待推送的本地变更（离线产生，联网后清空）。
/// [op] 区分 create（建账户/记账）与 delete（删除账户/交易），默认 create。
/// [key] 为实体 uuid：create 时与 entity['uuid'] 一致；delete 时实体仅含 uuid。
class PendingChange {
  final String entityType; // account | transaction
  final String op; // create | delete
  final String key; // 实体 uuid
  final Map<String, dynamic> entity;

  PendingChange({
    required this.entityType,
    this.op = 'create',
    required this.key,
    required this.entity,
  });

  Map<String, dynamic> toMap() => {
        'entity_type': entityType,
        'op': op,
        'key': key,
        'entity': entity,
      };

  /// 发给后端的变更负载（后端只认 entity_type/op/entity）。
  Map<String, dynamic> toPushMap() => {
        'entity_type': entityType,
        'op': op,
        'entity': entity,
      };

  /// 兼容旧版队列数据（无 op/key 时按 create 处理，key 取 entity.uuid）。
  factory PendingChange.fromMap(Map<String, dynamic> m) {
    final entity = Map<String, dynamic>.from(m['entity'] as Map);
    return PendingChange(
      entityType: m['entity_type'] as String,
      op: m['op'] as String? ?? 'create',
      key: m['key'] as String? ?? (entity['uuid'] as String? ?? ''),
      entity: entity,
    );
  }
}
