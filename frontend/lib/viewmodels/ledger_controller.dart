/// 账本视图模型（MVVM 的 ViewModel 层）：持有账户/交易/余额与同步状态，
/// 暴露加载、同步、建账户、记账、冲正、导出等命令。视图层通过 ListenableBuilder 订阅。
import 'package:decimal/decimal.dart';
import 'package:flutter/foundation.dart';
import '../core/api/ledger_api.dart';
import '../core/api/sync_models.dart';
import '../core/models/account.dart';
import '../core/models/posting.dart';
import '../core/models/transaction.dart';
import '../core/store/local_store.dart';
import '../core/sync/sync_service.dart';

/// 同步三态：已同步 / 待同步 / 离线。对应设计稿同步徽标。
enum SyncState { synced, pending, offline }

class LedgerController extends ChangeNotifier {
  final LedgerApi api;
  final LocalStore store;
  late final SyncService sync;
  LedgerController(this.api, this.store) {
    sync = SyncService(api, store);
  }

  List<LocalAccount> accounts = [];
  List<LocalTxn> txns = [];
  Map<String, Map<String, String>> balances = {}; // uuid -> commodity -> 金额串
  Map<String, String> netWorth = {}; // commodity -> 金额串（资产 − 负债）
  Map<String, Map<String, String>> byType = {}; // type -> commodity -> 金额串
  int lastSeq = 0;
  int pending = 0;
  bool online = false;
  bool loading = true;
  bool showClosedAccounts = false; // 账户页「显示已关闭」开关（0.4-A）
  String? lastError; // 最近一次同步的 API 级错误（如 403 未加入账本）
  Set<String> reversedUuids = {};

  SyncState get syncState {
    if (!online) return SyncState.offline;
    return pending > 0 ? SyncState.pending : SyncState.synced;
  }

  /// 启动时：初始化本地库、加载本地数据、执行一次同步。
  Future<void> init() async {
    await store.init();
    await reload();
    try {
      final ok = await sync.sync();
      await reload();
      online = ok;
    } on ApiException catch (e) {
      online = false;
      lastError = e.message;
    }
    loading = false;
    notifyListeners();
  }

  /// 切换到另一个账本（P1-C 多账本）：
  /// 先尽力推送当前账本剩余离线变更，再清空本地数据，切到新账本重新拉取。
  Future<void> switchToLedger(int newLedgerId) async {
    try {
      await sync.sync();
    } catch (_) {
      // 推送失败也继续切换（离线数据可能丢失，属预期边界）。
    }
    await store.clearAll();
    LedgerApi.setLedgerId(newLedgerId);
    accounts = [];
    txns = [];
    balances = {};
    netWorth = {};
    byType = {};
    lastSeq = 0;
    pending = 0;
    online = false;
    lastError = null;
    reversedUuids = {};
    loading = true;
    notifyListeners();
    await init();
  }

  /// 从本地库重新计算所有派生数据并通知视图。
  Future<void> reload() async {
    final acc = await store.getAllAccounts();
    final tx = await store.getAllTxns();
    // 排序：先按类型固定顺序；同类型内根分类按 sort_order 升序（子分类跟随其父），
    // 未设置排序（sort_order=0）时回退按 name，保持旧库稳定展示。
    final sortOf = <String, int>{for (final a in acc) a.uuid: a.sortOrder};
    int typeIdx(String t) => typeOrder.indexOf(t);
    acc.sort((a, b) {
      final ti = typeIdx(a.type) - typeIdx(b.type);
      if (ti != 0) return ti;
      final ka = a.parentUuid != null ? (sortOf[a.parentUuid] ?? 0) : a.sortOrder;
      final kb = b.parentUuid != null ? (sortOf[b.parentUuid] ?? 0) : b.sortOrder;
      if (ka != kb) return ka.compareTo(kb);
      final sa = a.parentUuid != null ? 0 : a.sortOrder;
      final sb = b.parentUuid != null ? 0 : b.sortOrder;
      if (sa != sb) return sa.compareTo(sb);
      return a.name.compareTo(b.name);
    });
    final bal = await store.computeBalances();
    final net = await store.computeNetWorth(acc);
    final byType = await store.computeBalancesByType(acc);
    final seq = await store.getLastSeq();
    final pend = await store.countPending();
    final reversed = <String>{
      for (final t in tx)
        if (t.reversedOf != null) t.reversedOf!,
    };
    accounts = acc;
    txns = tx;
    balances = bal;
    netWorth = net;
    this.byType = byType;
    lastSeq = seq;
    pending = pend;
    reversedUuids = reversed;
    notifyListeners();
  }

  /// 手动触发同步（点击徽标 / 联网恢复时）。API 级错误记入 [lastError]，不抛出。
  Future<bool> syncNow() async {
    try {
      final ok = await sync.sync();
      await reload();
      online = ok;
      lastError = null;
      notifyListeners();
      return ok;
    } on ApiException catch (e) {
      online = false;
      lastError = e.message;
      notifyListeners();
      return false;
    }
  }

  void markOffline() {
    if (online) {
      online = false;
      notifyListeners();
    }
  }

