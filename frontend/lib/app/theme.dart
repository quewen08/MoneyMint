/// 视觉主题：依据 docs/prototype/index.html 设计稿的规范基线定义。
/// 主色 #3B6EF6 / 成功 #2BA471 / 警示 #E8590C；圆角 12–16；金额等宽数字对齐。
import 'package:flutter/material.dart';

class AppColors {
  static const blue = Color(0xFF3B6EF6);
  static const green = Color(0xFF2BA471);
  static const warn = Color(0xFFE8590C);
  static const bg = Color(0xFFF5F6F8);
  static const card = Color(0xFFFFFFFF);
  static const ink = Color(0xFF1A1D1F);
  static const sub = Color(0xFF6B7280);
  static const line = Color(0xFFE5E7EB);

  static const Map<int, Color> typeColors = {
    // 仅作语义补充，主调仍由蓝/绿/橙构成
  };
}

class AppTheme {
  static ThemeData get light => ThemeData(
        useMaterial3: true,
        brightness: Brightness.light,
        scaffoldBackgroundColor: AppColors.bg,
        colorScheme: ColorScheme.light(
          primary: AppColors.blue,
          secondary: AppColors.green,
          error: AppColors.warn,
          surface: AppColors.card,
          onSurface: AppColors.ink,
          onPrimary: Colors.white,
        ),
        appBarTheme: const AppBarTheme(
          backgroundColor: AppColors.bg,
          foregroundColor: AppColors.ink,
          elevation: 0,
          centerTitle: false,
          titleTextStyle: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: AppColors.ink,
          ),
        ),
        cardTheme: CardThemeData(
          color: AppColors.card,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
          margin: EdgeInsets.zero,
        ),
        elevatedButtonTheme: ElevatedButtonThemeData(
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.blue,
            foregroundColor: Colors.white,
            // 注意：Flutter 3.35 起 Size.fromHeight(h) 等价于 Size(infinity, h)（另一轴变
            // 成 infinity），用作按钮 minimumSize 会强制无限宽，在窄约束容器（如 AppBar
            // actions）里触发 "BoxConstraints forces an infinite width"。此处显式写
            // Size(0, h) 才保留「最小高度 h、宽度随内容」的原意。
            minimumSize: const Size(0, 50),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
            textStyle:
                const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
          ),
        ),
        outlinedButtonTheme: OutlinedButtonThemeData(
          style: OutlinedButton.styleFrom(
            foregroundColor: AppColors.blue,
            side: const BorderSide(color: AppColors.blue),
            // 同上：Flutter 3.35 的 Size.fromHeight 会强制无限宽，改显式 Size(0, 48)。
            minimumSize: const Size(0, 48),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
            textStyle:
                const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
          ),
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: const Color(0xFFF3F4F6),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(10),
            borderSide: BorderSide.none,
          ),
          contentPadding:
              const EdgeInsets.symmetric(horizontal: 12, vertical: 14),
        ),
        bottomNavigationBarTheme: const BottomNavigationBarThemeData(
          backgroundColor: AppColors.card,
          selectedItemColor: AppColors.blue,
          unselectedItemColor: AppColors.sub,
          selectedLabelStyle:
              TextStyle(fontSize: 11, fontWeight: FontWeight.w700),
          unselectedLabelStyle: TextStyle(fontSize: 11),
          type: BottomNavigationBarType.fixed,
          elevation: 0,
        ),
      );

  /// 通用卡片装饰（白底、圆角 14、极淡阴影），用于非 Card 容器。
  static BoxDecoration get cardDecoration => BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(14),
        boxShadow: const [
          BoxShadow(
            color: Color(0x0A000000),
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      );

  static const TextStyle sectionTitle = TextStyle(
    fontSize: 13,
    fontWeight: FontWeight.w600,
    color: AppColors.sub,
  );

  static const TextStyle muted = TextStyle(
    fontSize: 12,
    color: AppColors.sub,
  );
}
