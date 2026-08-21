/// 本地存储（IndexedDB via sembast_web）。前端离线时的真相源。
/// 账户/交易均以 uuid 为主键；另存同步水位与待推送队列。
import 'package:decimal/decimal.dart';
import 'package:sembast_web/sembast_web.dart';
import '../models/account.dart';
import '../models/posting.dart';
import '../models/transaction.dart';
import '../models/pending_change.dart';

class LocalStore {
  static final LocalStore _instance = LocalStore._();
  factory LocalStore() => _instance;
  LocalStore._();

  late Database _db;
  bool _ready = false;

  final _accounts = stringMapStoreFactory.store('accounts');
  final _txns = stringMapStoreFactory.store('txns');
  final _meta = stringMapStoreFactory.store('meta');
  final _pending = intMapStoreFactory.store('pending');

  Future<void> init() async {
    if (_ready) return;
    final factory = databaseFactoryWeb;
    _db = await factory.openDatabase('familyledger.db');
    _ready = true;
  }

  // ---- 账户 ----
  Future<void> putAccount(LocalAccount a) async {
    await _accounts.record(a.uuid).put(_db, a.toMap());
  }

  Future<List<LocalAccount>> getAllAccounts() async {
    final snaps = await _accounts.find(_db);
    return snaps.map((s) => LocalAccount.fromMap(s.value)).toList();
  }

  Future<LocalAccount?> getAccount(String uuid) async {
    final rec = await _accounts.record(uuid).get(_db);
    return rec == null ? null : LocalAccount.fromMap(rec);
  }

  Future<void> deleteAccount(String uuid) async {
    await _accounts.record(uuid).delete(_db);
  }

  /// 账户 uuid 重映射（自然键合并后）：把 [oldUuid] 换成服务器真实 [newUuid]，
  /// 并同步更新所有引用它的交易的 postings。
  Future<void> remapAccountUuid(String oldUuid, String newUuid) async {
    final rec = await _accounts.record(oldUuid).get(_db);
    if (rec == null) return;
    await _accounts.record(oldUuid).delete(_db);
    await _accounts
        .record(newUuid)
        .put(_db, {...Map<String, dynamic>.from(rec), 'uuid': newUuid});
    final txns = await getAllTxns();
    for (final t in txns) {
      if (!t.postings.any((p) => p.accountUuid == oldUuid)) continue;
      final postings = t.postings
          .map((p) => p.accountUuid == oldUuid
              ? LocalPosting(
                  accountUuid: newUuid,
                  commodity: p.commodity,
                  amount: p.amount,
                )
              : p)
          .toList();
      await putTxn(LocalTxn(
        uuid: t.uuid,
        date: t.date,
        flag: t.flag,
        description: t.description,
        postings: postings,
        reversedOf: t.reversedOf,
      ));
    }
  }

  // ---- 交易 ----
  Future<void> putTxn(LocalTxn t) async {
    await _txns.record(t.uuid).put(_db, t.toMap());
  }

  Future<List<LocalTxn>> getAllTxns() async {
    final snaps = await _txns.find(_db,
        finder: Finder(sortOrders: [SortOrder('date', false)]));
    return snaps.map((s) => LocalTxn.fromMap(s.value)).toList();
  }

  Future<void> deleteTxn(String uuid) async {
    await _txns.record(uuid).delete(_db);
  }

  // ---- 同步水位 ----
  Future<int> getLastSeq() async {
    final v = await _meta.record('last_seq').get(_db);
    return v == null ? 0 : (v['seq'] as int? ?? 0);
  }

  Future<void> setLastSeq(int seq) async {
    await _meta.record('last_seq').put(_db, {'seq': seq});
  }

  // ---- 设备 id（稳定，用于服务端记录水位）----
  Future<String> getClientId() async {
    final v = await _meta.record('client_id').get(_db);
    if (v != null) return v['id'] as String;
    final id = 'web-${DateTime.now().microsecondsSinceEpoch}';
    await _meta.record('client_id').put(_db, {'id': id});
    return id;
  }

