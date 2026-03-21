# -*- coding: utf-8 -*-
from flask import Blueprint, request, jsonify
from flask_jwt_extended import jwt_required
import beanquery
import os
import re
from app.utils.ledger_utils import load_ledger, get_include_structure, LEDGER_FILE

# 创建蓝图
ledger_bp = Blueprint('ledger', __name__)


@ledger_bp.route('', methods=['GET'])
@jwt_required()
def get_ledger():
    """获取账本基本信息"""
    entries, errors, options = load_ledger()
    from app.utils.ledger_utils import ledger_cache

    # 确保返回的数据都是可序列化的类型
    # 将frozenset等不可序列化对象转换为可序列化类型
    serializable_options = {}
    for key, value in options.items():
        if isinstance(value, (frozenset, set)):
            # 将set/frozenset转换为list
            serializable_options[key] = list(value)
        elif isinstance(value, (int, float, str, bool, list, dict, type(None))):
            # 基本类型直接保留
            serializable_options[key] = value
        else:
            # 其他类型转换为字符串
            serializable_options[key] = str(value)

    # 确保errors也是可序列化的
    serializable_errors = []
    for error in errors:
        if hasattr(error, '__dict__'):
            # 如果是对象，转换为字典
            serializable_errors.append(error.__dict__)
        else:
            # 否则转换为字符串
            serializable_errors.append(str(error))

    return jsonify(
        {
            'title': serializable_options.get('title', 'MoneyMint Ledger'),
            'currency': serializable_options.get('operating_currency', 'CNY'),
            'entries_count': len(entries),
            'errors_count': len(errors),
            'errors': serializable_errors,
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

@ledger_bp.route('/files', methods=['GET'])
@jwt_required()
def get_files():
    """获取所有账本相关文件的列表"""
    include_structure = get_include_structure()
    files = []
    base_dir = os.path.dirname(LEDGER_FILE)

    # 遍历include结构，收集所有文件
    def collect_files(file_path):
        if file_path not in files:
            files.append(file_path)
        for included_file in include_structure.get(file_path, []):
            collect_files(included_file)

    collect_files(LEDGER_FILE)

    # 转换为相对于base_dir的路径
    relative_files = [os.path.relpath(f, base_dir) for f in files]
    
    return jsonify({'files': relative_files})

@ledger_bp.route('/files/<path:file_path>', methods=['GET'])
@jwt_required()
def get_file_content(file_path):
    """获取文件内容"""
    base_dir = os.path.dirname(LEDGER_FILE)
    full_path = os.path.normpath(os.path.join(base_dir, file_path))

    # 安全检查：确保文件在base_dir内
    if not full_path.startswith(base_dir):
        return jsonify({'error': 'Access denied'}), 403

    try:
        with open(full_path, 'r', encoding='utf-8') as f:
            content = f.read()
        return jsonify({'content': content})
    except Exception as e:
        return jsonify({'error': str(e)}), 400

@ledger_bp.route('/files/<path:file_path>', methods=['PUT'])
@jwt_required()
def save_file_content(file_path):
    """保存文件内容"""
    base_dir = os.path.dirname(LEDGER_FILE)
    full_path = os.path.normpath(os.path.join(base_dir, file_path))

    # 安全检查：确保文件在base_dir内
    if not full_path.startswith(base_dir):
        return jsonify({'error': 'Access denied'}), 403

    data = request.json
    content = data.get('content', '')

    try:
        with open(full_path, 'w', encoding='utf-8') as f:
            f.write(content)
        # 重新加载账本以更新缓存
        load_ledger()
        return jsonify({'message': 'File saved successfully'})
    except Exception as e:
        return jsonify({'error': str(e)}), 400