#!/usr/bin/env python3
# -*- coding: utf-8 -*-
from app import app
from flask_jwt_extended import create_access_token

with app.app_context():
    token = create_access_token(identity='test_user')
    print(f"Generated JWT Token: {token}")
    print(f"\nUse this token in API requests with Authorization: Bearer {token}")
