# VSFTPD-manager

This repository provides a lightweight FTP environment built around **vsftpd**, with two Go web services and helper scripts for installation and user management.

The stack provides:

- FTP server using **vsftpd**
- Web-based FTP file browser for users
- Web-based FTP administration panel
- Automated Fedora setup
- CLI bulk user creation
- Nginx reverse proxy integration

The system is designed for **Fedora-based servers** and relies on **Linux system users** for authentication.

---

# Architecture

The stack consists of four components.

## 1. FTP Server (vsftpd)

The FTP server is powered by **vsftpd** and configured using the provided setup script.

Features:

- Local users only
- Per-user FTP directories
- Chrooted users
- Passive FTP configuration
- Firewall and SELinux configuration
- Per-user vsftpd configuration

Each FTP user receives the directory:

```

/home/<username>/ftp

```

User configuration files are stored in:

```

/etc/vsftpd/user_config/<username>

```

Allowed users are listed in:

```

/etc/vsftpd/user_list

```

---

# 2. Web FTP Browser (Go)

A lightweight web interface that allows users to browse and download files from their FTP directory.

Internal port:

```

127.0.0.1:5663

```

Features:

- PAM authentication using system accounts
- Access limited to users listed in `/etc/vsftpd/user_list`
- Directory browsing
- File downloads
- Recursive navigation
- Minimal dark UI

Authentication flow:

1. User logs in with Linux username/password
2. PAM validates credentials
3. User must exist in `/etc/vsftpd/user_list`
4. A session cookie is created
5. User is redirected to their FTP directory

File access is restricted to:

```

/home/<user>/ftp

```

Example route:

```

/file/<user>/<path>

```

---

# 3. Web FTP Admin Panel (Go)

A simple administration interface used to manage FTP users.

Internal port:

```

127.0.0.1:5662

```

Functions include:

### Create User

Creates a new Linux user and prepares FTP access:

- Creates Linux user
- Sets password
- Creates `/home/<user>/ftp`
- Adds user to `/etc/vsftpd/user_list`
- Creates vsftpd user configuration

### Bulk User Import

Upload file format:

```

username password
username password

```

The system automatically creates each user.

### Change Password

Uses PAM authentication:

1. User submits username and current password
2. PAM verifies credentials
3. Password is updated with `chpasswd`

### Registration Control

Creating the file:

```

/.disablereg

```

will disable user creation from the web interface.

---

# 4. Nginx Reverse Proxy

The system uses **nginx** as the public entry point.

Nginx listens on:

```

port 5661

````

and routes traffic to the Go services.

### Nginx Configuration

```
server {
    listen 5661;

    location / {
        proxy_pass http://127.0.0.1:5663;
    }
    
    location /manage {
        proxy_pass http://127.0.0.1:5662;
    }

    location /changepw {
        proxy_pass http://127.0.0.1:5662;
    }

    location /create {
        auth_basic "Restricted";
        auth_basic_user_file /etc/nginx/.ftpadmin;
        proxy_pass http://127.0.0.1:5662;
    }

    location /upload {
        auth_basic "Restricted";
        auth_basic_user_file /etc/nginx/.ftpadmin;
        proxy_pass http://127.0.0.1:5662;
    }
}
````

### Route Behavior

| Route       | Purpose                      |
| ----------- | ---------------------------- |
| `/`         | FTP web browser              |
| `/manage`   | Admin interface              |
| `/changepw` | Password change              |
| `/create`   | User creation (protected)    |
| `/upload`   | Bulk user import (protected) |

User creation and bulk import are protected using **HTTP Basic Authentication**.

Credentials are stored in:

```
/etc/nginx/.ftpadmin
```

Example creation:

```
sudo htpasswd -c /etc/nginx/.ftpadmin admin
```

---

# Helper Scripts

## FTP Server Setup Script

This script installs and configures **vsftpd** for Fedora.

Actions performed:

