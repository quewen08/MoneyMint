/// 认证视图模型（MVVM 的 ViewModel 层）：封装登录态与登录/注册/退出/改密命令，
/// 并管理多账本（P1-C）：登录后拉取账本列表并选定当前账本。
/// 持有 LedgerApi 实例，登录态 token 与当前账本 id 均由 LedgerApi 静态持久化于 localStorage。
import 'package:flutter/foundation.dart';
import '../core/api/ledger_api.dart';
import '../core/api/sync_models.dart';

class AuthController extends ChangeNotifier {
  final LedgerApi api;
  AuthController(this.api);

  bool _ready = false;
  bool get ready => _ready;

  bool get loggedIn => LedgerApi.loggedIn;

  String? _username;
  String? get username => _username;

  List<Ledger> _ledgers = [];
  List<Ledger> get ledgers => _ledgers;

  Ledger? _currentLedger;
  Ledger? get currentLedger => _currentLedger;

  int? get currentLedgerId => _currentLedger?.id;

  /// 当前账本中的角色：owner/editor/viewer；无当前账本为空串。
  String get role => _currentLedger?.role ?? '';

  /// 是否至少属于一个账本。没有任何账本（等待 owner 邀请）为 false。
  bool get isMember => _ledgers.isNotEmpty;

  bool _busy = false;
  bool get busy => _busy;

  String? _error;
  String? get error => _error;

  /// 启动时恢复 token 与当前账本；若已登录则拉取用户信息与账本列表。
  /// me() 抛 401 表示服务端已不认这个 token（典型：本地 db 已被替换/重置，
  /// 或者只是过期被踢），必须清掉本地登录态，否则会一直困在
  /// 「loggedIn && !isMember → AwaitInviteScreen」的怪圈里。
  /// 其他异常（网络抖动等）保留 token，等下次启动再尝试。
  Future<void> init() async {
    await LedgerApi.loadToken();
    await LedgerApi.loadLedgerId();
    if (LedgerApi.loggedIn) {
      try {
        final me = await api.me();
        _applyUser(me);
        await _loadLedgers();
      } catch (e) {
        if (e is ApiException && e.statusCode == 401) {
          _resetLocalAuth();
        }
      }
    }
    _ready = true;
    notifyListeners();
  }

  /// 401 时统一清登录态：清 token / 当前账本 / 用户名 / 账本列表。
  /// 不删服务端会话（已失效），也不改 _busy/_error，仅供「本地状态归零」。
  void _resetLocalAuth() {
    LedgerApi.clearToken();
    LedgerApi.setLedgerId(null);
    _username = null;
    _ledgers = [];
    _currentLedger = null;
  }

  Future<void> _run(Future<void> Function() action) async {
    _busy = true;
    _error = null;
    notifyListeners();
    try {
      await action();
    } on ApiException catch (e) {
      // register/login 期间拿到 401 通常是「服务端被重置、当前 token 失效」。
      // 清掉本地登录态、跳回 LoginScreen，让用户重新注册/登录。
      if (e.statusCode == 401) {
        _resetLocalAuth();
      }
      _error = e.message;
    } catch (e) {
      _error = '失败：$e';
    } finally {
      _busy = false;
      notifyListeners();
    }
  }

  /// 从 me()/login()/register() 返回的 {user: {...}} 中提取用户名。
  void _applyUser(Map<String, dynamic> j) {
    final user = j['user'];
    if (user is Map) {
      _username = user['username'] as String? ?? user['display_name'] as String?;
    }
  }

  /// 拉取账本列表，并选定当前账本（优先恢复上次选择，否则第一个）。
  Future<void> _loadLedgers() async {
    _ledgers = await api.listLedgers();
    if (_ledgers.isEmpty) {
      _currentLedger = null;
      LedgerApi.setLedgerId(null);
      return;
    }
    final saved = LedgerApi.ledgerId;
    Ledger? target;
    for (final l in _ledgers) {
      if (l.id == saved) {
        target = l;
        break;
      }
    }
    target ??= _ledgers.first;
    _currentLedger = target;
    LedgerApi.setLedgerId(target.id);
  }

  /// 重新拉取账本列表（创建/被邀请后刷新）。
  Future<void> refreshLedgers() async {
    await _loadLedgers();
    notifyListeners();
  }

  /// 切换到指定账本：更新当前账本与 LedgerApi 上下文，但不清理本地数据
  /// （本地数据清理由 LedgerController.switchToLedger 负责）。
  void selectLedger(Ledger l) {
    _currentLedger = l;
    LedgerApi.setLedgerId(l.id);
    notifyListeners();
  }

  Future<void> login(String user, String pass) => _run(() async {
        _applyUser(await api.login(user, pass));
        await _loadLedgers();
      });

  Future<void> register(String user, String pass) => _run(() async {
        _applyUser(await api.register(user, pass));
        await _loadLedgers();
      });

  /// 凭旧密码修改密码（服务端签发新 token，本端同步更新登录态）。
  Future<void> changePassword(String oldPass, String newPass) =>
      _run(() => api.changePassword(oldPass, newPass));

  Future<void> logout() async {
    await api.logout();
    _username = null;
    _ledgers = [];
    _currentLedger = null;
    LedgerApi.setLedgerId(null);
    notifyListeners();
  }
}
