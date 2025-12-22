# -*- coding: utf-8 -*-
from flask import Blueprint, request, jsonify
from flask_jwt_extended import jwt_required
import beanquery.query as beanquery
import logging

# 配置日志
logging.basicConfig(
    level=logging.DEBUG,
    filename="app.log",
    filemode="w",
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    encoding="utf-8",
)
logger = logging.getLogger(__name__)
from app.utils.ledger_utils import load_ledger
from datetime import datetime
import re

# 创建蓝图
stats_bp = Blueprint("stats", __name__)


# 解析position字段，提取金额部分
def parse_position(position):
    """解析position字段，提取金额部分
    Beancount的position格式可能是：
    - 数字类型：int/float
    - 字符串类型：'金额 货币'，如'-16581.55 CNY'
    - Amount对象：有amount属性
    - Inventory对象：包含多个Amount对象的集合
    """
    if isinstance(position, (int, float)):
        return float(position)

    try:
        # 处理字符串类型
        if isinstance(position, str):
            # 查找第一个数字或负号
            match = re.search(r'^-?\d+(\.\d+)?', position.strip())
            if match:
                return float(match.group())

        # 处理Amount对象
        elif hasattr(position, 'amount'):
            return float(position.amount)

        # 处理Inventory对象（包含多个Amount）
        elif hasattr(position, 'get_amounts'):
            total = 0.0
            for amount_obj in position.get_amounts():
                if hasattr(amount_obj, 'amount'):
                    total += float(amount_obj.amount)
            return total

        # 处理可迭代对象（可能是Inventory的其他实现）
        elif hasattr(position, '__iter__') and not isinstance(position, (str, dict)):
            try:
                total = 0.0
                for item in position:
                    if hasattr(item, 'amount'):
                        total += float(item.amount)
                    else:
                        # 尝试直接解析item
                        total += parse_position(item)
                return total
            except Exception:
                pass

        # 作为最后尝试，转换为字符串再处理
        elif hasattr(position, '__str__'):
            position_str = str(position)
            match = re.search(r'(-?\d+(\.\d+)?)', position_str)
            if match:
                return float(match.group(1))
    except Exception as e:
        logger.error(f"解析position失败: {position}, 类型: {type(position)}, 错误: {str(e)}")

    return 0.0


