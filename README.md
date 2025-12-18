# MoneyMint Accounting System

A personal accounting system based on beancount and fava, with backend services and a mobile-friendly frontend interface.

## Project Structure

```
MoneyMint/
├── backend/          # Python backend services
│   ├── app/         # Application code
│   ├── config/      # Configuration files
│   └── requirements.txt
├── frontend/         # Nuxt frontend project
│   ├── components/  # Components
│   ├── pages/       # Pages
│   └── nuxt.config.ts
└── README.md
```

## Technology Stack

- **Backend**: Python, Flask, Beancount, Fava
- **Frontend**: Nuxt.js, Vue 3, Tailwind CSS
- **Database**: Beancount ledger files (plain text)

## Features

- Ledger management
- Income and expense records
- Category statistics
- Chart analysis
- Mobile-friendly design
- Transaction record copy function: quickly create similar transactions
- Account filtering when adding transactions: easy to select target accounts when there are multiple accounts
- View current account transaction records in account details: support date filtering and pagination

## Installation and Running

### Backend

```bash
cd backend
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
python app/main.py
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```