# -*- coding: utf-8 -*-
"""
MoneyMint 后端应用主入口
"""
import os
from app import app

if __name__ == '__main__':
    ENV = os.getenv('FLASK_ENV', 'development')
    app.run(debug=ENV == 'development', host='0.0.0.0', port=5000)
