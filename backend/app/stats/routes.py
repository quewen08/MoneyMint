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
)
logger = logging.getLogger(__name__)
from app.utils.ledger_utils import load_ledger
from datetime import datetime

# 创建蓝图
stats_bp = Blueprint("stats", __name__)


@stats_bp.route("/category-stats", methods=["GET"])
@jwt_required()
def get_category_stats():
    """获取分类统计数据"""
    try:
        # 加载账本
        logger.debug("开始加载账本...")
        entries, errors, options = load_ledger()

        # 检查账本加载是否有错误
        if errors and len(errors) > 0:
            logger.error(f"账本加载错误: {[str(e) for e in errors]}")
            return (
                jsonify({"error": "账本加载错误", "details": [str(e) for e in errors]}),
                500,
            )

        # 检查必要的数据是否存在
        if not entries:
            logger.error("账本条目为空")
            return jsonify({"error": "账本数据无效", "details": "账本条目为空"}), 500

        if not options:
            logger.error("账本选项为空")
            return jsonify({"error": "账本数据无效", "details": "账本选项为空"}), 500

        # 获取货币类型，处理列表形式的返回值
        currency = options.get("operating_currency", ["CNY"])[0] if options.get("operating_currency") else "CNY"
        logger.debug(f"获取到货币类型: {currency}")

        # 从请求参数中获取日期范围
        start_date = request.args.get("start_date")
        end_date = request.args.get("end_date")

        # 验证日期格式
        if start_date:
            try:
                datetime.strptime(start_date, "%Y-%m-%d")
                logger.debug(f"开始日期格式验证通过: {start_date}")
            except ValueError:
                logger.error(f"无效的开始日期格式: {start_date}")
                return jsonify({"error": "无效的开始日期格式，应为YYYY-MM-DD"}), 400

        if end_date:
            try:
                datetime.strptime(end_date, "%Y-%m-%d")
                logger.debug(f"结束日期格式验证通过: {end_date}")
            except ValueError:
                logger.error(f"无效的结束日期格式: {end_date}")
                return jsonify({"error": "无效的结束日期格式，应为YYYY-MM-DD"}), 400

        # 先不使用日期条件查询，而是在Python中过滤
        # 构建简单的查询条件
        def build_simple_query(account_pattern):
            """构建简单的查询条件，只包含账户匹配"""
            return f"select date, account, position where account ~ '{account_pattern}'"
        
        def parse_position(position):
            """解析position字段，提取金额部分
            Beancount的position格式通常是'金额 货币'，如'-16581.55 CNY'
            """
            if isinstance(position, (int, float)):
                return float(position)
            
            try:
                # 移除货币符号，提取数字部分
                if isinstance(position, str):
                    # 查找第一个数字或负号
                    import re
                    match = re.search(r'^-?\d+(\.\d+)?', position.strip())
                    if match:
                        return float(match.group())
                elif hasattr(position, 'amount'):
                    # 如果是beancount的Amount对象
                    return float(position.amount)
                elif hasattr(position, '__str__'):
                    # 尝试转换为字符串再处理
                    position_str = str(position)
                    import re
                    match = re.search(r'^-?\d+(\.\d+)?', position_str.strip())
                    if match:
                        return float(match.group())
            except Exception as e:
                logger.error(f"解析position失败: {position}, 错误: {str(e)}")
            
            return 0.0

        # 查询收入原始数据（包含日期）
        income_query = build_simple_query("^Income:")
        logger.debug(f"执行收入查询: {income_query}")

        try:
            income_result = beanquery.run_query(entries, options, income_query)
            logger.debug(f"收入查询执行成功")
        except Exception as e:
            logger.error(f"收入查询执行失败: {str(e)}")
            return jsonify({"error": "收入查询失败", "details": str(e)}), 500

        # 查询支出原始数据（包含日期）
        expense_query = build_simple_query("^Expenses:")
        logger.debug(f"执行支出查询: {expense_query}")

        try:
            expense_result = beanquery.run_query(entries, options, expense_query)
            logger.debug(f"支出查询执行成功")
        except Exception as e:
            logger.error(f"支出查询执行失败: {str(e)}")
            return jsonify({"error": "支出查询失败", "details": str(e)}), 500

        # 解析日期范围（用于后续过滤）
        start_dt = None
        end_dt = None

        if start_date:
            start_dt = datetime.strptime(start_date, "%Y-%m-%d")
            logger.debug(f"解析的开始日期: {start_dt}")

        if end_date:
            end_dt = datetime.strptime(end_date, "%Y-%m-%d")
            logger.debug(f"解析的结束日期: {end_dt}")

        # 在Python中处理收入数据 - 按账户聚合并过滤日期
        income_stats = []
        income_account_totals = {}

        if income_result and len(income_result) == 2:
            income_rtypes, income_rrows = income_result
            logger.debug(f"收入查询结果类型: {income_rtypes}")
            logger.debug(f"收入查询结果行: {income_rrows}")

            # 提取表头名称
            income_headers = []
            if income_rtypes and isinstance(income_rtypes, list):
                for col in income_rtypes:
                    try:
                        if isinstance(col, tuple) and len(col) > 0:
                            income_headers.append(str(col[0]))
                    except Exception as e:
                        logger.error(f"处理收入表头失败: {str(e)}")
                        continue

                logger.debug(f"收入表头: {income_headers}")

                # 处理行数据，按账户聚合并过滤日期
                if income_rrows and isinstance(income_rrows, list) and income_headers:
                    for row in income_rrows:
                        try:
                            if isinstance(row, tuple) and len(row) == len(
                                income_headers
                            ):
                                row_dict = dict(zip(income_headers, row))

                                # 提取日期并验证格式
                                entry_date = None
                                if "date" in row_dict:
                                    try:
                                        # 尝试将日期转换为datetime对象
                                        if isinstance(row_dict["date"], datetime):
                                            entry_date = row_dict["date"]
                                        else:
                                            # 尝试字符串解析
                                            entry_date = datetime.strptime(
                                                str(row_dict["date"]), "%Y-%m-%d"
                                            )
                                    except (ValueError, TypeError) as e:
                                        logger.warning(
                                            f"解析日期失败: {row_dict['date']}, 错误: {str(e)}"
                                        )
                                        continue

                                # 日期过滤
                                if entry_date:
                                    if start_dt and entry_date < start_dt:
                                        continue
                                    if end_dt and entry_date > end_dt:
                                        continue

                                account = row_dict.get("account")
                                position = row_dict.get("position", 0)

                                # 解析金额
                                amount = parse_position(position)
                                
                                # 收入账户在beancount中通常显示为负数，需要转换为正数
                                if account and account.startswith("Income:"):
                                    amount = abs(amount)

                                # 累加同一账户的金额
                                if account:
                                    if account in income_account_totals:
                                        income_account_totals[account] += amount
                                    else:
                                        income_account_totals[account] = amount

                                logger.debug(f"处理收入行: {row_dict}")
                        except Exception as e:
                            logger.error(f"处理收入行数据失败: {str(e)}")
                            continue

        # 将收入聚合结果转换为列表格式
        for account, total in income_account_totals.items():
            income_stats.append({"account": account, "total": total})

        # 按金额降序排序
        income_stats.sort(key=lambda x: x["total"], reverse=True)

        # 在Python中处理支出数据 - 按账户聚合并过滤日期
        expense_stats = []
        expense_account_totals = {}

        if expense_result and len(expense_result) == 2:
            expense_rtypes, expense_rrows = expense_result
            logger.debug(f"支出查询结果类型: {expense_rtypes}")
            logger.debug(f"支出查询结果行: {expense_rrows}")

            # 提取表头名称
            expense_headers = []
            if expense_rtypes and isinstance(expense_rtypes, list):
                for col in expense_rtypes:
                    try:
                        if isinstance(col, tuple) and len(col) > 0:
                            expense_headers.append(str(col[0]))
                    except Exception as e:
                        logger.error(f"处理支出表头失败: {str(e)}")
                        continue

                logger.debug(f"支出表头: {expense_headers}")

                # 处理行数据，按账户聚合并过滤日期
                if (
                    expense_rrows
                    and isinstance(expense_rrows, list)
                    and expense_headers
                ):
                    for row in expense_rrows:
                        try:
                            if isinstance(row, tuple) and len(row) == len(
                                expense_headers
                            ):
                                row_dict = dict(zip(expense_headers, row))

                                # 提取日期并验证格式
                                entry_date = None
                                if "date" in row_dict:
                                    try:
                                        # 尝试将日期转换为datetime对象
                                        if isinstance(row_dict["date"], datetime):
                                            entry_date = row_dict["date"]
                                        else:
                                            # 尝试字符串解析
                                            entry_date = datetime.strptime(
                                                str(row_dict["date"]), "%Y-%m-%d"
                                            )
                                    except (ValueError, TypeError) as e:
                                        logger.warning(
                                            f"解析日期失败: {row_dict['date']}, 错误: {str(e)}"
                                        )
                                        continue

                                # 日期过滤
                                if entry_date:
                                    if start_dt and entry_date < start_dt:
                                        continue
                                    if end_dt and entry_date > end_dt:
                                        continue

                                account = row_dict.get("account")
                                position = row_dict.get("position", 0)

                                # 解析金额
                                amount = parse_position(position)
                                
                                # 支出账户处理：确保金额为正数（如果是负数需要转换）
                                if account and account.startswith("Expenses:"):
                                    amount = abs(amount)

                                # 累加同一账户的金额
                                if account:
                                    if account in expense_account_totals:
                                        expense_account_totals[account] += amount
                                    else:
                                        expense_account_totals[account] = amount

                                logger.debug(f"处理支出行: {row_dict}")
                        except Exception as e:
                            logger.error(f"处理支出行数据失败: {str(e)}")
                            continue

        # 将支出聚合结果转换为列表格式
        for account, total in expense_account_totals.items():
            expense_stats.append({"account": account, "total": total})

        # 按金额降序排序
        expense_stats.sort(key=lambda x: x["total"], reverse=True)

        # 计算总收入和总支出
        total_income = 0.0
        for item in income_stats:
            if "total" in item:
                try:
                    total_income += float(item["total"])
                except (ValueError, TypeError) as e:
                    logger.warning(f"计算收入总和时类型转换失败: {str(e)}")
                    continue

        total_expense = 0.0
        for item in expense_stats:
            if "total" in item:
                try:
                    total_expense += float(item["total"])
                except (ValueError, TypeError) as e:
                    logger.warning(f"计算支出总和时类型转换失败: {str(e)}")
                    continue

        logger.debug(f"总收入: {total_income}, 总支出: {total_expense}")

        # 构建响应
        response = {
            "income": {"categories": income_stats, "total": total_income},
            "expense": {"categories": expense_stats, "total": total_expense},
            "currency": currency,
        }

        logger.debug(f"API响应: {response}")
        return jsonify(response), 200

    except Exception as e:
        logger.error(f"API处理异常: {str(e)}", exc_info=True)
        return (
            jsonify(
                {
                    "error": "服务器内部错误",
                    "details": str(e),
                    "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                }
            ),
            500,
        )