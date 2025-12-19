# MoneyMint Accounting System [中文](README-zh.md)|ENGLISH [![docker image size](https://img.shields.io/docker/image-size/quewen08/money-mint/latest?label=docker-image)](https://hub.docker.com/repository/docker/quewen08/money-mint/general) ![docker pulls](https://img.shields.io/docker/pulls/quewen08/money-mint)

A modern personal accounting system built on Beancount and Nuxt.js, featuring powerful backend services and a mobile-friendly frontend interface to help users easily manage their personal finances.

## 🚀 Project Overview

MoneyMint is an open-source personal financial management system designed to provide an intuitive interface and robust accounting features. The system adopts a decoupled architecture with a Python/Flask backend using Beancount as the core accounting engine, and a modern frontend built with Nuxt.js and Vue 3.

### Core Advantages

- **Beancount-based**: Leverages professional double-entry accounting system to ensure financial data accuracy
- **Modern Interface**: Responsive design supporting both desktop and mobile devices
- **Real-time Synchronization**: SSE technology enables real-time data updates
- **Flexible Deployment**: Supports local deployment and Docker containerization
- **Data Security**: JWT authentication mechanism with data stored in local plain text files

## 📋 Features

### Core Functionality

- **Ledger Management**: Support for multiple ledgers with data stored in secure plain text format
- **Income & Expense Tracking**: Complete double-entry accounting system supporting various transaction types
- **Category Statistics**: Financial data statistics by category, time, account, and other dimensions
- **Chart Analysis**: Visual charts displaying financial status and trends
- **Account Management**: Multi-account management including assets, liabilities, income, and expense accounts

### Enhanced Features

- **Transaction Copy**: Quickly create similar transactions to improve accounting efficiency
- **Smart Account Filtering**: Account search and filtering functionality when adding transactions for easy multi-account selection
- **Account Transaction Details**: View all transaction records for a specific account in the account details page, supporting date filtering and pagination
- **Real-time Data Updates**: SSE technology enables real-time ledger data push updates
- **User Authentication**: Complete user login, registration, and permission management system

## 🛠️ Technology Stack

### Backend

- **Python 3.11**: Main development language
- **Flask**: Web application framework
- **Beancount**: Core accounting engine
- **Fava**: Beancount's web interface (optional integration)
- **Flask-JWT-Extended**: JWT authentication
- **Flask-CORS**: Cross-origin resource sharing
- **Flask-Bcrypt**: Password encryption

### Frontend

- **Nuxt.js 3**: Full-stack Vue.js framework solution
- **Vue 3**: Progressive JavaScript framework
- **Tailwind CSS**: Utility-first CSS framework
- **TypeScript**: Type-safe JavaScript superset
- **Pinia**: Vue 3 state management library

### Data Storage

- **Beancount Plain Text Ledger**: Human-readable financial data format
- **Local File System**: Data stored locally ensuring privacy and security

## 📁 Project Structure

```
MoneyMint/
├── backend/                    # Python backend services
│   ├── app/                   # Application code
│   │   ├── __init__.py        # Application initialization
│   │   ├── auth/              # Authentication module
│   │   ├── ledger/            # Ledger management module
│   │   ├── entries/           # Transaction entries module
│   │   ├── accounts/          # Accounts management module
│   │   ├── events/            # SSE events module
│   │   └── utils/             # Utility functions
│   ├── data/                  # Ledger data directory
│   │   └── main.bean          # Default ledger file
│   ├── .env                   # Environment configuration
│   ├── requirements.txt       # Python dependencies
│   ├── run.py                 # Application entry point
│   ├── setup.py               # Package installation configuration
│   └── README.md              # Backend module documentation
├── frontend/                  # Nuxt frontend project
│   ├── assets/                # Static assets
│   ├── components/            # Vue components
│   ├── composables/           # Composable functions
│   │   └── useApi.ts          # API call encapsulation
│   ├── pages/                 # Page components
│   │   ├── index.vue          # Homepage/Dashboard
│   │   ├── accounts.vue       # Account management
│   │   ├── entries.vue        # Transaction records
│   │   └── login.vue          # Login page
│   ├── plugins/               # Nuxt plugins
│   ├── public/                # Public resources
│   ├── nuxt.config.ts         # Nuxt configuration
│   ├── package.json           # Node.js dependencies
│   └── tailwind.config.js     # Tailwind CSS configuration
├── Dockerfile                 # Docker build file
├── docker-compose.yml         # Docker Compose configuration
├── README.md                  # English documentation
└── README-zh.md               # Chinese documentation
```

