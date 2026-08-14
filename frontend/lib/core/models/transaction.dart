/// 本地交易模型（复式记账核心）。交易不可变：改账 = 新增冲正交易。
import 'posting.dart';

class LocalTxn {
  final String uuid;
  final String date;
  final String flag; // '*' 已结清 / '!' 待核对
  final String description;
  final List<LocalPosting> postings;
  final String? reversedOf; // 若为冲正交易，记录被冲正交易的 uuid（仅本地/展示用）

  LocalTxn({
    required this.uuid,
    required this.date,
    required this.flag,
    required this.description,
    required this.postings,
    this.reversedOf,
  });

  Map<String, dynamic> toMap() => {
        'uuid': uuid,
        'date': date,
        'flag': flag,
        'description': description,
        'postings': postings.map((p) => p.toMap()).toList(),
        if (reversedOf != null) 'reversed_of': reversedOf,
      };

  factory LocalTxn.fromMap(Map<String, dynamic> m) => LocalTxn(
        uuid: m['uuid'] as String,
        date: m['date'] as String,
        flag: m['flag'] as String,
        description: m['description'] as String,
        reversedOf: m['reversed_of'] as String?,
        postings: (m['postings'] as List)
            .map((e) => LocalPosting.fromMap(e as Map<String, dynamic>))
            .toList(),
      );
}
