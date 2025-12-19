# -*- coding: utf-8 -*-
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

    # 收集所有账户
    accounts = set()
    for entry in entries:
        if hasattr(entry, 'postings'):
            for posting in entry.postings:
                accounts.add(posting.account)

    return jsonify(sorted(list(accounts)))


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
