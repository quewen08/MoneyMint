# -*- coding: utf-8 -*-
"""
MoneyMint 后端应用主入口
"""
import os
from dotenv import load_dotenv

from app import app

if __name__ == '__main__':

    # 加载环境变量文件
    load_status = load_dotenv()

    ENV = os.getenv('FLASK_ENV', 'development')
    if not load_status:
        print("警告：未加载到环境变量文件 .env")
    else:
        print("成功加载环境变量文件 .env")
        # 打印所有加载的环境变量
        if ENV == 'development':
            for key, value in os.environ.items():
                print(f"{key}: {value}")

    app.run(debug=ENV == 'development', host='0.0.0.0', port=5000)