@stats_bp.route('/monthly-expenses', methods=['GET'])
@jwt_required()
def get_monthly_expenses():
    """获取每月消费统计，支持日期范围参数"""
    try:
        # 加载账本
        entries, errors, options = load_ledger()

        if errors and len(errors) > 0:
            return jsonify({"error": "账本加载错误", "details": [str(e) for e in errors]}), 500

        if not entries or not options:
            return jsonify({"error": "账本数据无效"}), 500

        # 获取日期范围参数
        start_date = request.args.get('start_date')
        end_date = request.args.get('end_date')

        # 构建查询条件
        where_clause = "account ~ 'Expenses:*'"
        group_by_clause = "month, account"
        order_by_clause = "total DESC"

        # 如果提供了日期范围
        if start_date and end_date:
            try:
                # 解析日期
                start = datetime.strptime(start_date, "%Y-%m-%d")
                end = datetime.strptime(end_date, "%Y-%m-%d")

                # 构建日期范围条件
                where_clause = f"date >= {start.strftime('%Y-%m-%d')} and date <= {end.strftime('%Y-%m-%d')} and account ~ 'Expenses:*'"
                group_by_clause = "year, month, account"
            except ValueError:
                return jsonify({"error": "日期格式无效，应为YYYY-MM-DD"}), 400
        else:
            # 使用当前年月
            current_year = datetime.now().year
            current_month = datetime.now().month
            where_clause = f"year = {current_year} and month = {current_month} and account ~ 'Expenses:*'"
            group_by_clause = "year, month, account"

        # 构建查询
        query = f"""SELECT 
            year, month, account, sum(cost(position)) as total
        FROM 
            {where_clause}
        GROUP BY {group_by_clause}
        ORDER BY {order_by_clause}"""

        try:
            result = beanquery.run_query(entries, options, query)
        except Exception as e:
            return jsonify({"error": "查询执行失败", "details": str(e)}), 500

        # 处理查询结果
        if result and len(result) == 2:
            rtypes, rrows = result
            headers = [str(col[0]) for col in rtypes]

            expenses = []
            total_expense = 0.0

            for row in rrows:
                if len(row) == len(headers):
                    row_dict = dict(zip(headers, row))
                    account = row_dict.get("account", "")
                    total = row_dict.get("total", 0)

                    # 解析金额
                    amount = parse_position(total)
                    total_expense += amount

                    expenses.append({"account": account, "total": amount})

            # 按金额降序排序
            expenses.sort(key=lambda x: x["total"], reverse=True)

            return (
                jsonify(
                    {
                        "monthly_expenses": expenses,
                        "total_expense": total_expense,
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )
        else:
            return (
                jsonify(
                    {
                        "monthly_expenses": [],
                        "total_expense": 0.0,
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )

    except Exception as e:
        logger.error(f"API处理异常: {str(e)}", exc_info=True)
        return jsonify({"error": "服务器内部错误", "details": str(e)}), 500


@stats_bp.route('/monthly-income-expense', methods=['GET'])
@jwt_required()
def get_monthly_income_expense():
    """获取本月收支统计，支持日期范围参数"""
    try:
        # 加载账本
        entries, errors, options = load_ledger()

        if errors and len(errors) > 0:
            return jsonify({"error": "账本加载错误", "details": [str(e) for e in errors]}), 500

        if not entries or not options:
            return jsonify({"error": "账本数据无效"}), 500

        # 获取日期范围参数
        start_date = request.args.get('start_date')
        end_date = request.args.get('end_date')

        # 构建查询条件
        where_clause = "account ~ 'Expenses' OR account ~ 'Liabilities' OR account ~ 'Income'"
        group_by_clause = "year, month, account"

        # 如果提供了日期范围
        if start_date and end_date:
            try:
                # 解析日期
                start = datetime.strptime(start_date, "%Y-%m-%d")
                end = datetime.strptime(end_date, "%Y-%m-%d")

                # 构建日期范围条件，不使用单引号包裹日期
                where_clause = f"date >= {start.strftime('%Y-%m-%d')} and date <= {end.strftime('%Y-%m-%d')} and (account ~ 'Expenses' OR account ~ 'Liabilities' OR account ~ 'Income')"
            except ValueError:
                return jsonify({"error": "日期格式无效，应为YYYY-MM-DD"}), 400
        else:
            # 使用当前年月
            current_year = datetime.now().year
            current_month = datetime.now().month
            where_clause = f"year = {current_year} and month = {current_month} and (account ~ 'Expenses' OR account ~ 'Liabilities' OR account ~ 'Income')"

        # 构建查询
        query = f"""SELECT 
            year, month, root(account, 1) as account, sum(position) as total 
        FROM 
            {where_clause}
        GROUP BY {group_by_clause}
        ORDER BY year, month, account"""

        try:
            result = beanquery.run_query(entries, options, query)
        except Exception as e:
            return jsonify({"error": "查询执行失败", "details": str(e)}), 500

        # 处理查询结果
        if result and len(result) == 2:
            rtypes, rrows = result
            headers = [str(col[0]) for col in rtypes]

            income = 0.0
            expense = 0.0
            liabilities = 0.0

            for row in rrows:
                if len(row) == len(headers):
                    row_dict = dict(zip(headers, row))
                    account = row_dict.get("account", "").lower()
                    total = row_dict.get("total", 0)

                    # 解析金额
                    amount = parse_position(total)

                    if account == "income":
                        # 收入在beancount中通常为负数，转换为正数
                        income += abs(amount)
                    elif account == "expenses":
                        # 支出在beancount中通常为正数，但可能因查询方式不同而有差异
                        expense += abs(amount)
                    elif account == "liabilities":
                        # 负债处理
                        liabilities += amount

            return (
                jsonify(
                    {
                        "income": income,
                        "expense": expense,
                        "liabilities": liabilities,
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )
        else:
            return (
                jsonify(
                    {
                        "income": 0.0,
                        "expense": 0.0,
                        "liabilities": 0.0,
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )

    except Exception as e:
        logger.error(f"API处理异常: {str(e)}", exc_info=True)
        return jsonify({"error": "服务器内部错误", "details": str(e)}), 500


@stats_bp.route('/account-statistics', methods=['GET'])
@jwt_required()
def get_account_statistics():
    """获取账户统计数据，包含Assets和负债部分"""
    try:
        # 加载账本
        entries, errors, options = load_ledger()

        if errors and len(errors) > 0:
            return jsonify({"error": "账本加载错误", "details": [str(e) for e in errors]}), 500

        if not entries or not options:
            return jsonify({"error": "账本数据无效"}), 500

        # 获取日期范围参数
        start_date = request.args.get('start_date')
        end_date = request.args.get('end_date')

        # 构建查询条件
        where_clause = "account ~ 'Assets' OR account ~ 'Liabilities'"
        group_by_clause = "account"

        # 如果提供了日期范围
        if start_date and end_date:
            try:
                # 解析日期
                start = datetime.strptime(start_date, "%Y-%m-%d")
                end = datetime.strptime(end_date, "%Y-%m-%d")

                # 构建日期范围条件，不使用单引号包裹日期
                # where_clause = f"date >= {start.strftime('%Y-%m-%d')} and date <= {end.strftime('%Y-%m-%d')} and (account ~ 'Assets' OR account ~ 'Liabilities')"
                where_clause = f"(account ~ 'Assets' OR account ~ 'Liabilities')"
            except ValueError:
                return jsonify({"error": "日期格式无效，应为YYYY-MM-DD"}), 400

        # 构建查询
        query = f"""SELECT 
            account, sum(position) as total 
        FROM 
            {where_clause}
        GROUP BY {group_by_clause}
        ORDER BY account"""

        try:
            result = beanquery.run_query(entries, options, query)
        except Exception as e:
            return jsonify({"error": "查询执行失败", "details": str(e)}), 500

        # 处理查询结果
        if result and len(result) == 2:
            rtypes, rrows = result
            headers = [str(col[0]) for col in rtypes]

            accounts = []
            assets_total = 0.0
            liabilities_total = 0.0

            for row in rrows:
                if len(row) == len(headers):
                    row_dict = dict(zip(headers, row))
                    account_full = row_dict.get("account", "")
                    total = row_dict.get("total", 0)

                    # 解析金额
                    amount = parse_position(total)

                    # 提取账户类型和名称
                    account_parts = account_full.split(':')
                    account_type = account_parts[0] if account_parts else "Other"
                    account_name = ':'.join(account_parts[1:]) if len(account_parts) > 1 else account_parts[0]

                    # 处理资产和负债的总和
                    if account_type.lower() == "assets":
                        assets_total += abs(amount)  # 资产金额取绝对值
                    elif account_type.lower() == "liabilities":
                        liabilities_total += amount  # 负债保留原始值

                    accounts.append(
                        {
                            "name": account_name,
                            "fullName": account_full,
                            "type": account_type,
                            "balance": abs(amount),  # 账户余额取绝对值
                            "rawBalance": amount,  # 保留原始值用于内部计算
                            "currency": (
                                options.get("operating_currency", ["CNY"])[0]
                                if options.get("operating_currency")
                                else "CNY"
                            ),
                        }
                    )

            # 按账户类型和名称排序
            accounts.sort(key=lambda x: (x["type"].lower(), x["name"]))

            return (
                jsonify(
                    {
                        "accounts": accounts,
                        "assets_total": assets_total,
                        "liabilities_total": liabilities_total,
                        "net_worth": assets_total - liabilities_total,  # 净资产 = 资产总额 - 负债总额
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )
        else:
            return (
                jsonify(
                    {
                        "accounts": [],
                        "assets_total": 0.0,
                        "liabilities_total": 0.0,
                        "net_worth": 0.0,
                        "currency": (
                            options.get("operating_currency", ["CNY"])[0]
                            if options.get("operating_currency")
                            else "CNY"
                        ),
                    }
                ),
                200,
            )

    except Exception as e:
        logger.error(f"API处理异常: {str(e)}", exc_info=True)
        return jsonify({"error": "服务器内部错误", "details": str(e)}), 500
