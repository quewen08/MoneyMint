# -*- coding: utf-8 -*-
import hashlib
import os
import time
import threading
import beancount.loader
import beanquery.query
from datetime import datetime

# 配置
LEDGER_FILE = os.getenv('LEDGER_FILE', 'data/main.bean')
PAD_EQUITY_ACCOUNT = os.getenv('PAD_EQUITY_ACCOUNT', 'Equity:Opening-Balances')

# 缓存机制
ledger_cache = {'entries': None, 'errors': None, 'options': None, 'last_modified': 0}
query_cache = {}  # 查询结果缓存，键为查询哈希，值为(结果, 账本修改时间)

# 锁用于线程安全
cache_lock = threading.Lock()
query_cache_lock = threading.Lock()

# SSE订阅者
sse_subscribers = []


def get_file_modification_time():
    """获取账本文件的最后修改时间"""
    # 获取主文件的修改时间
    main_mtime = os.path.getmtime(LEDGER_FILE)

    # 检查data目录下所有文件的修改时间
    data_dir = os.path.dirname(LEDGER_FILE)

    if os.path.exists(data_dir):
        for root, dirs, files in os.walk(data_dir):
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
    global sse_subscribers

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


def run_query_with_cache(entries, options, query):
    """运行查询并使用缓存（带线程安全）

    Args:
        entries: beancount条目列表
        options: beancount选项
        query: beanquery查询语句

    Returns:
        查询结果，如果查询失败返回None
    """
    # 查询计时
    start_time = time.time()

    global query_cache

    # 生成查询的哈希值作为缓存键
    query_hash = hashlib.md5((query + str(options)).encode('utf-8')).hexdigest()

    with query_cache_lock:
        # 检查缓存中是否有有效结果
        current_mtime = ledger_cache['last_modified']
        if query_hash in query_cache:
            cached_result, cache_mtime = query_cache[query_hash]
            if cache_mtime == current_mtime:
                return cached_result

    # 缓存不存在或已过期，执行查询

    try:
        result = beanquery.query.run_query(entries, options, query)
    except Exception as e:
        print(f"Query execution failed: {str(e)}")
        return None

    # 将结果存入缓存
    with query_cache_lock:
        query_cache[query_hash] = (result, current_mtime)

    # 查询计时结束
    end_time = time.time()
    query_time = end_time - start_time
    print(f"Query executed in {query_time:.4f} seconds")

    return result


def initialize_default_ledger():
    """初始化默认账本文件"""
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
2023-01-01 open Equity:Opening-Balances CNY
'''
            )


# 确保数据目录存在
os.makedirs('data', exist_ok=True)

# 初始化默认账本
initialize_default_ledger()