  // ---- 待推送队列 ----
  Future<void> addPending(PendingChange c) async {
    await _pending.add(_db, c.toMap());
  }

  /// 读全部待推送条目（带各自记录 key，用于逐条移除/重写）。
  Future<List<PendingEntry>> getPendingEntries() async {
    final snaps = await _pending.find(_db);
    snaps.sort((a, b) => a.key.compareTo(b.key));
    return snaps
        .map((s) => PendingEntry(s.key, PendingChange.fromMap(s.value)))
        .toList();
  }

  /// 重写一条待推送条目（自然键合并后修正引用）。
  Future<void> updatePendingEntity(int key, PendingChange c) async {
    await _pending.record(key).put(_db, c.toMap());
  }

  /// 按记录 key 批量移除（push 成功后逐条清除，P1-B2）。
  Future<void> removePendingKeys(List<int> keys) async {
    for (final k in keys) {
      await _pending.record(k).delete(_db);
    }
  }

  Future<void> clearPending() async {
    await _pending.delete(_db);
  }

  /// 清空本地账本数据（账户/交易/待推送队列/水位），
  /// 用于多账本切换时避免不同账本数据串库（P1-C）。
  /// 保留 client_id（设备标识稳定），仅清 last_seq 以便切换后重拉全量。
  Future<void> clearAll() async {
    await _accounts.delete(_db);
    await _txns.delete(_db);
    await _pending.delete(_db);
    await _meta.record('last_seq').delete(_db);
  }

  /// 待推送队列长度（用于 UI 显示离线积压）。
  Future<int> countPending() async {
    final snaps = await _pending.find(_db);
    return snaps.length;
  }

  /// 默认账户（纯前端记忆，0.4-C）：按账本隔离，kind ∈
  /// expense | income | transferOut | transferIn，值为账户 uuid。
  /// 记一笔弹窗按交易类型预选默认账户，提交时回写「上次选择」。
  Future<Map<String, String>> getDefaultAccounts(int? ledgerId) async {
    final v = await _meta.record('default_accounts_${ledgerId ?? 0}').get(_db);
    if (v == null) return {};
    final map = Map<String, dynamic>.from(v);
    return map.map((k, val) => MapEntry(k, val as String));
  }

  Future<void> setDefaultAccount(int? ledgerId, String kind, String uuid) async {
    final cur = await getDefaultAccounts(ledgerId);
    cur[kind] = uuid;
    await _meta.record('default_accounts_${ledgerId ?? 0}').put(_db, cur);
  }

  Future<void> clearDefaultAccounts(int? ledgerId) async {
    await _meta.record('default_accounts_${ledgerId ?? 0}').delete(_db);
  }

  /// 应用一条服务器拉回的变更：
  ///   op=create → 按 uuid 幂等 upsert；
  ///   op=delete → 按 uuid 删除本地记录（仅交易；账户 0.4 起改为 close）；
  ///   op=close  → 按 uuid 更新账户 close_date（不删除，保留历史交易，0.4-A）；
  ///   op=update → 按 uuid 全量 upsert（0.4-C 账户排序等字段更新）。
  Future<void> applyChange(
      String entityType, String op, Map<String, dynamic> entity) async {
    final uuid = entity['uuid'] as String?;
    if (op == 'update') {
      // 账户字段更新（当前为 sort_order）：按 uuid 全量 upsert即可。
      if (entityType == 'account' && uuid != null) {
        await putAccount(LocalAccount.fromMap(entity));
      }
      return;
    }
    if (op == 'close') {
      // 账户关闭：更新 close_date，保留账户与引用交易。
      if (entityType == 'account' && uuid != null) {
        final rec = await _accounts.record(uuid).get(_db);
        if (rec != null) {
          await _accounts.record(uuid).put(_db,
              {...Map<String, dynamic>.from(rec), 'close_date': entity['close_date']});
        }
      }
      return;
    }
    if (op == 'delete') {
      if (entityType == 'account' && uuid != null) {
        // 0.4 起账户不再 delete；兼容旧 pull 数据按 close 处理。
        final rec = await _accounts.record(uuid).get(_db);
        if (rec != null) {
          await _accounts.record(uuid).put(_db,
              {...Map<String, dynamic>.from(rec), 'close_date': entity['close_date'] ?? ''});
        } else {
          await deleteAccount(uuid);
        }
      } else if (entityType == 'transaction' && uuid != null) {
        await deleteTxn(uuid);
      }
      return;
    }
    if (entityType == 'account') {
      await putAccount(LocalAccount.fromMap(entity));
    } else if (entityType == 'transaction') {
      await putTxn(LocalTxn.fromMap(entity));
    }
    // commodity 为派生实体，忽略。
  }

