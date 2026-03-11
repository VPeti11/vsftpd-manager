#!/bin/bash

set -e

if [ "$EUID" -ne 0 ]; then
  echo "Run as root: sudo ./scripts/install.sh"
  exit 1
fi

BASE_DIR=$(pwd)

FTPWEB_BIN="$BASE_DIR/ftpweb"
FTPADMIN_BIN="$BASE_DIR/ftpadmin_localhost"

NGINX_CONF="/etc/nginx/conf.d/ftpadminweb.conf"

echo "Using project directory:"
echo "$BASE_DIR"

echo "Installing required packages..."

dnf install -y \
vsftpd \
nginx \
firewalld \
policycoreutils \
policycoreutils-python-utils \
httpd-tools

echo "Ensuring binaries exist..."

if [ ! -f "$FTPWEB_BIN" ]; then
    echo "Missing ftpweb binary"
    exit 1
fi

if [ ! -f "$FTPADMIN_BIN" ]; then
    echo "Missing ftpadmin_localhost binary"
    exit 1
fi

echo "Setting executable permissions..."

chmod +x "$FTPWEB_BIN"
chmod +x "$FTPADMIN_BIN"

echo "Applying SELinux labels..."

semanage fcontext -a -t bin_t "$FTPWEB_BIN" || true
semanage fcontext -a -t bin_t "$FTPADMIN_BIN" || true

restorecon -v "$FTPWEB_BIN"
restorecon -v "$FTPADMIN_BIN"

echo "Configuring vsftpd..."

rm -f /etc/vsftpd/vsftpd.conf

cat <<EOF > /etc/vsftpd/vsftpd.conf
anonymous_enable=NO
local_enable=YES
write_enable=YES
local_umask=022
dirmessage_enable=YES
xferlog_enable=YES
connect_from_port_20=YES
xferlog_std_format=YES
chroot_local_user=YES
listen=YES
listen_ipv6=NO
pam_service_name=vsftpd
userlist_enable=YES
userlist_deny=NO
allow_writeable_chroot=YES
pasv_enable=YES
pasv_min_port=50000
pasv_max_port=51000
user_config_dir=/etc/vsftpd/user_config/
EOF

echo "Adjusting PAM..."

sed -i '/pam_shells.so/d' /etc/pam.d/vsftpd

echo "Preparing vsftpd directories..."

mkdir -p /etc/vsftpd/user_config
touch /etc/vsftpd/user_list

echo "Configuring firewall..."

systemctl enable firewalld
systemctl start firewalld

firewall-cmd --permanent --add-service=ftp
firewall-cmd --permanent --add-port=5661/tcp
firewall-cmd --permanent --add-port=50000-51000/tcp
firewall-cmd --reload

echo "Setting SELinux booleans..."

setsebool -P ftpd_full_access 1

echo "Installing nginx configuration..."

cat <<EOF > $NGINX_CONF
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
EOF

echo "Creating systemd service for ftpweb..."

cat <<EOF > /etc/systemd/system/ftpweb.service
[Unit]
Description=FTP Web Browser
After=network.target

[Service]
Type=simple
WorkingDirectory=$BASE_DIR
ExecStart=$FTPWEB_BIN
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

echo "Creating systemd service for ftpadmin..."

cat <<EOF > /etc/systemd/system/ftpadmin.service
[Unit]
Description=FTP Admin Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=$BASE_DIR
ExecStart=$FTPADMIN_BIN
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

echo "Reloading systemd..."

systemctl daemon-reload

echo "Enabling services..."

systemctl enable vsftpd
systemctl enable nginx
systemctl enable ftpweb
systemctl enable ftpadmin

echo "Starting services..."

systemctl restart vsftpd
systemctl restart nginx
systemctl restart ftpweb
systemctl restart ftpadmin

echo ""
echo "Installation complete."
echo ""
echo "Create nginx admin account:"
htpasswd -c /etc/nginx/.ftpadmin admin
echo ""
echo "Open:"
echo "http://SERVER-IP:5661"
echo ""
echo "Admin panel:"
echo "http://SERVER-IP:5661/manage"
echo ""
