# -*- coding: utf-8 -*-
from flask import Blueprint, request, jsonify
from flask_jwt_extended import jwt_required
import beanquery
from app.utils.ledger_utils import load_ledger

# 创建蓝图
ledger_bp = Blueprint('ledger', __name__)


@ledger_bp.route('', methods=['GET'])
@jwt_required()
def get_ledger():
    """获取账本基本信息"""
    entries, errors, options = load_ledger()
    from app.utils.ledger_utils import ledger_cache

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


@ledger_bp.route('/query', methods=['POST'])
@jwt_required()
def run_query():
    """执行Beancount查询"""
    data = request.json
    query = data.get('query', '')

    if not query:
        return jsonify({'error': 'Query is required'}), 400

    entries, errors, options = load_ledger()

    try:
        rows = beanquery.run_query(entries, options, query)
        headers = [str(h) for h in rows[0]]
        results = [dict(zip(headers, row)) for row in rows[1]]
        return jsonify({'headers': headers, 'results': results})
    except Exception as e:
        return jsonify({'error': str(e)}), 400
