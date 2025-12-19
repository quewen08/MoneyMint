# -*- coding: utf-8 -*-
from flask_bcrypt import Bcrypt
import os

# 从环境变量获取用户配置
ADMIN_USERNAME = os.getenv('ADMIN_USERNAME', 'admin')
ADMIN_PASSWORD = os.getenv('ADMIN_PASSWORD', 'admin123')
ENABLE_REGISTRATION = os.getenv('ENABLE_REGISTRATION', 'true').lower() == 'true'

# 用户数据（共享存储）
users = {
    ADMIN_USERNAME: {'password_hash': Bcrypt().generate_password_hash(ADMIN_PASSWORD).decode('utf-8'), 'role': 'admin'}
}