## 🚀 Quick Start

### Local Deployment

#### 1. Backend Setup

```bash
# Enter backend directory
cd backend

# Create virtual environment
python -m venv venv

# Activate virtual environment
# Windows
env\Scripts\activate
# Linux/macOS
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Run backend service
python app/main.py
```

The backend service will start at `http://localhost:5000`.

#### 2. Frontend Setup

```bash
# Enter frontend directory
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev
```

The frontend development server will start at `http://localhost:3000`.

### Docker Deployment

Quickly deploy the entire application using Docker Compose:

```bash
# Pull latest image
docker pull quewen08/money-mint:latest

# Copy docker-compose.yml file
cp docker-compose.yml .

# Build and start containers
docker-compose up -d

# Check container status
docker-compose ps
```

The application will be available at `http://localhost:3000`.

## 🔧 Configuration Options

### Backend Configuration

Backend can be configured via environment variables or `.env` file:

| Configuration | Description | Default Value |
|---------------|-------------|---------------|
| `LEDGER_FILE` | Ledger file path | `data/main.bean` |
| `JWT_SECRET_KEY` | JWT secret key | `your-secret-key-change-this-in-production` |
| `ADMIN_USERNAME` | Admin username | `admin` |
| `ADMIN_PASSWORD` | Admin password | `admin123` |
| `ENABLE_REGISTRATION` | Whether to enable registration | `true` |
| `PAD_EQUITY_ACCOUNT` | Equity account | `Equity:Opening-Balances` |

### Frontend Configuration

Frontend configuration file is located at `frontend/nuxt.config.ts`, main configuration items:

| Configuration | Description | Default Value |
|---------------|-------------|---------------|
| `apiBaseUrl` | Backend API address | `http://localhost:5000/api` |
| `app.port` | Frontend service port | `3000` |

## 📖 User Guide

### 1. User Authentication

- When accessing the system for the first time, use the default admin account to log in:
  - Username: `admin`
  - Password: `admin123`
- You can change the password or create new users after logging in

### 2. Ledger Management

- The system automatically creates a default ledger file
- Ledgers can be managed through backend API or by directly editing ledger files
- Multi-ledger switching is supported (requires manual configuration)

### 3. Account Management

- View all accounts and their balances
- Click on an account name to view its transaction records
- Support date range filtering and pagination when viewing transaction records

### 4. Adding Transactions

- Select transaction type (income, expense, transfer, etc.)
- Fill in transaction date, amount, description, etc.
- Select relevant accounts and categories
- Use the copy function to quickly create similar transactions

### 5. Data Statistics

- Dashboard displays total income, total expenses, and net income
- Category statistics show the proportion of various expenses
- Chart analysis intuitively displays financial trends

## 🤝 Contribution Guidelines

Contributions of all kinds are welcome! Whether you're reporting bugs, suggesting new features, or submitting code directly, your support is greatly appreciated.

### Contribution Process

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Standards

- Follow existing code style
- Ensure code passes all tests before submission
- Add documentation for new features
- Provide clear commit messages

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/quewen08/MoneyMint/blob/master/LICENSE) file for details.

## 📞 Contact

- Project address: [https://github.com/quewen08/MoneyMint](https://github.com/quewen08/MoneyMint)
- Issue feedback: [https://github.com/quewen08/MoneyMint/issues](https://github.com/quewen08/MoneyMint/issues)

## 📦 Technical Documentation

- [Beancount Official Documentation](https://beancount.github.io/docs/)
- [Flask Official Documentation](https://flask.palletsprojects.com/)
- [Nuxt.js Official Documentation](https://nuxt.com/docs/)

## 🙏 Acknowledgments

Thanks to all the developers and users who have contributed to the MoneyMint project!

---

If you find MoneyMint helpful, please give us a ⭐ Star to support our work!