  /// 本地计算各账户余额：map[uuid] -> map[commodity] -> 金额字符串。
  /// 使用 Decimal 精确累加，遵守「禁止浮点参与金额运算」约定。
  Future<Map<String, Map<String, String>>> computeBalances() async {
    final sums = await _balancesDecimal();
    return sums.map((uuid, m) => MapEntry(
        uuid, m.map((c, v) => MapEntry(c, v.toStringAsFixed(2)))));
  }

  /// 净资产（设计稿语义：资产 − 负债），按币种汇总。
  /// map[commodity] -> 金额字符串。用于首页大字号净值。
  Future<Map<String, String>> computeNetWorth(List<LocalAccount> accounts) async {
    final sums = await _balancesDecimal();
    final net = <String, Decimal>{};
    for (final a in accounts) {
      if (a.type != 'Assets' && a.type != 'Liabilities') continue;
      final m = sums[a.uuid];
      if (m == null) continue;
      final sign = Decimal.fromInt(a.type == 'Assets' ? 1 : -1);
      m.forEach((c, v) {
        net[c] = (net[c] ?? Decimal.zero) + v * sign;
      });
    }
    return net.map((c, v) => MapEntry(c, v.toStringAsFixed(2)));
  }

  /// 按账户类型汇总余额：map[type] -> map[commodity] -> 金额字符串。
  /// 用于报表页/首页的分类汇总。
  Future<Map<String, Map<String, String>>> computeBalancesByType(
      List<LocalAccount> accounts) async {
    final sums = await _balancesDecimal();
    final byType = <String, Map<String, Decimal>>{};
    for (final a in accounts) {
      final m = sums[a.uuid];
      if (m == null) continue;
      byType.putIfAbsent(a.type, () => {});
      m.forEach((c, v) {
        byType[a.type]![c] = (byType[a.type]![c] ?? Decimal.zero) + v;
      });
    }
    return byType.map((t, m) => MapEntry(
        t, m.map((c, v) => MapEntry(c, v.toStringAsFixed(2)))));
  }

  /// 内部：以 Decimal 累加每个账户每币种的余额（未格式化）。
  Future<Map<String, Map<String, Decimal>>> _balancesDecimal() async {
    final txns = await getAllTxns(); // 已按 date 倒序，但累加与顺序无关
    final sums = <String, Map<String, Decimal>>{};
    for (final t in txns) {
      for (final p in t.postings) {
        final amt = Decimal.tryParse(p.amount) ?? Decimal.zero;
        sums.putIfAbsent(p.accountUuid, () => {});
        sums[p.accountUuid]![p.commodity] =
            (sums[p.accountUuid]![p.commodity] ?? Decimal.zero) + amt;
      }
    }
    return sums;
  }
}

/// 待推送条目（记录 key + 变更内容）。key 为 sembast 自动整数主键。
class PendingEntry {
  final int key;
  final PendingChange change;
  PendingEntry(this.key, this.change);
}