* Installs vsftpd
* Writes `/etc/vsftpd/vsftpd.conf`
* Configures passive FTP ports
* Creates required directories
* Configures firewall
* Sets SELinux booleans
* Initializes user list
* Removes PAM shell restriction

Run with:

```
sudo ./scripts/install.sh
```

---

## Bulk User CLI Script

Alternative CLI tool for creating users.

Supports:

* Import from file
* Interactive user creation

Example file:

```
user1 password1
user2 password2
```

Run:

```
sudo ./scripts/users.sh users.txt
```

or interactive:

```
sudo ./scripts/users.sh
```

---

# Directory Layout

```
vsftpd-manager/

ftpweb/
    main.go

ftpadmin/
    main.go

scripts/
    setup.sh
    bulk_users.sh

ftpadminweb-nginx.conf

README.md
```

---

# Requirements

Tested on:

```
Fedora Linux
```

Required packages:

```
vsftpd
nginx
firewalld
policycoreutils
```

Go version:

```
Go 1.25+
```

Go dependency:

```
github.com/msteinert/pam
```

---

# Building the Go Services

Build the FTP browser:

```
cd ftpweb
go build
```

Build the admin service:

```
cd ftpadmin
go build
```

---

# Running the Services

Start the services locally:

```
./ftpweb
./ftpadmin
```

Nginx will expose them through port:

```
5661
```

Example access:

```
http://server:5661
```

Admin panel:

```
http://server:5661/manage
```

---

# Security Notes

Important recommendations:

* Always run nginx in front of the Go services
* Restrict access to `/create` and `/upload`
* Use HTTPS in production
* Monitor `/etc/vsftpd/user_list`
* Limit firewall exposure

Users are restricted to their own directory:

```
/home/<user>/ftp
```

The web file browser validates user identity before allowing file access.

---

# Typical Workflow

1. Run setup script
2. Configure nginx
3. Start vsftpd
4. Build and run both Go services
5. Create users via `/manage`
6. Users connect using FTP clients
7. Users browse files via web browser

---
# Systemd Services

Both Go services are intended to run as **systemd services** so they automatically start on boot and restart if they crash.

The services run locally and are exposed externally through **nginx**.

Service files should be placed in:

```

/etc/systemd/system/

```

---

# ftpweb Service

This service runs the **FTP web browser**.

Service file:

```

/etc/systemd/system/ftpweb.service

````

```
[Unit]
Description=ftpweb
After=network.target

[Service]
Type=simple
WorkingDirectory=/root/ftpweb
ExecStart=/root/ftpweb/ftpweb
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
````

---

# ftpadmin Service

This service runs the **FTP administration panel**.

Service file:

```
/etc/systemd/system/ftpadmin.service
```

```
[Unit]
Description=ftpadmin
After=network.target

[Service]
Type=simple
WorkingDirectory=/root/ftpadmin
ExecStart=/root/ftpadmin/ftpadmin
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
```

---

# Enable and Start Services

Reload systemd after creating the service files:

```
sudo systemctl daemon-reload
```

Enable services at boot:

```
sudo systemctl enable ftpweb
sudo systemctl enable ftpadmin
```

Start the services:

```
sudo systemctl start ftpweb
sudo systemctl start ftpadmin
```

Check service status:

```
systemctl status ftpweb
systemctl status ftpadmin
```

---

# SELinux Configuration

Since Fedora uses **SELinux**, the binaries must be labeled so systemd can execute them.

Label the binaries as `bin_t`:

```
sudo chcon -t bin_t /root/ftpweb/ftpweb
sudo chcon -t bin_t /root/ftpadmin/ftpadmin
```

To make the change persistent across relabels:

```
sudo semanage fcontext -a -t bin_t "/root/ftpweb/ftpweb"
sudo semanage fcontext -a -t bin_t "/root/ftpadmin/ftpadmin"

sudo restorecon -v /root/ftpweb/ftpweb
sudo restorecon -v /root/ftpadmin/ftpadmin
```

If `semanage` is not installed:

```
sudo dnf install policycoreutils-python-utils
```

# License

GPLv3
