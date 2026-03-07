#!/bin/bash

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
  echo "Error: Please run as root (use sudo)."
  exit 1
fi

# Function to process a single user
process_user() {
    local username=$1
    local password=$2

    echo "------------------------------------------"
    echo "Processing user: $username"

    # 1. Create the user with no shell access
    if id "$username" &>/dev/null; then
        echo "User $username already exists. Skipping creation."
    else
        useradd -m -s /usr/sbin/nologin "$username"
        echo "User $username created."
    fi

    # 2. Set the password (non-interactive)
    echo "$username:$password" | chpasswd
    echo "Password set for $username."

    # 3. Create the FTP directory and set permissions
    mkdir -p "/home/$username/ftp"
    chown "$username:$username" "/home/$username/ftp"
    chmod 755 "/home/$username/ftp"
    echo "Directory /home/$username/ftp created and permissions set (755)."

    # 4. Add user to vsftpd user_list
    if ! grep -q "^$username$" /etc/vsftpd/user_list; then
        echo "$username" >> /etc/vsftpd/user_list
        echo "User added to /etc/vsftpd/user_list."
    fi

    # 5. Create the user-specific vsftpd config
    USER_CONFIG_FILE="/etc/vsftpd/user_config/$username"
    echo "local_root=/home/$username/ftp" > "$USER_CONFIG_FILE"
    echo "Config file created: $USER_CONFIG_FILE"
}

# Check if a file was provided as an argument, otherwise ask for input
if [ -n "$1" ] && [ -f "$1" ]; then
    echo "Reading from file: $1"
    while read -r user pass; do
        [[ -z "$user" || "$user" =~ ^# ]] && continue
        process_user "$user" "$pass"
    done < "$1"
else
    echo "No file provided. Enter your list (format: username password)."
    echo "Press Ctrl+D when finished:"
    while read -r user pass; do
        [[ -z "$user" ]] && break
        process_user "$user" "$pass"
    done
fi

echo "------------------------------------------"
echo "All tasks complete."

