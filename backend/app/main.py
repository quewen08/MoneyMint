# -*- coding: utf-8 -*-
from flask import Flask, request, jsonify, Response
from flask_cors import CORS
import beancount.loader
import beanquery
import os
import time
import threading
import json
from dotenv import load_dotenv
from datetime import datetime

# 加载环境变量
load_dotenv()

app = Flask(__name__)
CORS(app)

# 配置
LEDGER_FILE = os.getenv('LEDGER_FILE', 'data/main.bean')

# 确保数据目录存在
os.makedirs('data', exist_ok=True)

# 初始化默认账本文件
if not os.path.exists(LEDGER_FILE):
    with open(LEDGER_FILE, 'w', encoding='utf-8') as f:
        f.write(
            '''option "title" "MoneyMint Ledger"
option "operating_currency" "CNY"

2023-01-01 open Assets:Cash CNY
2023-01-01 open Assets:Bank CNY
2023-01-01 open Income:Salary CNY
2023-01-01 open Expenses:Food CNY
2023-01-01 open Expenses:Transport CNY
'''
        )

# 缓存机制
ledger_cache = {'entries': None, 'errors': None, 'options': None, 'last_modified': 0}

# 锁用于线程安全
cache_lock = threading.Lock()

# SSE订阅者
sse_subscribers = []


def get_file_modification_time():
    """获取账本文件的最后修改时间"""
    # 获取主文件的修改时间
    main_mtime = os.path.getmtime(LEDGER_FILE)

    # 检查date目录下所有文件的修改时间
    data_dir = os.path.dirname(LEDGER_FILE)
    date_dir = os.path.join(data_dir, 'date')

    if os.path.exists(date_dir):
        for root, dirs, files in os.walk(date_dir):
            for file in files:
                if file.endswith('.bean'):
                    file_path = os.path.join(root, file)
                    file_mtime = os.path.getmtime(file_path)
                    if file_mtime > main_mtime:
                        main_mtime = file_mtime

    return main_mtime


def load_ledger():
    """加载账本文件（带缓存机制）"""
    global ledger_cache

    current_mtime = get_file_modification_time()

    with cache_lock:
        # 如果缓存为空或文件已修改，则重新加载
        if ledger_cache['entries'] is None or current_mtime > ledger_cache['last_modified']:
            print("Reloading ledger file...")
            # 使用UTF-8编码加载账本文件
            entries, errors, options = beancount.loader.load_file(LEDGER_FILE, encoding='utf-8')
            ledger_cache = {'entries': entries, 'errors': errors, 'options': options, 'last_modified': current_mtime}
            if len(errors) > 0:
                print(f"errors: {errors}")

    return ledger_cache['entries'], ledger_cache['errors'], ledger_cache['options']


def notify_subscribers(event="update", data=None):
    """通知所有SSE订阅者"""
    global ssse_subscribers

    if data is None:
        # 发送账本摘要数据
        entries, errors, options = load_ledger()
        data = {
            'title': options.get('title', 'My Ledger'),
            'currency': options.get('operating_currency', 'CNY'),
            'entries_count': len(entries),
            'errors_count': len(errors),
            'errors': errors,
            'last_modified': ledger_cache['last_modified'],
        }

    # 发送事件给所有订阅者
    for subscriber in sse_subscribers[:]:  # 使用副本避免修改问题
        try:
            subscriber({'event': event, 'data': data})
        except Exception as e:
            # 移除失效的订阅者
            sse_subscribers.remove(subscriber)


@app.route('/api/ledger', methods=['GET'])
def get_ledger():
    """获取账本基本信息"""
    entries, errors, options = load_ledger()
    return jsonify(
        {
            'title': options.get('title', 'My Ledger'),
            'currency': options.get('operating_currency', 'CNY'),
            'entries_count': len(entries),
            'errors_count': len(errors),
            'errors': errors,
            'last_modified': ledger_cache['last_modified'],
        }
    )


@app.route('/api/entries', methods=['GET'])
def get_entries():
    """获取所有记账条目（支持时间段筛选、分页和排序）"""
    entries, errors, options = load_ledger()

    # 从请求参数中获取筛选条件
    start_date = request.args.get('start_date')
    end_date = request.args.get('end_date')
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

            entry_data = {'type': type(entry).__name__, 'date': entry.date.isoformat(), 'meta': dict(entry.meta)}

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


@app.route('/api/query', methods=['POST'])
def run_query():
    """执行Beancount查询"""
    data = request.json
    query = data.get('query', '')

    if not query:
        return jsonify({'error': 'Query is required'}), 400

    entries, errors, options = load_ledger()

    try:
        rows = beancount.query.run_query(entries, options, query)
        headers = [str(h) for h in rows[0]]
        results = [dict(zip(headers, row)) for row in rows[1]]
        return jsonify({'headers': headers, 'results': results})
    except Exception as e:
        return jsonify({'error': str(e)}), 400


