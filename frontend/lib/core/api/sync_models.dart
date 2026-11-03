/// 同步协议传输模型（pull/push 响应）+ 成员模型。

class SyncChange {
  final int seq;
  final String entityType;
  final String op;
  final Map<String, dynamic> entity;

  SyncChange(this.seq, this.entityType, this.op, this.entity);

  factory SyncChange.fromJson(Map<String, dynamic> j) => SyncChange(
        (j['seq'] as num).toInt(),
        j['entity_type'] as String,
        j['op'] as String,
        j['entity'] as Map<String, dynamic>,
      );
}

class PullResp {
  final int checkpoint;
  final List<SyncChange> changes;
  PullResp(this.checkpoint, this.changes);

  factory PullResp.fromJson(Map<String, dynamic> j) => PullResp(
        (j['checkpoint'] as num).toInt(),
        (j['changes'] as List)
            .map((e) => SyncChange.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}

/// push 中一条变更的逐条结果（P1-B2：单条失败不阻塞其余）。
class PushResult {
  final int index;
  final String entityType;
  final String uuid;
  final String status; // ok | error
  final bool accepted;
  final String? serverUuid; // 自然键合并后的真实 uuid（仅发生合并时非空）
  final String? error;

  PushResult({
    required this.index,
    required this.entityType,
    required this.uuid,
    required this.status,
    required this.accepted,
    this.serverUuid,
    this.error,
  });

  bool get ok => status == 'ok';

  factory PushResult.fromJson(Map<String, dynamic> j) => PushResult(
        index: (j['index'] as num).toInt(),
        entityType: j['entity_type'] as String,
        uuid: j['uuid'] as String? ?? '',
        status: j['status'] as String,
        accepted: j['accepted'] as bool? ?? false,
        serverUuid: j['server_uuid'] as String?,
        error: j['error'] as String?,
      );
}

class PushResp {
  final int checkpoint;
  final int accepted;
  final List<PushResult> results;
  PushResp(this.checkpoint, this.accepted, this.results);

  factory PushResp.fromJson(Map<String, dynamic> j) => PushResp(
        (j['checkpoint'] as num).toInt(),
        (j['accepted'] as num).toInt(),
        (j['results'] as List)
            .map((e) => PushResult.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}

/// 账本（P1-C 多账本）：用户所属账本及其角色。
class Ledger {
  final int id;
  final String name;
  final String defaultCommodity;
  final String role; // owner | editor | viewer

  Ledger({
    required this.id,
    required this.name,
    this.defaultCommodity = '',
    required this.role,
  });

  factory Ledger.fromJson(Map<String, dynamic> j) => Ledger(
        id: (j['id'] as num).toInt(),
        name: j['name'] as String,
        defaultCommodity: j['default_commodity'] as String? ?? '',
        role: j['role'] as String? ?? '',
      );
}

/// 账本成员（P1-B3 成员管理）。
class Member {
  final int userId;
  final String username;
  final String displayName;
  final String role; // owner | editor | viewer

  Member({
    required this.userId,
    required this.username,
    required this.displayName,
    required this.role,
  });

  factory Member.fromJson(Map<String, dynamic> j) => Member(
        userId: (j['user_id'] as num).toInt(),
        username: j['username'] as String,
        displayName: j['display_name'] as String? ?? '',
        role: j['role'] as String,
      );

  static String roleLabel(String role) => switch (role) {
        'owner' => '所有者',
        'editor' => '编辑',
        'viewer' => '只读',
        _ => role,
      };
}

/// 一份账本备份的元信息（对应服务端 manifest.json 的一条记录）。
///
/// 备份是全库级产物（含所有账本与成员），仅系统管理员可查看与下载。
class BackupItem {
  final String name;
  final int size;
  final String sha256;
  final String trigger; // auto | manual
  final DateTime createdAt;
  final int durationMs;
  final Map<String, int> stats;
  final String? error;

  BackupItem({
    required this.name,
    required this.size,
    required this.sha256,
    required this.trigger,
    required this.createdAt,
    this.durationMs = 0,
    this.stats = const {},
    this.error,
  });

  /// 是否是一次成功的备份（失败记录只用于排查，不提供下载）。
  bool get succeeded => error == null || error!.isEmpty;

  bool get isManual => trigger == 'manual';

  /// 易读体积（KB / MB）。
  String get sizeLabel {
    if (size >= 1024 * 1024) {
      return '${(size / 1024 / 1024).toStringAsFixed(1)} MB';
    }
    return '${(size / 1024).toStringAsFixed(0)} KB';
  }

  factory BackupItem.fromJson(Map<String, dynamic> j) {
    final rawStats = j['stats'];
    var stats = <String, int>{};
    if (rawStats is Map) {
      rawStats.forEach((k, v) {
        if (v is num) stats[k.toString()] = v.toInt();
      });
    }
    return BackupItem(
      name: j['name'] as String? ?? '',
      size: (j['size'] as num?)?.toInt() ?? 0,
      sha256: j['sha256'] as String? ?? '',
      trigger: j['trigger'] as String? ?? '',
      createdAt: DateTime.tryParse(j['created_at'] as String? ?? '')?.toLocal() ??
          DateTime.fromMillisecondsSinceEpoch(0),
      durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
      stats: stats,
      error: j['error'] as String?,
    );
  }
}
