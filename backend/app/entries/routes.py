# -*- coding: utf-8 -*-
from flask import Blueprint, request, jsonify
from flask_jwt_extended import jwt_required
import os
import re
from datetime import datetime, timedelta
from app.utils.ledger_utils import load_ledger, notify_subscribers

# 创建蓝图
entries_bp = Blueprint('entries', __name__)

# 配置
LEDGER_FILE = os.getenv('LEDGER_FILE', 'data/main.bean')
PAD_EQUITY_ACCOUNT = os.getenv('PAD_EQUITY_ACCOUNT', 'Equity:Opening-Balances')


@entries_bp.route('', methods=['GET'])
@jwt_required()
def get_entries():
    """获取所有记账条目（支持时间段筛选、账户筛选、分页和排序）"""
    entries, errors, options = load_ledger()

    # 从请求参数中获取筛选条件
    start_date = request.args.get('start_date')
    end_date = request.args.get('end_date')
    account = request.args.get('account')  # 添加账户筛选条件
    page = int(request.args.get('page', 1))
    page_size = int(request.args.get('page_size', 20))
    sort = request.args.get('sort', 'date')  # 默认按日期排序
    order = request.args.get('order', 'desc')  # 默认降序

    # 转换为适合JSON的格式并应用筛选
    entries_data = []
    for entry in entries:
        if hasattr(entry, 'date'):
            # 时间段筛选
            if start_date or end_date:
                entry_date = entry.date
                if start_date and entry_date < datetime.fromisoformat(start_date).date():
                    continue
                if end_date and entry_date > datetime.fromisoformat(end_date).date():
                    continue

            # 递归函数：确保字典中所有键都是字符串类型
            def ensure_string_keys(d):
                if isinstance(d, dict):
                    result = {}
                    for k, v in d.items():
                        result[str(k)] = ensure_string_keys(v)
                    return result
                elif isinstance(d, (list, tuple)):
                    return [ensure_string_keys(item) for item in d]
                else:
                    return d

            # 确保meta字段的所有键都是字符串类型，避免JSON序列化错误
            meta_dict = {}
            if hasattr(entry, 'meta'):
                meta_dict = ensure_string_keys(dict(entry.meta))
            entry_data = {'type': type(entry).__name__, 'date': entry.date.isoformat(), 'meta': meta_dict}

            # 添加交易描述
            if hasattr(entry, 'narration'):
                entry_data['narration'] = entry.narration

            # 添加标签（将frozenset转换为列表以便JSON序列化）
            if hasattr(entry, 'tags') and entry.tags:
                entry_data['tags'] = list(entry.tags)

            # 添加记账行信息
            if hasattr(entry, 'postings'):
                entry_data['postings'] = []
                for posting in entry.postings:
                    posting_data = {'account': posting.account}
                    if hasattr(posting, 'units') and posting.units:
                        posting_data['units'] = {'number': posting.units.number, 'currency': posting.units.currency}
                    entry_data['postings'].append(posting_data)

            # 添加账户信息（Open类型条目）
            if hasattr(entry, 'account'):
                entry_data['account'] = entry.account

            # 账户筛选
            if account:
                # 检查Open/Close/Balance类型的账户
                if hasattr(entry, 'account') and entry.account == account:
                    entries_data.append(entry_data)
                # 检查Transaction类型的记账行
                elif hasattr(entry, 'postings'):
                    for posting in entry.postings:
                        if posting.account == account:
                            entries_data.append(entry_data)
                            break
            else:
                entries_data.append(entry_data)

    # 排序
    entries_data.sort(key=lambda x: x[sort] if sort in x else '', reverse=(order == 'desc'))

    # 分页
    total = len(entries_data)
    start = (page - 1) * page_size
    end = start + page_size
    paginated_entries = entries_data[start:end]

    # 返回分页结果
    return jsonify(
        {
            'entries': paginated_entries,
            'pagination': {
                'total': total,
                'page': page,
                'page_size': page_size,
                'pages': (total + page_size - 1) // page_size,
            },
        }
    )


