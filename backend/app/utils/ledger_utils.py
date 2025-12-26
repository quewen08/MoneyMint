# -*- coding: utf-8 -*-
import hashlib
import os
import re
import time
import threading
import beancount.loader
import beanquery.query
from collections import defaultdict
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
            print(f"Reloading ledger file: {LEDGER_FILE}")
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
        # 如果LEDGER_FILE包含目录，确保目录存在
        os.makedirs(os.path.dirname(LEDGER_FILE), exist_ok=True)
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


def analyze_include_structure(main_file):
    """分析Beancount文件的include结构

    Args:
        main_file: 主Beancount文件路径

    Returns:
        dict: include结构，键为包含文件的路径，值为被包含文件的列表
        dict: 反向映射，键为被包含文件的路径，值为包含它的文件路径
    """
    include_structure = defaultdict(list)
    reverse_include = {}
    processed_files = set()
    base_dir = os.path.dirname(main_file)

    def process_file(file_path):
        """递归处理文件的include结构"""
        if file_path in processed_files:
            return
        processed_files.add(file_path)

        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
        except (FileNotFoundError, PermissionError):
            return

        # 正则匹配include语句
        include_pattern = re.compile(r'^\s*include\s+"([^"]+)"', re.MULTILINE)
        for match in include_pattern.finditer(content):
            included_file = match.group(1)
            # 处理相对路径
            if not os.path.isabs(included_file):
                included_file_path = os.path.normpath(os.path.join(os.path.dirname(file_path), included_file))
            else:
                included_file_path = included_file

            # 添加到include结构
            include_structure[file_path].append(included_file_path)
            reverse_include[included_file_path] = file_path

            # 递归处理被包含的文件
            process_file(included_file_path)

    # 从主文件开始处理
    process_file(main_file)
    return include_structure, reverse_include


def find_suitable_include_file(main_file, target_file):
    """找到适合包含目标文件的文件

    Args:
        main_file: 主Beancount文件路径
        target_file: 要被包含的文件路径

    Returns:
        str: 应该包含目标文件的文件路径
    """
    print(f'查找包含文件: {target_file}')
    include_structure, reverse_include = analyze_include_structure(main_file)
    base_dir = os.path.dirname(main_file)

    # 获取目标文件的绝对路径
    if not os.path.isabs(target_file):
        target_file_path = os.path.normpath(os.path.join(base_dir, target_file))
    else:
        target_file_path = target_file

    # 检查目标文件是否已经被包含
    if target_file_path in reverse_include:
        return reverse_include[target_file_path]

    # 获取目标文件的目录和文件名
    target_dir = os.path.dirname(target_file_path)
    target_filename = os.path.basename(target_file_path)

    # 检查目标目录下是否有合适的文件可以包含它
    dir_files = [f for f in os.listdir(target_dir) if f.endswith('.bean')]

    # 优先选择与目标文件同目录下的main.bean或target_file所在目录同名的.bean文件
    possible_includes = []
    for f in dir_files:
        if f == 'main.bean' or f == f'{os.path.basename(target_dir)}.bean':
            possible_includes.append(os.path.join(target_dir, f))

    # 如果找到合适的文件，检查它是否已经被包含在某个父文件中
    for include_file in possible_includes:
        if include_file in reverse_include:
            return include_file

    # 如果没有找到，检查是否有与目标文件同目录的其他文件已经被包含
    for f in dir_files:
        file_path = os.path.join(target_dir, f)
        if file_path in reverse_include:
            return file_path

    # 最后，尝试找到包含目标目录的文件
    parent_dir = os.path.dirname(target_dir)
    while parent_dir != base_dir:
        print(f'检查目录: {parent_dir}')
        parent_files = [f for f in os.listdir(parent_dir) if f.endswith('.bean')]
        for f in parent_files:
            file_path = os.path.join(parent_dir, f)
            if file_path in include_structure:
                return file_path
        parent_dir = os.path.dirname(parent_dir)

    # 如果都没有找到，默认使用date/ledge.bean
    return os.path.join(base_dir, 'date', 'ledge.bean')


# 初始化默认账本
initialize_default_ledger()
