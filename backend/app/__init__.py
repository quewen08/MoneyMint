# -*- coding: utf-8 -*-

# 从version.py导入应用版本信息
from .version import __version__
from flask import Flask, jsonify
from flask_cors import CORS
from flask_bcrypt import Bcrypt
from flask_jwt_extended import JWTManager
import os
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 创建应用实例
app = Flask(__name__)

# 配置CORS
CORS(app)

from datetime import timedelta

# JWT 配置
app.config['JWT_SECRET_KEY'] = os.getenv('JWT_SECRET_KEY', 'your-secret-key-change-this-in-production')
app.config['JWT_ACCESS_TOKEN_EXPIRES'] = timedelta(days=int(os.getenv('JWT_ACCESS_TOKEN_EXPIRES', 7)))  # 默认7天

# 初始化扩展
bcrypt = Bcrypt(app)
jwt = JWTManager(app)

# 导入路由
from app.auth.routes import auth_bp
from app.ledger.routes import ledger_bp
from app.entries.routes import entries_bp
from app.accounts.routes import accounts_bp
from app.events.routes import events_bp
from app.stats.routes import stats_bp

# 注册蓝图
app.register_blueprint(auth_bp, url_prefix='/api/auth')
app.register_blueprint(ledger_bp, url_prefix='/api/ledger')
app.register_blueprint(entries_bp, url_prefix='/api/entries')
app.register_blueprint(accounts_bp, url_prefix='/api/accounts')
app.register_blueprint(events_bp, url_prefix='/api/events')
app.register_blueprint(stats_bp, url_prefix='/api/stats')


@app.route('/api/version', methods=['GET'])
def get_version():
    """获取应用版本信息"""
    return jsonify({'version': __version__})