@app.route('/api/entries', methods=['POST'])
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

    # 写入对应的月份文件
    with open(date_file, 'a', encoding='utf-8') as f:
        f.write('\n' + entry_str)

    # 检查ledge.bean中是否已包含该月份文件
    include_line = f'include "{year}/{year}-{month}.bean"'
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


@app.route('/api/entries/<path:entry_id>', methods=['PUT'])
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


@app.route('/api/entries/<path:entry_id>', methods=['DELETE'])
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


@app.route('/api/accounts', methods=['GET'])
def get_accounts():
    """获取所有账户"""
    entries, errors, options = load_ledger()

    # 收集所有账户
    accounts = set()
    for entry in entries:
        if hasattr(entry, 'postings'):
            for posting in entry.postings:
                accounts.add(posting.account)

    return jsonify(sorted(list(accounts)))


@app.route('/api/accounts/balances', methods=['GET'])
def get_account_balances():
    """获取所有账户的余额信息（支持日期范围筛选）"""
    entries, errors, options = load_ledger()
    currency = options.get('operating_currency', 'CNY')

    # 从请求参数中获取日期范围
    start_date = request.args.get('start_date')
    end_date = request.args.get('end_date')

    # 转换为日期对象用于比较
    start_date_obj = datetime.fromisoformat(start_date).date() if start_date else None
    end_date_obj = datetime.fromisoformat(end_date).date() if end_date else None

    # 计算账户余额
    balances = {}
    account_notes = {}
    account_currencies = {}

    for entry in entries:
        # 检查entry是否有日期属性
        if hasattr(entry, 'date'):
            # 应用日期范围筛选
            if start_date_obj or end_date_obj:
                entry_date = entry.date
                if start_date_obj and entry_date < start_date_obj:
                    continue
                if end_date_obj and entry_date > end_date_obj:
                    continue

        # 处理账户的Open信息（包含note和currency）
        if hasattr(entry, 'account') and (type(entry).__name__ == 'Open'):
            account = entry.account
            # 获取note信息
            if hasattr(entry, 'meta') and entry.meta:
                if 'note' in entry.meta:
                    account_notes[account] = entry.meta['note']
            # 获取currency信息
            if hasattr(entry, 'currencies') and entry.currencies:
                # currencies可能是字符串列表或对象列表
                if isinstance(entry.currencies[0], str):
                    account_currencies[account] = entry.currencies[0]
                else:
                    account_currencies[account] = (
                        entry.currencies[0].currency if hasattr(entry.currencies[0], 'currency') else currency
                    )

        # 计算账户余额
        if hasattr(entry, 'postings'):
            for posting in entry.postings:
                account = posting.account
                if hasattr(posting, 'units') and posting.units:
                    # 确保amount是数字类型
                    amount = float(posting.units.number)
                    if account not in balances:
                        balances[account] = 0.0
                    balances[account] += amount

    # 构建账户详情
    account_details = []
    for account, balance in balances.items():
        # 提取账户类型
        account_type = account.split(':')[0] if ':' in account else 'Other'

        account_details.append(
            {
                'name': account,
                'type': account_type,
                'balance': float(balance),  # 确保balance是float类型
                'currency': account_currencies.get(account, currency),
                'note': account_notes.get(account, ''),
            }
        )

    # 按账户类型和名称排序
    account_details.sort(key=lambda x: (x['type'], x['name']))

    return jsonify(account_details)


@app.route('/api/events', methods=['GET'])
def events():
    """SSE端点，用于实时通知账本更新"""

    def event_stream():
        # 客户端连接时发送欢迎消息
        yield f'data: {{"event": "connected", "data": "Connected to SSE server"}}\n\n'
        # 发送账户列表
        entries, errors, options = load_ledger()

        # 收集所有账户
        accounts = set()
        for entry in entries:
            if hasattr(entry, 'postings'):
                for posting in entry.postings:
                    accounts.add(posting.account)
        # 发送账户列表
        yield f'data: {{"event": "accounts", "data": {json.dumps(sorted(list(accounts)))}}}\n\n'

        # 创建一个队列来接收事件
        queue = []

        # 定义回调函数，用于将事件添加到队列
        def callback(event_data):
            queue.append(event_data)

        # 添加订阅者
        sse_subscribers.append(callback)

        try:
            while True:
                # 如果队列中有事件，发送它们
                while queue:
                    event_data = queue.pop(0)
                    # 使用字符串格式化来构造JSON数据，避免在生成器中使用jsonify
                    yield f'data: {{"event": "{event_data["event"]}", "data": {json.dumps(event_data["data"])}}}\n\n'

                # 等待一段时间再检查新事件
                time.sleep(0.5)
        except GeneratorExit:
            # 客户端断开连接时移除订阅者
            sse_subscribers.remove(callback)
        except Exception as e:
            # 其他错误时移除订阅者
            print(f"SSE Error: {e}")
            if callback in sse_subscribers:
                sse_subscribers.remove(callback)

    return Response(event_stream(), mimetype='text/event-stream')


if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
