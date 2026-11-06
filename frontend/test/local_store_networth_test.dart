// 本地净值算式与双算校验测试（FR-13 / FR-14 / TC-13 / TC-15）。
// 使用内存 sembast（databaseFactoryMemory），不依赖浏览器 IndexedDB。
import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/models/account.dart';
import 'package:frontend/core/models/posting.dart';
import 'package:frontend/core/models/transaction.dart';
import 'package:frontend/core/store/local_store.dart';
import 'package:sembast/sembast_memory.dart';

Database? _db;
LocalStore? _store;

LocalAccount _account(String uuid, String type) => LocalAccount(
      uuid: uuid,
      name: 'Test:$uuid',
      type: type,
      openDate: '2026-01-01',
    );

/// 写入一组账户与一笔覆盖全部 postings 的交易（金额为定点十进制字符串）。
Future<void> _seed(List<LocalAccount> accounts, List<LocalPosting> postings) async {
  for (final a in accounts) {
    await _store!.putAccount(a);
  }
  await _store!.putTxn(LocalTxn(
    uuid: 'txn-1',
    date: '2026-01-01',
    flag: '*',
    description: '期初',
    postings: postings,
  ));
}

void main() {
  setUp(() async {
    _db = await databaseFactoryMemory.openDatabase(
        'networth_${DateTime.now().microsecondsSinceEpoch}');
    _store = LocalStore.forTesting(_db!);
  });

  tearDown(() async {
    await _db!.close();
    _db = null;
    _store = null;
  });

  test('资产 100 + 负债 -20 → 净资产 80（负债不再被乘 -1）', () async {
    final accounts = [
      _account('assets', 'Assets'),
      _account('liabs', 'Liabilities'),
      _account('equity', 'Equity'),
    ];
    await _seed(accounts, [
      LocalPosting(accountUuid: 'assets', commodity: 'CNY', amount: '100.00'),
      LocalPosting(accountUuid: 'liabs', commodity: 'CNY', amount: '-20.00'),
      LocalPosting(accountUuid: 'equity', commodity: 'CNY', amount: '-80.00'),
    ]);

    final net = await _store!.computeNetWorth(await _store!.getAllAccounts());
    expect(net['CNY'], '80.00');
  });

  test('资产 -25 + 负债 -20 → 净资产 -45（负净值如实保留负号）', () async {
    final accounts = [
      _account('assets', 'Assets'),
      _account('liabs', 'Liabilities'),
      _account('equity', 'Equity'),
    ];
    await _seed(accounts, [
      LocalPosting(accountUuid: 'assets', commodity: 'CNY', amount: '-25.00'),
      LocalPosting(accountUuid: 'liabs', commodity: 'CNY', amount: '-20.00'),
      LocalPosting(accountUuid: 'equity', commodity: 'CNY', amount: '45.00'),
    ]);

    final net = await _store!.computeNetWorth(await _store!.getAllAccounts());
    expect(net['CNY'], '-45.00');
  });

  test('净值双算一致：netA（资产+负债）与 netB（-(收入+权益)-支出）差值为 0', () async {
    final accounts = [
      _account('assets', 'Assets'),
      _account('liabs', 'Liabilities'),
      _account('equity', 'Equity'),
      _account('income', 'Income'),
      _account('expense', 'Expenses'),
    ];
    // 复式平衡：资产 100、负债 -20、权益 -90、收入 -5、支出 +15，五类合计 0。
    await _seed(accounts, [
      LocalPosting(accountUuid: 'assets', commodity: 'CNY', amount: '100.00'),
      LocalPosting(accountUuid: 'liabs', commodity: 'CNY', amount: '-20.00'),
      LocalPosting(accountUuid: 'equity', commodity: 'CNY', amount: '-90.00'),
      LocalPosting(accountUuid: 'income', commodity: 'CNY', amount: '-5.00'),
      LocalPosting(accountUuid: 'expense', commodity: 'CNY', amount: '15.00'),
    ]);

    final audit = await _store!.computeNetWorthAudit(await _store!.getAllAccounts());
    expect(audit['CNY'], '0.00');
    for (final v in audit.values) {
      expect(Decimal.tryParse(v), Decimal.zero);
    }
  });

  test('无交易时净值为空映射', () async {
    final net = await _store!.computeNetWorth(await _store!.getAllAccounts());
    expect(net, isEmpty);
    final audit = await _store!.computeNetWorthAudit(await _store!.getAllAccounts());
    expect(audit, isEmpty);
  });
}
