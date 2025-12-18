# MoneyMint 记账系统

基于 beancount 和 fava 的个人记账系统，包含后端服务和移动端友好的前端界面。

## 项目结构

```
MoneyMint/
├── backend/          # Python 后端服务
│   ├── app/         # 应用代码
│   ├── config/      # 配置文件
│   └── requirements.txt
├── frontend/         # Nuxt 前端项目
│   ├── components/  # 组件
│   ├── pages/       # 页面
│   └── nuxt.config.ts
└── README.md
```

## 技术栈

- **后端**: Python, Flask, Beancount, Fava
- **前端**: Nuxt.js, Vue 3, Tailwind CSS
- **数据库**: Beancount 账本文件 (纯文本)

## 功能特性

- 账本管理
- 收支记录
- 分类统计
- 图表分析
- 移动端友好
- 交易记录复制功能：快速创建相似交易
- 添加交易时的账户筛选功能：在多账户时方便选择目标账户
- 账户详情中查看当前账户交易记录：支持按日期筛选和分页

## 安装与运行

### 后端

```bash
cd backend
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
python app/main.py
```

### 前端

```bash
cd frontend
npm install
npm run dev
```