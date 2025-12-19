# -*- coding: utf-8 -*-
from flask import Blueprint, Response
from flask_jwt_extended import jwt_required
import time
import json
from app.utils.ledger_utils import load_ledger, sse_subscribers

# 创建蓝图
events_bp = Blueprint('events', __name__)


@events_bp.route('', methods=['GET'])
@jwt_required()
def events():
    """SSE端点，用于实时通知账本更新"""

    def event_stream():
        # 客户端连接时发送欢迎消息
        yield f'data: {{"event": "connected", "data": "Connected to SSE server"}}\n\n'
        # 发送账户列表
        entries, errors, options = load_ledger()

        # 收集所有账户
        accounts = set()
        for entry in entries:
            if hasattr(entry, 'postings'):
                for posting in entry.postings:
                    accounts.add(posting.account)
        # 发送账户列表
        yield f'data: {{"event": "accounts", "data": {json.dumps(sorted(list(accounts)))}}}\n\n'

        # 创建一个队列来接收事件
        queue = []

        # 定义回调函数，用于将事件添加到队列
        def callback(event_data):
            queue.append(event_data)

        # 添加订阅者
        sse_subscribers.append(callback)

        try:
            while True:
                # 如果队列中有事件，发送它们
                while queue:
                    event_data = queue.pop(0)
                    # 使用字符串格式化来构造JSON数据，避免在生成器中使用jsonify
                    yield f'data: {{"event": "{event_data["event"]}", "data": {json.dumps(event_data["data"])}}}\n\n'

                # 等待一段时间再检查新事件
                time.sleep(0.5)
        except GeneratorExit:
            # 客户端断开连接时移除订阅者
            sse_subscribers.remove(callback)
        except Exception as e:
            # 其他错误时移除订阅者
            print(f"SSE Error: {e}")
            if callback in sse_subscribers:
                sse_subscribers.remove(callback)

    return Response(event_stream(), mimetype='text/event-stream')
