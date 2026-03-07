package main

import (
    "bufio"
    "html/template"
    "log"
    "net/http"
    "os"
    "os/exec"
    "strings"

    "github.com/msteinert/pam"
)

var tpl = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
<title>FTP Admin</title>
<style>
body {
    font-family: Arial, sans-serif;
    background: #f4f4f7;
    margin: 0;
    padding: 0;
}
.container {
    width: 600px;
    margin: 40px auto;
    background: white;
    padding: 25px;
    border-radius: 10px;
    box-shadow: 0 0 10px rgba(0,0,0,0.1);
}
h2 {
    margin-top: 0;
    color: #333;
}
form {
    margin-bottom: 30px;
}
input[type=text], input[type=password], input[type=file] {
    width: 100%;
    padding: 10px;
    margin: 6px 0 12px 0;
    border: 1px solid #ccc;
    border-radius: 5px;
}
button {
    background: #007bff;
    color: white;
    padding: 10px 18px;
    border: none;
    border-radius: 5px;
    cursor: pointer;
}
button:hover {
    background: #0056b3;
}
.notice {
    padding: 10px;
    background: #ffe8e8;
    border-left: 4px solid #ff4d4d;
    margin-bottom: 20px;
}
</style>
</head>
<body>
<div class="container">

<h2>Create FTP User</h2>
{{if .DisableReg}}
<div class="notice"><b>Registration disabled</b> (/.disablereg exists)</div>
{{else}}
<form method="POST" action="/create">
  <label>Username:</label>
  <input name="username" required>

  <label>Password:</label>
  <input name="password" type="password" required>

  <button type="submit">Create User</button>
</form>

<h3>Bulk Upload Users</h3>
<form method="POST" action="/upload" enctype="multipart/form-data">
  <label>Select file (format: username password per line):</label>
  <input type="file" name="userfile" required>
  <button type="submit">Upload & Process</button>
</form>
{{end}}

<hr>

<h2>Change Password</h2>
<form method="POST" action="/changepw">
  <label>Username:</label>
  <input name="username" required>

  <label>Current Password:</label>
  <input name="oldpw" type="password" required>

  <label>New Password:</label>
  <input name="newpw" type="password" required>

  <button type="submit">Change Password</button>
</form>

</div>
</body>
</html>
`))

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/create", createHandler)
    http.HandleFunc("/changepw", changePwHandler)
    http.HandleFunc("/upload", uploadHandler)

    log.Println("Serving on :5662")
    log.Fatal(http.ListenAndServe("127.0.0.1:5662", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    _, disable := os.Stat("/.disablereg")
    tpl.Execute(w, map[string]bool{"DisableReg": disable == nil})
}

func createHandler(w http.ResponseWriter, r *http.Request) {
    if _, err := os.Stat("/.disablereg"); err == nil {
        http.Error(w, "Registration disabled", 403)
        return
    }

    if r.Method != "POST" {
        http.Redirect(w, r, "/", 302)
        return
    }

    user := r.FormValue("username")
    pass := r.FormValue("password")

    if user == "" || pass == "" {
        http.Error(w, "Missing username or password", 400)
        return
    }

    processUser(user, pass)

    w.Write([]byte("User created successfully"))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
    if _, err := os.Stat("/.disablereg"); err == nil {
        http.Error(w, "Registration disabled", 403)
        return
    }

    if r.Method != "POST" {
        http.Redirect(w, r, "/", 302)
        return
    }

    file, _, err := r.FormFile("userfile")
    if err != nil {
        http.Error(w, "File upload error", 400)
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.Fields(line)
        if len(parts) != 2 {
            continue
        }
        processUser(parts[0], parts[1])
    }

    exec.Command("systemctl", "reload", "vsftpd").Run()

    w.Write([]byte("Bulk user import complete"))
}

func changePwHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Redirect(w, r, "/", 302)
        return
    }

    user := r.FormValue("username")
    oldpw := r.FormValue("oldpw")
    newpw := r.FormValue("newpw")

    if user == "" || oldpw == "" || newpw == "" {
        http.Error(w, "Missing fields", 400)
        return
    }

    // PAM authentication
    t, err := pam.StartFunc("login", user, func(s pam.Style, msg string) (string, error) {
        return oldpw, nil
    })
    if err != nil || t.Authenticate(0) != nil {
        http.Error(w, "Authentication failed", 403)
        return
    }

    appendIfMissing("/etc/vsftpd/user_list", user)

    chpasswd := exec.Command("chpasswd")
    chpasswd.Stdin = strings.NewReader(user + ":" + newpw)
    chpasswd.Run()

    w.Write([]byte("Password changed successfully"))
}

func processUser(user, pass string) {
    if !userExists(user) {
        exec.Command("useradd", "-m", "-s", "/usr/sbin/nologin", user).Run()
    }

    chpasswd := exec.Command("chpasswd")
    chpasswd.Stdin = strings.NewReader(user + ":" + pass)
    chpasswd.Run()

    os.MkdirAll("/home/"+user+"/ftp", 0755)
    exec.Command("chown", "-R", user+":"+user, "/home/"+user+"/ftp").Run()

    appendIfMissing("/etc/vsftpd/user_list", user)

    os.WriteFile("/etc/vsftpd/user_config/"+user, []byte("local_root=/home/"+user+"/ftp\n"), 0644)
}

func userExists(username string) bool {
    cmd := exec.Command("id", username)
    return cmd.Run() == nil
}

func appendIfMissing(path, user string) {
    data, _ := os.ReadFile(path)
    lines := strings.Split(string(data), "\n")
    for _, l := range lines {
        if l == user {
            return
        }
    }
    f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
    defer f.Close()
    f.WriteString(user + "\n")
}

