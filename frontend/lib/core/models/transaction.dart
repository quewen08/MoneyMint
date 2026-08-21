/// 本地交易模型（复式记账核心）。交易不可变：改账 = 新增冲正交易。
import 'posting.dart';

class LocalTxn {
  final String uuid;
  final String date;
  final String flag; // '*' 已结清 / '!' 待核对
  final String description;
  final List<LocalPosting> postings;
  final List<String> tags; // 标签（展示/检索/导出 #tag 用）
  final String? reversedOf; // 若为冲正交易，记录被冲正交易的 uuid（仅本地/展示用）
  final String? createdByName; // 记账人显示名；pull 同步回或本地新建时回填

  LocalTxn({
    required this.uuid,
    required this.date,
    required this.flag,
    required this.description,
    required this.postings,
    this.tags = const [],
    this.reversedOf,
    this.createdByName,
  });

  Map<String, dynamic> toMap() => {
        'uuid': uuid,
        'date': date,
        'flag': flag,
        'description': description,
        'postings': postings.map((p) => p.toMap()).toList(),
        'tags': tags,
        if (reversedOf != null) 'reversed_of': reversedOf,
        if (createdByName != null) 'created_by_name': createdByName,
      };

  factory LocalTxn.fromMap(Map<String, dynamic> m) {
    final rawTags = m['tags'];
    final List<String> tags = rawTags is List
        ? rawTags.map((e) => e.toString()).toList()
        : const <String>[];
    return LocalTxn(
      uuid: m['uuid'] as String,
      date: m['date'] as String,
      flag: m['flag'] as String,
      description: m['description'] as String,
      reversedOf: m['reversed_of'] as String?,
      tags: tags,
      createdByName: m['created_by_name'] as String?,
      postings: (m['postings'] as List)
          .map((e) => LocalPosting.fromMap(e as Map<String, dynamic>))
          .toList(),
    );
  }
}
