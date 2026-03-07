#!/bin/bash

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
  echo "Please run as root (use sudo)."
  exit 1
fi

echo "Installing vsftpd..."
dnf install vsftpd -y

echo "Configuring /etc/vsftpd/vsftpd.conf..."
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
local_root=/opt/stacks/ftp
allow_writeable_chroot=YES
pasv_enable=YES
pasv_min_port=50000
pasv_max_port=51000
user_config_dir=/etc/vsftpd/user_config/
EOF

echo "Modifying PAM and User List..."
# Remove the pam_shells.so requirement
sed -i '/auth       required    pam_shells.so/d' /etc/pam.d/vsftpd

# Clear the user_list file
rm -f /etc/vsftpd/user_list
> /etc/vsftpd/user_list

echo "Creating directories and setting permissions..."
mkdir -p /etc/vsftpd/user_config/
mkdir -p /opt/stacks/ftp

echo "Configuring Firewall..."
firewall-cmd --permanent --add-port=21/tcp
firewall-cmd --permanent --add-port=50000-51000/tcp
firewall-cmd --reload

echo "Setting SELinux booleans..."
setsebool -P ftpd_full_access 1

echo "Setup complete."

