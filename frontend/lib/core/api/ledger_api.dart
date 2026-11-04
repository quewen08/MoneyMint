/// 后端 REST 客户端（认证 + 同步协议 + 删除 + 成员管理 + 导出）。
/// 数据写入走 /api/sync/push，拉取走 /api/sync/pull；删除走 REST DELETE；
/// 导出走 /api/export。所有请求自动附带登录后拿到的 Bearer token（持久化于 localStorage）。
import 'dart:convert';
import 'dart:html' as html;
import 'package:flutter/foundation.dart' show kDebugMode;
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
    _isAdmin = false;
    html.window.localStorage.remove(_tokenKey);
  }

  // ---- 系统管理员标记 ----
  // 控制「数据备份」入口是否可见。不持久化：每次 me()/登录 时从服务端刷新，
  // 避免本地缓存的旧值让已被取消管理员身份的用户仍看到备份入口。
  static bool _isAdmin = false;
  static bool get isAdmin => _isAdmin;

  /// 从登录/me 响应里提取 is_admin（服务端位于 user 对象内）。
  static void _captureAdmin(Map<String, dynamic> j) {
    final u = j['user'];
    if (u is Map && u['is_admin'] is bool) {
      _isAdmin = u['is_admin'] as bool;
    } else if (j['is_admin'] is bool) {
      _isAdmin = j['is_admin'] as bool;
    }
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
    _captureAdmin(j);
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
    _captureAdmin(j);
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

  /// 获取当前登录用户信息（含 role 与 is_admin；未加入账本 role 为空串）。
  Future<Map<String, dynamic>> me() async {
    final r = await http.get(Uri.parse('$base/api/auth/me'), headers: _headers);
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    _captureAdmin(j);
    return j;
  }

  /// ---- 系统设置（B-4 注册开关）----

  /// 查询注册是否开放。公开端点（无需登录，注册页在登录前就要用）；
  /// 查询失败按开放处理，避免服务端抖动误锁注册入口。
  Future<bool> registrationEnabled() async {
    try {
      final r = await http.get(Uri.parse('$base/api/settings/registration'));
      if (r.statusCode != 200) return true;
      final j = jsonDecode(r.body) as Map<String, dynamic>;
      return j['enabled'] as bool? ?? true;
    } catch (_) {
      return true;
    }
  }

  /// 设置注册开关（仅系统管理员；owner 运行时切换，持久化到 app_settings）。
  Future<void> setRegistrationEnabled(bool enabled) async {
    final r = await http.post(
      Uri.parse('$base/api/settings/registration'),
      headers: _headers,
      body: jsonEncode({'enabled': enabled}),
    );
    _check(r);
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

  // ---- 交易回收站（M3）----

  /// 列出回收站（服务端已软删交易），按删除时间倒序。
  ///
  /// 只走服务端：交易被删时本地库已移除该条，离线状态下拿不到。
  Future<List<TrashItem>> listTrash() async {
    final r = await http.get(
      Uri.parse('$base/api/transactions/trash'),
      headers: _headers,
    );
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    return (j['transactions'] as List? ?? const [])
        .map((e) => TrashItem.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 从回收站恢复一笔交易（服务端会写 sync_log op=create，其他设备随后拉回）。
  Future<void> restoreTransaction(String uuid) async {
    final r = await http.post(
      Uri.parse('$base/api/transactions/$uuid/restore'),
      headers: _headers,
      body: '{}',
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

  // ---- 备份（全库级，仅系统管理员）----

  /// 列出全部备份记录（含失败记录，按时间倒序）。
  Future<List<BackupItem>> listBackups() async {
    final r = await http.get(Uri.parse('$base/api/backup/list'), headers: _headers);
    _check(r);
    final j = jsonDecode(r.body) as Map<String, dynamic>;
    final items = j['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map<String, dynamic>>()
        .map(BackupItem.fromJson)
        .toList();
  }

  /// 手动触发一次备份。
  Future<BackupItem> runBackup() async {
    final r = await http.post(Uri.parse('$base/api/backup/run'), headers: _headers);
    _check(r);
    return BackupItem.fromJson(jsonDecode(r.body) as Map<String, dynamic>);
  }

  /// 下载指定备份文件（服务端会校验文件名白名单）。
  Future<List<int>> downloadBackup(String name) async {
    final r = await http.get(
      Uri.parse('$base/api/backup/download?name=${Uri.encodeQueryComponent(name)}'),
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

/// 后端 API 地址。
///
/// 默认为**空**（同域相对路径）：前端与 API 同端口部署（Go 服务托管前端）时，
/// 请求自动发往当前访问的域名——反向代理、直连、换端口、HTTPS 都无需任何改动。
///
/// 仅在两种场景需要注入：
///   1. 本地开发（flutter run 的 dev server 端口与后端 8080 不同）：
///        flutter run -d chrome --dart-define=API_BASE=http://localhost:8080
///   2. 前端与后端分离部署（不同域名）：
///        flutter build web --dart-define=API_BASE=https://api.example.com
///
/// 历史教训：此前这里硬编码 'http://localhost:8080'，构建时被编译进 JS。
/// 通过反向代理访问时，页面 JS 发的 API 请求指向的是「访问者自己电脑的 8080」，
/// 表现为：NAS 本机一切正常，换设备或走域名后 API 全挂（HTTPS 下还会被
/// 浏览器按混合内容直接拦截）。
/// 后端 API 基地址。三档取值：
///
/// 1. 构建时显式注入：`--dart-define=API_BASE=https://api.example.com`
///    （前后端分离部署、或本地后端不在 8080 时用）
/// 2. **debug 默认 `http://localhost:8080`**：`flutter run` 的页面运行在
///    dev server 源上，与后端不同源——若用相对路径，所有请求会打到
///    dev server 自身（那里没有后端），且失败被同步层静默吞掉，
///    表现为页面永远转圈、控制台毫无输出（实测踩坑）。
/// 3. **release 默认空串 = 同域相对路径**：Go 服务同端口托管前端，
///    反代 / 直连 / HTTPS 零配置。
const String kDefaultApiBase = String.fromEnvironment(
  'API_BASE',
  defaultValue: kDebugMode ? 'http://localhost:8080' : '',
);
