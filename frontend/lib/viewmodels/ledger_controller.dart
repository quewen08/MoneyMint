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
  }) async {
    final a = await sync.createAccount(
      name: name,
      type: type,
      openDate: openDate,
      commodity: commodity,
    );
    await reload();
    return a;
  }

  Future<LocalTxn> createTransaction({
    required String date,
    required String description,
    required List<LocalPosting> postings,
    String? reversedOf,
  }) async {
    final t = await sync.createTransaction(
      date: date,
      description: description,
      postings: postings,
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

  /// 删除账户（连带其引用交易）：本地删除 + 入待推送队列，随后同步。
  Future<void> deleteAccount(LocalAccount a) async {
    await sync.deleteAccount(a);
    await reload();
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
  List<LocalAccount> accountsOfType(String type) =>
      accounts.where((a) => a.type == type).toList();

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
