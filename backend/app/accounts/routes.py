# -*- coding: utf-8 -*-
import json
import os
from flask import Blueprint, request, jsonify
from flask_jwt_extended import jwt_required
from datetime import datetime
from app.utils.ledger_utils import load_ledger

# 创建蓝图
accounts_bp = Blueprint('accounts', __name__)


@accounts_bp.route('', methods=['GET'])
@jwt_required()
def get_accounts():
    """获取所有账户信息"""
    entries, errors, options = load_ledger()

    # 收集所有Open和Close的账户
    open_accounts = set()
    closed_accounts = set()
    
    for entry in entries:
        if hasattr(entry, 'account'):
            if type(entry).__name__ == 'Open':
                open_accounts.add(entry.account)
            elif type(entry).__name__ == 'Close':
                closed_accounts.add(entry.account)
    
    # 只返回未关闭的账户
    active_accounts = open_accounts - closed_accounts

    return jsonify(sorted(list(active_accounts)))


@accounts_bp.route('/balances', methods=['GET'])
@jwt_required()
def get_account_balances():
    """获取账户余额"""
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
    open_accounts = set()
    closed_accounts = set()

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
        if hasattr(entry, 'account'):
            if type(entry).__name__ == 'Open':
                account = entry.account
                open_accounts.add(account)
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
            elif type(entry).__name__ == 'Close':
                account = entry.account
                closed_accounts.add(account)

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

    # 只返回未关闭的账户
    active_accounts = open_accounts - closed_accounts
    
    # 构建账户详情
    account_details = []
    for account, balance in balances.items():
        # 只包含活跃账户
        if account not in active_accounts:
            continue
            
        # 提取账户类型和二级分类
        account_parts = account.split(':')
        account_type = account_parts[0] if len(account_parts) > 0 else 'Other'
        account_subtype = account_parts[1] if len(account_parts) > 1 else ''

        account_details.append(
            {
                'name': account,
                'type': account_type,
                'subtype': account_subtype,
                'balance': float(balance),  # 确保balance是float类型
                'currency': account_currencies.get(account, currency),
                'note': account_notes.get(account, ''),
            }
        )

    # 按账户类型和名称排序
    account_details.sort(key=lambda x: (x['type'], x['name']))

    return jsonify(account_details)


@accounts_bp.route('/config', methods=['GET'])
@jwt_required()
def get_account_config():
    """获取账户分类配置"""
    # 获取配置文件路径
    config_file_path = os.path.join(os.path.dirname(__file__), 'config', 'account_config.json')
    
    # 读取配置文件
    with open(config_file_path, 'r', encoding='utf-8') as f:
        account_config = json.load(f)
    
    return jsonify(account_config)