  Future<LocalAccount> createAccount({
    required String name,
    required String type,
    required String openDate,
    String? commodity,
    String? icon,
    String? color,
    String? parentUuid,
    String? subType,
  }) async {
    final a = await sync.createAccount(
      name: name,
      type: type,
      openDate: openDate,
      commodity: commodity,
      icon: icon,
      color: color,
      parentUuid: parentUuid,
      subType: subType,
    );
    await reload();
    return a;
  }

  Future<LocalTxn> createTransaction({
    required String date,
    required String description,
    required List<LocalPosting> postings,
    List<String>? tags,
    String? reversedOf,
  }) async {
    final t = await sync.createTransaction(
      date: date,
      description: description,
      postings: postings,
      tags: tags,
      reversedOf: reversedOf,
    );
    await reload();
    return t;
  }

  /// 冲正一笔交易：新增一笔反向交易（所有分录取负），原始交易不变。
  Future<LocalTxn> reverseTxn(LocalTxn t) async {
    final postings = t.postings
        .map((p) => LocalPosting(
              accountUuid: p.accountUuid,
              commodity: p.commodity,
              amount: (-Decimal.parse(p.amount)).toString(),
            ))
        .toList();
    final r = await sync.createTransaction(
      date: t.date,
      description: '冲正: ${t.description}',
      postings: postings,
      reversedOf: t.uuid,
    );
    await reload();
    return r;
  }

  Future<String> exportText() => api.export();

  /// 导出多文件目录（main.bean + accounts/ + date/）打包的 zip 字节流。
  Future<List<int>> exportArchive() => api.exportArchive();

  // ---- 删除（P1-B1：软删 + 同步到其他端）----

  /// 删除一笔交易：本地删除 + 入待推送队列（op=delete），随后同步。
  Future<void> deleteTxn(LocalTxn t) async {
    await sync.deleteTxn(t);
    await reload();
  }

  /// 关闭账户（Beancount close 语义，0.4-A 起）：本地置 close_date + 入队 op=close，随后同步。
  /// 不再删除账户与引用交易，历史保留。
  Future<void> closeAccount(LocalAccount a) async {
    await sync.closeAccount(a);
    await reload();
  }

  /// 重排账户顺序（0.4-C 分类拖拽排序）：透传到 SyncService，本地即时重排 + 入队同步。
  Future<void> reorderAccounts(List<LocalAccount> ordered) async {
    await sync.reorderAccounts(ordered);
    await reload();
  }

  // ---- 默认账户（纯前端记忆，0.4-C，按账本隔离）----

  /// 读取默认账户映射（kind -> uuid）。kind ∈ expense|income|transferOut|transferIn。
  Future<Map<String, String>> get defaultAccounts =>
      store.getDefaultAccounts(LedgerApi.ledgerId);

  /// 设置某类交易的默认账户 uuid（并持久化）。
  Future<void> setDefaultAccount(String kind, String uuid) =>
      store.setDefaultAccount(LedgerApi.ledgerId, kind, uuid);

  /// 清除当前账本的全部默认账户设置。
  Future<void> clearDefaultAccounts() =>
      store.clearDefaultAccounts(LedgerApi.ledgerId);

  /// 切换「显示已关闭」开关（仅影响账户页展示，不影响余额/净资产计算）。
  void toggleShowClosedAccounts(bool v) {
    showClosedAccounts = v;
    notifyListeners();
  }

  // ---- 成员管理（P1-B3，仅 owner）----

  Future<List<Member>> loadMembers() => api.listMembers();

  Future<void> addMember(String username, String role) =>
      api.addMember(username, role);

  Future<void> updateMemberRole(int userId, String role) =>
      api.updateMemberRole(userId, role);

  Future<void> removeMember(int userId) => api.removeMember(userId);

  Future<void> resetMemberPassword(int userId, String newPassword) =>
      api.resetMemberPassword(userId, newPassword);

  String accountName(String uuid) {
    final a = accounts.where((x) => x.uuid == uuid).firstWhere(
          (x) => x.uuid == uuid,
          orElse: () => LocalAccount(
              uuid: uuid, name: uuid, type: '', openDate: ''),
        );
    return a.display;
  }

  /// 按类型筛选账户（保持固定顺序：资产/负债/权益/收入/支出）。
  /// includeClosed=false（默认）仅返回未关闭账户；true 含已关闭（0.4-A）。
  List<LocalAccount> accountsOfType(String type, {bool includeClosed = false}) =>
      accounts
          .where((a) => a.type == type && (includeClosed || !a.isClosed))
          .toList();

  static const typeOrder = [
    'Assets',
    'Liabilities',
    'Equity',
    'Income',
    'Expenses',
  ];

  /// 交易在某账户上的净值（用于列表行尾展示）。
  String? txnHeadline(LocalTxn t) {
    if (t.postings.isEmpty) return null;
    final p = t.postings.first;
    return p.amount;
  }

  /// 交易涉及账户的有向摘要，如 "Cash → Food"。
  String txnFlow(LocalTxn t) {
    final names = t.postings.map((p) => accountName(p.accountUuid)).toList();
    if (names.length < 2) return names.isEmpty ? '' : names.first;
    return '${names.first} → ${names.last}';
  }
}