@entries_bp.route('', methods=['POST'])
@jwt_required()
def add_entry():
    """添加新的记账条目"""
    data = request.json

    # 解析日期，确定文件路径
    entry_date = data['date']
    year = entry_date.split('-')[0]
    month = entry_date.split('-')[1]

    # 构建文件路径
    data_dir = os.path.dirname(LEDGER_FILE)
    os.makedirs(os.path.join(data_dir, 'date'), exist_ok=True)
    date_file = os.path.join(data_dir, f'date/{year}/{year}-{month}.bean')
    ledge_file = os.path.join(data_dir, 'date/ledge.bean')

    # 确保年份目录存在
    year_dir = os.path.dirname(date_file)
    os.makedirs(year_dir, exist_ok=True)

    # 构建Beancount条目字符串
    entry_type = data.get('type', 'Transaction')
    entry_str = ""

    if entry_type == 'Open' or entry_type == 'Close':
        # Open/Close类型条目
        if not data.get('account'):
            return jsonify({'error': 'Account is required for Open/Close entries'}), 400

        entry_str = f"{data['date']} {entry_type} {data['account']}\n"

        # 添加货币
        if data.get('currency'):
            entry_str += f"  currency {data['currency']}\n"

        # 添加note
        if data.get('note'):
            entry_str += f"  note: \"{data['note']}\"\n"
    elif entry_type == 'Balance':
        # Balance类型条目
        if not all(k in data for k in ['account', 'amount']):
            return jsonify({'error': 'Account and amount are required for Balance entries'}), 400

        # 解析金额
        amount_parts = str(data['amount']).split()
        if len(amount_parts) == 2:
            number, currency = amount_parts
        else:
            # 如果没有指定货币，使用默认货币
            number = data['amount']
            currency = 'CNY'

        # 计算当前账户余额
        entries, errors, options = load_ledger()
        account = data['account']
        balance_date = datetime.fromisoformat(data['date']).date()

        # 计算截至balance日期前一天的账户余额
        current_balance = 0.0
        for entry in entries:
            if hasattr(entry, 'date') and entry.date < balance_date:
                if hasattr(entry, 'postings'):
                    for posting in entry.postings:
                        if posting.account == account and hasattr(posting, 'units') and posting.units:
                            current_balance += float(posting.units.number)

        # 转换目标余额为浮点数
        target_balance = float(number)

        # 构建条目字符串
        entry_str = ""

        # 如果当前余额与目标余额不一致，添加Pad指令
        if abs(current_balance - target_balance) > 0.001:  # 允许小数点后三位的误差
            # Pad的日期应该是balance的前一天
            pad_date = balance_date - timedelta(days=1)
            entry_str = f"{pad_date.isoformat()} pad {account} {PAD_EQUITY_ACCOUNT}\n"

        # 添加Balance断言
        entry_str += f"{data['date']} balance {account} {number} {currency}\n"
    elif entry_type == 'Pad':
        # Pad类型条目
        if not all(k in data for k in ['source_account', 'target_account']):
            return jsonify({'error': 'Source account and target account are required for Pad entries'}), 400

        entry_str = f"{data['date']} pad {data['source_account']} {data['target_account']}\n"
    else:
        # Transaction类型条目
        # 验证必填字段
        if not all(k in data for k in ['narration', 'postings']):
            return jsonify({'error': 'Missing required fields'}), 400

        entry_str = f"{data['date']} * \"{data['narration']}\"\n"

        if data.get('tags'):
            entry_str += f"  ; tags: {','.join(data['tags'])}\n"

        for posting in data['postings']:
            entry_str += f"  {posting['account']}  {posting['amount']}\n"

    # 写入对应的月份文件
    with open(date_file, 'a', encoding='utf-8') as f:
        f.write('\n' + entry_str)

    # 检查ledge.bean中是否已包含该月份文件
    include_line = f'include \"{year}/{year}-{month}.bean\"'
    ledge_content = ''

    if os.path.exists(ledge_file):
        with open(ledge_file, 'r', encoding='utf-8') as f:
            ledge_content = f.read()

    # 如果没有包含，则添加到ledge.bean中
    if include_line not in ledge_content:
        with open(ledge_file, 'a', encoding='utf-8') as f:
            f.write('\n' + include_line)

    # 通知SSE订阅者有新条目添加
    notify_subscribers("entry_added", data)

    return jsonify({'success': True, 'entry': data})


