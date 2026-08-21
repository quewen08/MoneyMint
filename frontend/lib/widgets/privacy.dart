/// 首页金额隐私开关（0.4-D）：localStorage 记忆，跨刷新保持。
/// 仅首页移动端账单视图使用；通过 AmountText.hidden 蒙层展示。
import 'dart:html' as html;

class Privacy {
  static const _key = 'fl_hide_home_amounts';

  /// 读取上次的隐藏状态（默认不隐藏）。
  static bool load() => html.window.localStorage[_key] == '1';

  /// 持久化隐藏状态。
  static void save(bool hidden) {
    if (hidden) {
      html.window.localStorage[_key] = '1';
    } else {
      html.window.localStorage.remove(_key);
    }
  }
}
