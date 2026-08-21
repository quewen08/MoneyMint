/// 后端 REST 客户端（认证 + 同步协议 + 删除 + 成员管理 + 导出）。
/// 数据写入走 /api/sync/push，拉取走 /api/sync/pull；删除走 REST DELETE；
/// 导出走 /api/export。所有请求自动附带登录后拿到的 Bearer token（持久化于 localStorage）。
import 'dart:convert';
import 'dart:html' as html;
import 'package:http/http.dart' as http;
import 'sync_models.dart';

class LedgerApi {
  final String base;
  LedgerApi([this.base = kDefaultApiBase]);

  // ---- 登录态（token 持久化于 localStorage，刷新后保持）----
  static const _tokenKey = 'fl_token';
  static String? _token;
  static bool get loggedIn => _token != null && _token!.isNotEmpty;

  /// 启动时从 localStorage 恢复 token。
  static Future<void> loadToken() async {
    _token = html.window.localStorage[_tokenKey];
  }

  /// 保存登录拿到的 token。
  static void setToken(String t) {
    _token = t;
    html.window.localStorage[_tokenKey] = t;
  }

  /// 清除 token（安全退出）。
  static void clearToken() {
    _token = null;
    html.window.localStorage.remove(_tokenKey);
  }

  // ---- 当前账本上下文（P1-C 多账本，持久化于 localStorage）----
  static const _ledgerKey = 'fl_ledger_id';
  static int? _ledgerId;

  /// 当前账本 id（用于业务请求携带 X-Ledger-Id）。
  static int? get ledgerId => _ledgerId;

  /// 设置当前账本 id 并持久化。
  static void setLedgerId(int? id) {
    _ledgerId = id;
    if (id == null) {
      html.window.localStorage.remove(_ledgerKey);
    } else {
      html.window.localStorage[_ledgerKey] = id.toString();
    }
  }

  /// 启动时从 localStorage 恢复当前账本 id。
  static Future<void> loadLedgerId() async {
    final v = html.window.localStorage[_ledgerKey];
    if (v != null && v.isNotEmpty) {
      _ledgerId = int.tryParse(v);
    }
  }

  // ---- 当前用户显示名（用于离线记账时回填「谁记的」，持久化于 localStorage）----
  static const _displayNameKey = 'fl_display_name';
  static String? _displayName;

  /// 当前用户显示名；无则为 null。
  static String? get displayName => _displayName;

  /// 设置当前用户显示名并持久化。
  static void setDisplayName(String? name) {
    _displayName = name;
    if (name == null || name.isEmpty) {
      html.window.localStorage.remove(_displayNameKey);
    } else {
      html.window.localStorage[_displayNameKey] = name;
    }
  }

