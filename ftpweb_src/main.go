package main

import (
    "bufio"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "os/user"
    "path/filepath"
    "strings"

    "github.com/msteinert/pam"
)

var loginPage = template.Must(template.New("login").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>FTP Login</title>
<style>
body {
    background: #1e1e1e;
    color: #ddd;
    font-family: Arial, sans-serif;
    display: flex;
    justify-content: center;
    align-items: center;
    height: 100vh;
}
form {
    background: #2b2b2b;
    padding: 30px;
    border-radius: 8px;
    box-shadow: 0 0 10px #000;
}
input {
    width: 100%;
    padding: 10px;
    margin: 8px 0;
    border-radius: 4px;
    border: none;
}
button {
    width: 100%;
    padding: 10px;
    background: #4CAF50;
    border: none;
    color: white;
    font-size: 16px;
    border-radius: 4px;
}
</style>
</head>
<body>
<form method="POST" action="/login">
    <h2>FTP Login</h2>
    <input type="text" name="username" placeholder="Username" required autofocus>
    <input type="password" name="password" placeholder="Password" required>
    <button type="submit">Login</button>
</form>
</body>
</html>
`))

var indexPage = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>{{.User}} FTP</title>
<style>
body {
    background: #121212;
    color: #eee;
    font-family: Arial, sans-serif;
    padding: 20px;
}
a {
    color: #4CAF50;
    text-decoration: none;
}
a:hover {
    text-decoration: underline;
}
.file {
    margin: 5px 0;
}
</style>
</head>
<body>
<h2>FTP Directory for {{.User}}</h2>
{{range .Files}}
<div class="file"><a href="/file/{{$.User}}/{{.Path}}">{{.Name}}</a></div>
{{end}}
</body>
</html>
`))

// PAM authentication
func pamAuth(username, password string) error {
    t, err := pam.StartFunc("login", username, func(s pam.Style, msg string) (string, error) {
        switch s {
        case pam.PromptEchoOff:
            return password, nil
        case pam.PromptEchoOn:
            return password, nil
        case pam.ErrorMsg, pam.TextInfo:
            return "", nil
        }
        return "", nil
    })
    if err != nil {
        return err
    }
    return t.Authenticate(0)
}

// Check if user is in /etc/vsftpd/user_list
func userAllowed(username string) bool {
    f, err := os.Open("/etc/vsftpd/user_list")
    if err != nil {
        return false
    }
    defer f.Close()

    sc := bufio.NewScanner(f)
    for sc.Scan() {
        if strings.TrimSpace(sc.Text()) == username {
            return true
        }
    }
    return false
}

// List files AND directories
type FileEntry struct {
    Name string
    Path string
}

func listFiles(base, dir string) ([]FileEntry, error) {
    var files []FileEntry
    entries, err := os.ReadDir(dir)
    if err != nil {
        return files, err
    }
    for _, e := range entries {
        rel, _ := filepath.Rel(base, filepath.Join(dir, e.Name()))
        files = append(files, FileEntry{
            Name: e.Name(),
            Path: rel,
        })
    }
    return files, nil
}

func main() {
    // Landing page
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        loginPage.Execute(w, nil)
    })

    // Login handler
    http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Redirect(w, r, "/", http.StatusSeeOther)
            return
        }

        username := r.FormValue("username")
        password := r.FormValue("password")

        if err := pamAuth(username, password); err != nil {
            http.Error(w, "Authentication failed", http.StatusUnauthorized)
            return
        }

        if !userAllowed(username) {
            http.Error(w, "User not allowed", http.StatusForbidden)
            return
        }

        http.SetCookie(w, &http.Cookie{
            Name:  "auth_user",
            Value: username,
            Path:  "/",
        })

        http.Redirect(w, r, "/home", http.StatusSeeOther)
    })

    // Home autoindex
    http.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
        c, err := r.Cookie("auth_user")
        if err != nil {
            http.Redirect(w, r, "/", http.StatusSeeOther)
            return
        }

        usr, err := user.Lookup(c.Value)
        if err != nil {
            http.Error(w, "User not found", http.StatusInternalServerError)
            return
        }

        base := filepath.Join(usr.HomeDir, "ftp")
        files, err := listFiles(base, base)
        if err != nil {
            http.Error(w, "Cannot read FTP directory", http.StatusInternalServerError)
            return
        }

        indexPage.Execute(w, map[string]interface{}{
            "User":  c.Value,
            "Files": files,
        })
    })

    // File and directory handler
    http.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
        c, err := r.Cookie("auth_user")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        parts := strings.Split(r.URL.Path, "/")
        if len(parts) < 4 {
            http.NotFound(w, r)
            return
        }

        userReq := parts[2]
        if userReq != c.Value {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        usr, _ := user.Lookup(userReq)
        base := filepath.Join(usr.HomeDir, "ftp")
        relPath := strings.Join(parts[3:], "/")
        target := filepath.Join(base, relPath)

        info, err := os.Stat(target)
        if err != nil {
            http.NotFound(w, r)
            return
        }

        if info.IsDir() {
            files, _ := listFiles(base, target)
            indexPage.Execute(w, map[string]interface{}{
                "User":  userReq,
                "Files": files,
            })
            return
        }

        http.ServeFile(w, r, target)
    })

    log.Println("Listening on :5663")
    http.ListenAndServe(":5663", nil)
}