@entries_bp.route('/<path:entry_id>', methods=['PUT'])
@jwt_required()
def update_entry(entry_id):
    """更新现有的记账条目"""
    try:
        data = request.json
        # 解析entry_id为filename和lineno
        filename, lineno = entry_id.rsplit(':', 1)
        lineno = int(lineno)

        # 获取完整的文件路径
        full_path = os.path.join(os.path.dirname(LEDGER_FILE), filename)

        if not os.path.exists(full_path):
            return jsonify({'error': 'File not found'}), 404

        # 读取文件内容
        with open(full_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()

        # 找到要更新的条目
        # 条目通常从指定行开始，直到遇到空行或下一个条目的开始
        start_line = lineno - 1  # 转换为0-based索引
        end_line = start_line

        # 找到条目的结束位置
        while end_line < len(lines):
            # 如果遇到空行且不是条目开始行，则认为条目结束
            if end_line > start_line and lines[end_line].strip() == '':
                break
            # 如果遇到新的条目开始行（以日期开头），则认为当前条目结束
            if end_line > start_line and re.match(r'^\d{4}-\d{2}-\d{2}', lines[end_line].strip()):
                break
            end_line += 1

        # 构建新的条目字符串
        entry_type = data.get('type', 'Transaction')
        entry_str = ""

        if entry_type == 'Open' or entry_type == 'Close':
            # Open/Close类型条目
            if not data.get('account'):
                return jsonify({'error': 'Account is required for Open/Close entries'}), 400

            entry_str = f"{data['date']} {entry_type} {data['account']}\n"

            # 添加货币
            if data.get('currency'):
                entry_str += f"  currency {data['currency']}\n"

            # 添加note
            if data.get('note'):
                entry_str += f"  note: \"{data['note']}\"\n"
        elif entry_type == 'Balance':
            # Balance类型条目
            if not all(k in data for k in ['account', 'amount']):
                return jsonify({'error': 'Account and amount are required for Balance entries'}), 400

            # 解析金额
            amount_parts = str(data['amount']).split()
            if len(amount_parts) == 2:
                number, currency = amount_parts
            else:
                # 如果没有指定货币，使用默认货币
                number = data['amount']
                currency = 'CNY'

            entry_str = f"{data['date']} balance {data['account']} {number} {currency}\n"
        elif entry_type == 'Pad':
            # Pad类型条目
            if not all(k in data for k in ['source_account', 'target_account']):
                return jsonify({'error': 'Source account and target account are required for Pad entries'}), 400

            entry_str = f"{data['date']} pad {data['source_account']} {data['target_account']}\n"
        else:
            # Transaction类型条目
            # 验证必填字段
            if not all(k in data for k in ['narration', 'postings']):
                return jsonify({'error': 'Missing required fields'}), 400

            entry_str = f"{data['date']} * \"{data['narration']}\"\n"

            if data.get('tags'):
                entry_str += f"  ; tags: {','.join(data['tags'])}\n"

            for posting in data['postings']:
                entry_str += f"  {posting['account']}  {posting['amount']}\n"

        # 删除旧条目并插入新条目
        new_lines = lines[:start_line] + [entry_str]

        # 如果不是文件末尾，添加剩余内容
        if end_line < len(lines):
            # 如果旧条目后面有空行，保留一个空行
            if lines[end_line].strip() == '':
                new_lines.append('\n')
            new_lines.extend(lines[end_line:])

        # 写回文件
        with open(full_path, 'w', encoding='utf-8') as f:
            f.writelines(new_lines)

        # 通知SSE订阅者有条目更新
        notify_subscribers("entry_updated", data)

        return jsonify({'success': True, 'entry': data})
    except Exception as e:
        print(f"Error updating entry: {e}")
        return jsonify({'error': str(e)}), 500


@entries_bp.route('/<path:entry_id>', methods=['DELETE'])
@jwt_required()
def delete_entry(entry_id):
    """删除现有的记账条目"""
    try:
        # 解析entry_id为filename和lineno
        filename, lineno = entry_id.rsplit(':', 1)
        lineno = int(lineno)

        # 获取完整的文件路径
        full_path = os.path.join(os.path.dirname(LEDGER_FILE), filename)

        if not os.path.exists(full_path):
            return jsonify({'error': 'File not found'}), 404

        # 读取文件内容
        with open(full_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()

        # 找到要删除的条目
        start_line = lineno - 1  # 转换为0-based索引
        end_line = start_line

        # 找到条目的结束位置
        while end_line < len(lines):
            # 如果遇到空行且不是条目开始行，则认为条目结束
            if end_line > start_line and lines[end_line].strip() == '':
                break
            # 如果遇到新的条目开始行（以日期开头），则认为当前条目结束
            if end_line > start_line and re.match(r'^\d{4}-\d{2}-\d{2}', lines[end_line].strip()):
                break
            end_line += 1

        # 删除条目内容
        new_lines = lines[:start_line]

        # 如果不是文件末尾，添加剩余内容
        if end_line < len(lines):
            # 如果旧条目后面有两个空行，只保留一个
            if end_line < len(lines) and lines[end_line].strip() == '':
                end_line += 1
            if end_line < len(lines) and lines[end_line].strip() == '':
                end_line += 1
            new_lines.extend(lines[end_line:])

        # 写回文件
        with open(full_path, 'w', encoding='utf-8') as f:
            f.writelines(new_lines)

        # 通知SSE订阅者有条目删除
        notify_subscribers("entry_deleted", {'id': entry_id})

        return jsonify({'success': True})
    except Exception as e:
        print(f"Error deleting entry: {e}")
        return jsonify({'error': str(e)}), 500