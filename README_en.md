# NetBouncer

[![Go Version](https://img.shields.io/badge/Go-1.24.3+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](Dockerfile)

[English](README_en.md) | [中文](README.md)

NetBouncer is a high-performance network traffic monitoring tool that provides real-time traffic statistics, IP management, and a modern web interface. It supports iptables/ipset firewall, IP group management, batch operations, and more.

## ✨ Key Features

- 🔍 **Real-time Traffic Monitoring**: High-performance network packet capture based on libpcap
- 📊 **Visual Interface**: Modern React web interface with real-time traffic statistics
- 🛡️ **IP Management**: Support for banning/allowing individual IPs or CIDR ranges
- 📁 **Group Management**: IP group management for batch operations
- ⚡ **High Performance**: Built with Go, supporting high-concurrency traffic processing
- 🗄️ **Multiple Storage Backends**: Support for SQLite, MySQL, PostgreSQL databases
- 🔧 **Flexible Configuration**: Support for config files, command-line parameters, and Docker deployment
- 📱 **Responsive Design**: Web interface adapted for desktop and mobile devices
- 🛡️ **Multiple Firewall Types**: Support for iptables, ipset, and mock modes

## 🚀 Quick Start

### Option 1: Using Docker (Recommended)

```bash
# Pull the latest image
docker pull graydovee/netbouncer:latest

# Run container
docker run -d \
  --name netbouncer \
  --network host \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  graydovee/netbouncer:latest

# Access web interface
# http://localhost:8080
```

### Option 2: Build from Source

#### Prerequisites

- Go 1.24.3+
- Node.js 18+ (for building frontend)
- libpcap-dev (Linux)
- iptables/ipset (for firewall functionality)

#### Build Steps

```bash
# 1. Clone repository
git clone https://github.com/graydovee/netbouncer.git
cd netbouncer

# 2. Build frontend and backend
make all

# 3. Run application
./bin/netbouncer
```

## 📖 Usage Guide

### Basic Usage

```bash
# Start with default configuration (ipset mode)
netbouncer

# Start with config file
netbouncer -c config.yaml

# Specify network interface and listen address
netbouncer -i eth0 -l 0.0.0.0:9090

# Use iptables firewall mode
netbouncer --firewall-type iptables

# Use mock mode (for debugging)
netbouncer --firewall-type mock
```

### Configuration File Setup

> 💡 For detailed configuration options, see [CONFIGURATION.md](doc/CONFIGURATION.md)

Create a `config.yaml` file:

```yaml
# Monitor configuration
monitor:
  interface: "eth0"  # Network interface name (leave empty for auto-selection)
  exclude_subnets: "127.0.0.1/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
  window: 60  # Monitoring time window (seconds)
  timeout: 86400  # Time to clean up inactive connections (seconds)

# Firewall configuration
firewall:
  chain: "NETBOUNCER"  # iptables chain name
  ipset: "netbouncer"  # ipset name
  type: "ipset"        # Firewall type: iptables, ipset, mock

# Web service configuration
web:
  listen: "0.0.0.0:8080"  # Web service listen address

# Database configuration
database:
  driver: "sqlite"        # "sqlite", "mysql", "postgres"
  host: ""                # Database host address
  port: 0                 # Database port
  username: ""            # Database username
  password: ""            # Database password
  database: "netbouncer.db"  # Database name or file path
  dsn: ""                 # Database connection string (optional)
  log_level: "info"       # SQL log level: "silent", "error", "warn", "info"

# Initial rules configuration
rules:
  # Example: Create a default blocked group
  - group: "blocked"
    groupDescription: "Default blocked group"
    action: "block"
    override: false
    ipNets:
      - "192.168.1.100"
      - "10.0.0.0/24"
  
  # Example: Create a whitelist group
  - group: "whitelist"
    groupDescription: "Whitelist group"
    action: "allow"
    override: true
    ipNets:
      - "127.0.0.1"
      - "192.168.1.1"
```

### Rules Configuration

The `rules` section allows you to pre-configure IP groups and rules that will be created automatically when the application starts. This is useful for setting up default block lists, whitelists, and other common configurations.

#### Rules Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `group` | string | Yes | Group name to identify the rule group |
| `groupDescription` | string | No | Group description explaining the purpose |
| `action` | string | Yes | Action type: `block` or `allow` |
| `override` | bool | No | Whether to override existing groups (default: false) |
| `ipNets` | []string | Yes | List of IP addresses or CIDR ranges |

#### Use Cases

- **Pre-configured block lists**: Automatically create groups with known malicious IPs
- **Whitelist configuration**: Pre-configure trusted IP addresses
- **Testing environment**: Quickly set up test data for development
- **Production environment**: Pre-configure necessary IP rules based on security policies

### Common Command Line Parameters

| Parameter | Short | Description | Default |
|-----------|-------|-------------|---------|
| `--config` | `-c` | Config file path | - |
| `--monitor-interface` | `-i` | Network interface name | Auto-select |
| `--monitor-exclude-subnets` | `-e` | Excluded subnets | - |
| `--firewall-type` | `-f` | Firewall type (iptables\|ipset\|mock) | ipset |
| `--listen` | `-l` | Web service listen address | 0.0.0.0:8080 |
| `--db-driver` | - | Database driver (sqlite\|mysql\|postgres) | sqlite |
| `--db-name` | - | Database name or file path | netbouncer.db |
| `--db-log-level` | - | SQL log level (silent\|error\|warn\|info) | info |

## 🌐 Web Interface Usage

After starting the application, visit `http://localhost:8080` to access the web interface:

### Traffic Monitor Page
- View real-time network connection traffic statistics
- Sort by traffic, connections, and other fields
- Configurable auto-refresh interval
- One-click IP ban functionality

### IP Management Page
- View all IPs or by group
- Add new IP addresses or CIDR ranges
- Modify IP behavior (ban/allow)
- Change IP group membership
- Batch operations and bulk import

### Group Management Page
- Create, edit, and delete IP groups
- View group lists and group information

## 🗄️ Database Configuration

### SQLite (Default, Recommended)

```yaml
database:
  driver: "sqlite"
  database: "netbouncer.db"
```

### MySQL

```yaml
database:
  driver: "mysql"
  host: "localhost"
  port: 3306
  username: "netbouncer"
  password: "password"
  database: "netbouncer"
```

### PostgreSQL

```yaml
database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  username: "netbouncer"
  password: "password"
  database: "netbouncer"
```

## 🛡️ Firewall Configuration

### ipset Mode (Default)

```yaml
firewall:
  type: "ipset"
  ipset: "netbouncer"
```

### iptables Mode

```yaml
firewall:
  type: "iptables"
  chain: "NETBOUNCER"
```

### mock Mode (Debug)

```yaml
firewall:
  type: "mock"
```

## 🐳 Docker Deployment

### Using Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  netbouncer:
    image: graydovee/netbouncer:latest
    container_name: netbouncer
    network_mode: host
    cap_add:
      - NET_ADMIN
      - NET_RAW
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data:/app/data
    command: ["-c", "/app/config.yaml"]
    restart: unless-stopped
```

Run:
```bash
docker-compose up -d
```

## 🔧 Development

### Development Environment Setup

```bash
# Install Go dependencies
go mod tidy

# Start frontend development server
make web-dev

# Run backend (mock mode)
./bin/netbouncer --firewall-type mock
```

### Build

```bash
# Build all components
make all

# Build Go program only
make build-go

# Build frontend only
make build-web
```

## 📊 API Interface

For detailed API documentation, see [API.md](doc/API.md)

Main API endpoints:
- `GET /api/traffic` - Get traffic statistics
- `GET /api/ip` - Get IP list
- `POST /api/ip` - Create IP rule
- `GET /api/group` - Get group list
- `POST /api/group` - Create group

## 🔐 Authentication Configuration

NetBouncer supports two authentication methods: **BasicAuth** (simple username/password) and **OIDC** (OpenID Connect).

### BasicAuth (Recommended for Simple Deployment)

BasicAuth is the simplest authentication method, only requiring username and password:

```yaml
web:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true                      # Enable authentication
    type: "basic"                      # Authentication type
    basic:
      username: "admin"                # Username
      password: "your-secure-password" # Password
```

#### BasicAuth Features
- Simple and easy to use, no external dependencies
- Supports browser popup login dialog
- Supports frontend login page
- Supports API requests using Authorization header

#### API Call Example
```bash
curl -u admin:your-password http://localhost:8080/api/traffic
```

### OIDC (Recommended for Enterprise Deployment)

OIDC supports integration with external identity providers:

```yaml
web:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true                      # Enable authentication
    type: "oidc"                       # Authentication type
    oidc:
      client_id: "your-client-id"        # OIDC client ID
      client_secret: "your-client-secret" # OIDC client secret
      issuer_url: "https://accounts.google.com"  # OIDC provider URL
      redirect_url: "http://localhost:8080/auth/callback"  # Callback URL
```

#### Common OIDC Provider Configurations

**Google:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "xxx.apps.googleusercontent.com"
    client_secret: "xxx"
    issuer_url: "https://accounts.google.com"
    redirect_url: "http://your-domain:8080/auth/callback"
```

**Keycloak:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "netbouncer"
    client_secret: "xxx"
    issuer_url: "https://keycloak.example.com/realms/your-realm"
    redirect_url: "http://your-domain:8080/auth/callback"
```

**Authentik:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "netbouncer"
    client_secret: "xxx"
    issuer_url: "https://authentik.example.com/application/o/netbouncer/"
    redirect_url: "http://your-domain:8080/auth/callback"
```

### Authentication Flow

1. User accesses web interface
2. If not logged in:
   - BasicAuth: Shows login page or browser popup
   - OIDC: Redirects to OIDC provider login page
3. After successful authentication, access the application
4. Session remains valid for 24 hours

### API Authentication

When authentication is enabled, API requests require authentication:
- **BasicAuth**: Use `Authorization: Basic base64(username:password)` header
- **OIDC**: Requires valid session cookie

Unauthenticated API requests will return a 401 error.

## 🔒 Security Notes

- Using iptables or ipset requires root privileges
- Production environments should use database storage
- Regularly backup configuration files and data
- Do not commit configuration files with sensitive information to version control

## 🤝 Contributing

Issues and Pull Requests are welcome!

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📞 Support

- Check [Issues](https://github.com/graydovee/netbouncer/issues)
- Create a new Issue
- See [CONFIGURATION.md](doc/CONFIGURATION.md) for detailed configuration
- See [API.md](doc/API.md) for API interface

---

**NetBouncer** - Making network monitoring simple and efficient 🚀
