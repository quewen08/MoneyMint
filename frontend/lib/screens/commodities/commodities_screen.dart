/// PC 端「商品与价格」占位页（后端商品/价格 API 尚未就绪）。
import 'package:flutter/material.dart';
import '../../widgets/pc.dart';

class CommoditiesScreen extends StatelessWidget {
  const CommoditiesScreen({super.key});

  @override
  Widget build(BuildContext context) => const SingleChildScrollView(
        padding: EdgeInsets.all(24),
        child: ComingSoon('商品与价格',
            message: '商品目录与历史价格（用于跨币种成本估算）将在后续版本开放。'),
      );
}
