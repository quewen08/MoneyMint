# -*- coding: utf-8 -*-
from flask import Blueprint, request, jsonify
from app import bcrypt
from app.auth.users import users, ENABLE_REGISTRATION

# 创建蓝图
auth_bp = Blueprint('auth', __name__)


@auth_bp.route('/login', methods=['POST'])
def login():
    """用户登录"""
    data = request.json
    username = data.get('username')
    password = data.get('password')

    if not username or not password:
        return jsonify({'error': '用户名和密码不能为空'}), 400

    if username not in users:
        return jsonify({'error': '用户名或密码错误'}), 401

    # 验证密码
    if bcrypt.check_password_hash(users[username]['password_hash'], password):
        from flask_jwt_extended import create_access_token

        # 创建JWT Token
        access_token = create_access_token(identity=username)
        return jsonify({'access_token': access_token, 'username': username, 'role': users[username]['role']}), 200
    else:
        return jsonify({'error': '用户名或密码错误'}), 401


@auth_bp.route('/register/status', methods=['GET'])
def check_registration_status():
    """检查注册功能状态"""
    return jsonify({'enabled': ENABLE_REGISTRATION})


@auth_bp.route('/register', methods=['POST'])
def register():
    """用户注册"""
    # 检查是否启用了注册功能
    if not ENABLE_REGISTRATION:
        return jsonify({'error': '注册功能已关闭'}), 403

    data = request.json
    username = data.get('username')
    password = data.get('password')

    if not username or not password:
        return jsonify({'error': '用户名和密码不能为空'}), 400

    if len(password) < 6:
        return jsonify({'error': '密码长度不能少于6位'}), 400

    if username in users:
        return jsonify({'error': '用户名已存在'}), 409

    # 创建新用户
    users[username] = {'password_hash': bcrypt.generate_password_hash(password).decode('utf-8'), 'role': 'user'}

    return jsonify({'message': '注册成功'}), 201
