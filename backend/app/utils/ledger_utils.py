# -*- coding: utf-8 -*-
import os
import time
import threading
import beancount.loader
from datetime import datetime

# 配置
LEDGER_FILE = os.getenv('LEDGER_FILE', 'data/main.bean')
PAD_EQUITY_ACCOUNT = os.getenv('PAD_EQUITY_ACCOUNT', 'Equity:Opening-Balances')

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
