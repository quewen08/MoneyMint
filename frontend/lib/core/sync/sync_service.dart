/// 同步编排：pull 合并云端变更到本地，push 把离线产生的待推送队列发往云端。
/// 离线时写入本地即成功；联网后下次 sync() 自动补推。
/// P1-B2：push 逐条处理（单条失败不阻塞其余，results 逐条反馈）；
/// 自然键合并：账户同名冲突时保留先到者，用 server_uuid 重映射本地引用。
import '../api/ledger_api.dart';
import '../models/account.dart';
import '../models/posting.dart';
import '../models/transaction.dart';
import '../models/pending_change.dart';
import '../store/local_store.dart';

class SyncService {
  final LedgerApi api;
  final LocalStore store;
  SyncService(this.api, this.store);

  /// 全量同步：先拉 → 推本地队列（逐条，自然键合并重映射）→ 再拉合并服务端事件。
  /// 返回是否联网成功；网络失败返回 false；API 级错误（401/403/400）向上抛给 UI。
  Future<bool> sync() async {
    final clientId = await store.getClientId();
    try {
      await _pull(clientId);
      if ((await store.getPendingEntries()).isNotEmpty) {
        await _pushLoop(clientId);
      }
      await _pull(clientId); // 合并 push 后服务端写入/删除事件（含级联删除）
      return true;
    } on ApiException {
      rethrow;
    } catch (_) {
      return false; // 断网：保留本地数据，待下次同步。
    }
  }

  /// 拉取并应用服务端自水位以来的变更，推进本地水位。
  Future<void> _pull(String clientId) async {
    final lastSeq = await store.getLastSeq();
    final resp = await api.pull(lastSeq, clientId);
    for (final c in resp.changes) {
      await store.applyChange(c.entityType, c.op, c.entity);
    }
    if (resp.checkpoint > lastSeq) {
      await store.setLastSeq(resp.checkpoint);
    }
  }

  /// 推送待推送队列。最多两轮：
  /// 第一轮收集自然键合并（server_uuid）并重映射本地引用；第二轮重推修正后的条目。
  /// 成功后按逐条结果移除对应 pending；永久失败的条目保留待下次重试。
  Future<void> _pushLoop(String clientId) async {
    for (var round = 0; round < 2; round++) {
      final entries = await store.getPendingEntries();
      if (entries.isEmpty) return;
      final lastSeq = await store.getLastSeq();
      final resp = await api.push(
        clientId,
        lastSeq,
        entries.map((e) => e.change.toPushMap()).toList(),
      );

      // 逐条结果：收集合并映射 + 待移除条目。
      final uuidMap = <String, String>{};
      final removedKeys = <int>[];
      for (final r in resp.results) {
        final su = r.serverUuid;
        if (su != null && su.isNotEmpty && r.uuid.isNotEmpty && su != r.uuid) {
          uuidMap[r.uuid] = su;
        }
        if (r.ok && r.index >= 0 && r.index < entries.length) {
          removedKeys.add(entries[r.index].key);
        }
      }

      // 自然键合并：本地账户 uuid → server_uuid，并重写剩余 pending 的引用。
      for (final m in uuidMap.entries) {
        await store.remapAccountUuid(m.key, m.value);
        await _rewritePendingRefs(entries, removedKeys, m.key, m.value);
      }

      if (removedKeys.isNotEmpty) {
        await store.removePendingKeys(removedKeys);
      }
      if (resp.checkpoint > lastSeq) {
        await store.setLastSeq(resp.checkpoint);
      }
      if (resp.results.every((r) => r.ok)) return;
    }
  }

  /// 把 pending 中尚未移除条目的实体引用从 [oldUuid] 改写为 [newUuid]。
  Future<void> _rewritePendingRefs(
    List<PendingEntry> entries,
    List<int> removedKeys,
    String oldUuid,
    String newUuid,
  ) async {
    for (final e in entries) {
      if (removedKeys.contains(e.key)) continue;
      if (e.change.entityType != 'transaction') continue;
      final postings = e.change.entity['postings'];
      if (postings is! List) continue;
      var changed = false;
      final newPostings = postings.map((p) {
        final pm = Map<String, dynamic>.from(p as Map<String, dynamic>);
        if (pm['account_uuid'] == oldUuid) {
          pm['account_uuid'] = newUuid;
          changed = true;
        }
        return pm;
      }).toList();
      if (!changed) continue;
      final entity = Map<String, dynamic>.from(e.change.entity);
      entity['postings'] = newPostings;
      await store.updatePendingEntity(
        e.key,
        PendingChange(
          entityType: e.change.entityType,
          op: e.change.op,
          key: e.change.key,
          entity: entity,
        ),
      );
    }
  }

  /// 本地创建账户：生成 uuid，落本地，入待推送队列，随后尝试同步。
  Future<LocalAccount> createAccount({
    required String name,
    required String type,
    required String openDate,
    String? commodity,
  }) async {
    final a = LocalAccount(
      uuid: _newUuid(),
      name: name,
      type: type,
      openDate: openDate,
      restriction: commodity,
    );
    await store.putAccount(a);
    await store.addPending(PendingChange(
      entityType: 'account',
      key: a.uuid,
      entity: a.toMap(),
    ));
    await sync();
    return a;
  }

  /// 本地记账：生成 uuid，postings 引用账户 uuid，落本地，入待推送队列。
  /// [reversedOf] 用于标记这是某笔交易的冲正（仅本地/展示用，后端忽略未知字段）。
  Future<LocalTxn> createTransaction({
    required String date,
    required String description,
    required List<LocalPosting> postings,
    String? reversedOf,
  }) async {
    final t = LocalTxn(
      uuid: _newUuid(),
      date: date,
      flag: '*',
      description: description,
      postings: postings,
      reversedOf: reversedOf,
    );
    await store.putTxn(t);
    await store.addPending(PendingChange(
      entityType: 'transaction',
      key: t.uuid,
      entity: t.toMap(),
    ));
    await sync();
    return t;
  }

  /// 删除一笔交易（软删）：本地删除 + 入待推送队列（op=delete）后同步。
  /// 服务器软删并写 sync_log，其他端经 pull 收到 delete 事件。
  Future<void> deleteTxn(LocalTxn t) async {
    await store.deleteTxn(t.uuid);
    await store.addPending(PendingChange(
      entityType: 'transaction',
      op: 'delete',
      key: t.uuid,
      entity: {'uuid': t.uuid},
    ));
    await sync();
  }

  /// 删除账户（软删，连带其引用交易）：
  /// 本地删除账户 + 引用交易（与服务器级联语义一致），入待推送队列后同步。
  /// 服务器生成账户 + 各交易 delete 事件，其他端经 pull 同步清理。
  Future<void> deleteAccount(LocalAccount a) async {
    await store.deleteAccount(a.uuid);
    final txns = await store.getAllTxns();
    for (final t in txns) {
      if (t.postings.any((p) => p.accountUuid == a.uuid)) {
        await store.deleteTxn(t.uuid);
      }
    }
    await store.addPending(PendingChange(
      entityType: 'account',
      op: 'delete',
      key: a.uuid,
      entity: {'uuid': a.uuid},
    ));
    await sync();
  }

  String _newUuid() =>
      'loc-${DateTime.now().microsecondsSinceEpoch}-${(_counter++).toString()}';
  int _counter = 0;
}