  /// 启动时从 localStorage 恢复当前用户显示名。
  static Future<void> loadDisplayName() async {
    final v = html.window.localStorage[_displayNameKey];
    _displayName = v != null && v.isNotEmpty ? v : null;
  }

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (_token != null) 'Authorization': 'Bearer $_token',
        if (_ledgerId != null) 'X-Ledger-Id': '$_ledgerId',
      };

  // ---- 认证端点 ----

  /// 注册新账号。首个用户成为默认账本 owner；后续用户需等 owner 邀请。
  /// 成功返回 {token, user}。
  Future<Map<String, dynamic>> register(String username, String password) async {
    final r = await http.post(
      Uri.parse('$base/api/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'username': username, 'password': password}),
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    setToken(j['token'] as String);
    return j;
  }

  /// 登录，成功返回 {token, user}。
  Future<Map<String, dynamic>> login(String username, String password) async {
    final r = await http.post(
      Uri.parse('$base/api/auth/login'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'username': username, 'password': password}),
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    setToken(j['token'] as String);
    return j;
  }

  /// 安全退出：注销服务端会话并清除本地 token。
  Future<void> logout() async {
    if (_token != null) {
      try {
        await http.post(Uri.parse('$base/api/auth/logout'), headers: _headers);
      } catch (_) {
        // 网络失败也继续清除本地 token
      }
    }
    clearToken();
  }

  /// 获取当前登录用户信息（含 role；未加入账本 role 为空串）。
  Future<Map<String, dynamic>> me() async {
    final r = await http.get(Uri.parse('$base/api/auth/me'), headers: _headers);
    _check(r);
    return jsonDecode(r.body) as Map<String, dynamic>;
  }

  /// 用当前 token 换取新 token（会话再续 30 天，旧 token 立即失效）。
  Future<String> refresh() async {
    final r = await http.post(
      Uri.parse('$base/api/auth/refresh'),
      headers: _headers,
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    final t = j['token'] as String;
    setToken(t);
    return t;
  }

  /// 凭旧密码修改密码；成功后服务端签发新 token（旧会话全部失效）。
  Future<String> changePassword(String oldPassword, String newPassword) async {
    final r = await http.post(
      Uri.parse('$base/api/auth/change-password'),
      headers: _headers,
      body: jsonEncode({'old_password': oldPassword, 'new_password': newPassword}),
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    final t = j['token'] as String?;
    if (t != null) setToken(t);
    return t ?? '';
  }

  // ---- 同步协议 ----

  Future<PullResp> pull(int since, String clientId) async {
    final r = await http.get(
      Uri.parse('$base/api/sync/pull?since=$since&client_id=$clientId'),
      headers: _headers,
    );
    _check(r);
    return PullResp.fromJson(jsonDecode(r.body) as Map<String, dynamic>);
  }

  Future<PushResp> push(
    String clientId,
    int since,
    List<Map<String, dynamic>> changes,
  ) async {
    final r = await http.post(
      Uri.parse('$base/api/sync/push'),
      headers: _headers,
      body: jsonEncode({
        'client_id': clientId,
        'since': since,
        'changes': changes,
      }),
    );
    _check(r);
    return PushResp.fromJson(jsonDecode(r.body) as Map<String, dynamic>);
  }

  // ---- 删除/关闭（P1-B1 / 0.4-A close 语义）----

  /// 关闭账户（Beancount close）：DELETE /api/accounts/{uuid}，服务器置 close_date。
  /// 0.4-A 起语义从软删改为关闭，保留全部交易引用；HTTP 动词仍为 DELETE。
  Future<void> closeAccount(String uuid) async {
    final r = await http.delete(
      Uri.parse('$base/api/accounts/$uuid'),
      headers: _headers,
    );
    _check(r);
  }

  /// 软删一笔交易（写 sync_log delete）。
  Future<void> deleteTxn(String uuid) async {
    final r = await http.delete(
      Uri.parse('$base/api/transactions/$uuid'),
      headers: _headers,
    );
    _check(r);
  }

  // ---- 账本管理（P1-C 多账本）----

  /// 列出当前用户所属的全部账本（含角色）。
  Future<List<Ledger>> listLedgers() async {
    final r = await http.get(Uri.parse('$base/api/ledgers'), headers: _headers);
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    return (j['ledgers'] as List)
        .map((e) => Ledger.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 创建新账本（创建者成为 owner）。
  Future<Ledger> createLedger(String name, String defaultCommodity) async {
    final r = await http.post(
      Uri.parse('$base/api/ledgers'),
      headers: _headers,
      body: jsonEncode({'name': name, 'default_commodity': defaultCommodity}),
    );
    _check(r);
    return Ledger.fromJson(jsonDecode(r.body) as Map<String, dynamic>);
  }

  /// 更新账本名称与默认币种（仅 owner）。
  Future<Ledger> updateLedger(int id, String name, String defaultCommodity) async {
    final r = await http.patch(
      Uri.parse('$base/api/ledgers/$id'),
      headers: _headers,
      body: jsonEncode({'name': name, 'default_commodity': defaultCommodity}),
    );
    _check(r);
    return Ledger.fromJson(jsonDecode(r.body) as Map<String, dynamic>);
  }

  // ---- 成员管理（P1-B3，仅 owner）----

  Future<List<Member>> listMembers() async {
    final r = await http.get(
      Uri.parse('$base/api/ledger/members'),
      headers: _headers,
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    return (j['members'] as List)
        .map((e) => Member.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// owner 按用户名把已注册用户加入账本（角色 editor/viewer）。
  Future<void> addMember(String username, String role) async {
    final r = await http.post(
      Uri.parse('$base/api/ledger/members'),
      headers: _headers,
      body: jsonEncode({'username': username, 'role': role}),
    );
    _check(r);
  }

  /// owner 调整成员角色（POST /api/ledger/members/{userID} {role}）。
  Future<void> updateMemberRole(int userId, String role) async {
    final r = await http.post(
      Uri.parse('$base/api/ledger/members/$userId'),
      headers: _headers,
      body: jsonEncode({'role': role}),
    );
    _check(r);
  }

  /// owner 移除成员。
  Future<void> removeMember(int userId) async {
    final r = await http.delete(
      Uri.parse('$base/api/ledger/members/$userId'),
      headers: _headers,
    );
    _check(r);
  }

  /// owner 重置成员密码（其全部会话失效）。
  Future<void> resetMemberPassword(int userId, String newPassword) async {
    final r = await http.post(
      Uri.parse('$base/api/ledger/members/$userId/reset-password'),
      headers: _headers,
      body: jsonEncode({'new_password': newPassword}),
    );
    _check(r);
  }

  // ---- 导出 ----

  Future<String> export() async {
    final r = await http.get(Uri.parse('$base/api/export'), headers: _headers);
    _check(r);
    return r.body;
  }

  /// 导出多文件目录（main.bean + accounts/ + date/）打包的 zip 字节流。
  Future<List<int>> exportArchive() async {
    final r = await http.get(
      Uri.parse('$base/api/export/archive'),
      headers: _headers,
    );
    _check(r);
    return r.bodyBytes;
  }

  /// 检查响应：非 2xx 抛 ApiException（尽力提取服务端 error 字段）。
  void _check(http.Response r) {
    if (r.statusCode >= 200 && r.statusCode < 300) return;
    var message = 'HTTP ${r.statusCode}';
    try {
      final j = jsonDecode(r.body);
      if (j is Map && j['error'] is String) {
        message = j['error'] as String;
      }
    } catch (_) {}
    throw ApiException(r.statusCode, message);
  }
}

/// 带状态码的 API 异常，便于 UI 按 401/403 分别处理。
class ApiException implements Exception {
  final int statusCode;
  final String message;
  ApiException(this.statusCode, this.message);

  @override
  String toString() => message;
}

/// 默认后端地址。部署到 NAS 时改这里或注入覆盖。
const kDefaultApiBase = 'http://localhost:8080';
