/// PC 端「导入与导出」：真实导出标准 Beancount 文本与 CSV；
/// 导入（OFX/CSV）后端尚未就绪，暂为占位拖放区。
import 'dart:html' as html;
import 'dart:convert';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/common.dart';
import '../../widgets/pc.dart';

class ImportExportScreen extends StatelessWidget {
  const ImportExportScreen({super.key});

  Future<void> _exportBeancount(BuildContext context) async {
    final ledger = AppScope.of(context).ledger;
    try {
      final text = await ledger.exportText();
      _download('family-ledger.beancount', text, 'text/plain');
      if (context.mounted) {
        showAppSnack(context, '已导出 .beancount（${text.length} 字符）');
      }
    } catch (e) {
      if (context.mounted) showAppSnack(context, '导出失败：$e', warn: true);
    }
  }

  Future<void> _exportArchive(BuildContext context) async {
    final ledger = AppScope.of(context).ledger;
    try {
      final bytes = await ledger.exportArchive();
      _download('ledger-beancount.zip', bytes, 'application/zip');
      if (context.mounted) {
        showAppSnack(context, '已导出账本目录（zip，含 main.bean + accounts/ + date/）');
      }
    } catch (e) {
      if (context.mounted) showAppSnack(context, '导出失败：$e', warn: true);
    }
  }

  Future<void> _exportCsv(BuildContext context) async {
    final ledger = AppScope.of(context).ledger;
    final buf = StringBuffer();
    buf.writeln('date,description,flag,account,commodity,amount');
    for (final t in ledger.txns) {
      for (final p in t.postings) {
        final cells = [
          t.date,
          '"${t.description.replaceAll('"', '""')}"',
          t.flag,
          '"${ledger.accountName(p.accountUuid)}"',
          p.commodity,
          p.amount,
        ];
        buf.writeln(cells.join(','));
      }
    }
    _download('family-ledger.csv', const Utf8Encoder().convert(buf.toString()),
        'text/csv');
    if (context.mounted) {
      showAppSnack(context, '已导出 CSV（${ledger.txns.length} 笔交易）');
    }
  }

  void _download(String name, dynamic content, String mime) {
    final blob = html.Blob([content], mime);
    final url = html.Url.createObjectUrlFromBlob(blob);
    html.AnchorElement(href: url)
      ..download = name
      ..click();
    html.Url.revokeObjectUrl(url);
  }

  @override
  Widget build(BuildContext context) => SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 880),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                PcSectionTitle('导出 Beancount'),
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: AppTheme.cardDecoration,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('导出标准复式账本文本，可被 bean-check 直接校验。',
                          style: TextStyle(color: AppColors.sub, fontSize: 13)),
                      const SizedBox(height: 14),
                      Wrap(
                        spacing: 10,
                        runSpacing: 10,
                        children: [
                          PcButton('下载 .beancount', icon: Icons.download,
                              primary: true, onPressed: () => _exportBeancount(context)),
                          PcButton('下载账本目录 (zip)', icon: Icons.folder_zip_outlined,
                              onPressed: () => _exportArchive(context)),
                        ],
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                PcSectionTitle('导出 CSV'),
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: AppTheme.cardDecoration,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('导出为通用表格，便于在 Excel / 表格软件中查看与二次处理。',
                          style: TextStyle(color: AppColors.sub, fontSize: 13)),
                      const SizedBox(height: 14),
                      PcButton('下载 CSV', icon: Icons.download,
                          onPressed: () => _exportCsv(context)),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                PcSectionTitle('导入'),
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(24),
                  decoration: AppTheme.cardDecoration,
                  child: Column(
                    children: [
                      const Icon(Icons.cloud_upload_outlined,
                          size: 44, color: AppColors.sub),
                      const SizedBox(height: 10),
                      const Text('拖放 OFX / CSV 文件到此处',
                          style: TextStyle(fontWeight: FontWeight.w600)),
                      const SizedBox(height: 6),
                      const Text('银行流水自动导入将在后续版本开放。',
                          style: TextStyle(color: AppColors.sub, fontSize: 12)),
                      const SizedBox(height: 14),
                      PcButton('选择文件', onPressed: () {
                        showAppSnack(context, '导入功能即将上线', warn: true);
                      }),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      );
}
