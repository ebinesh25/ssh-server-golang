package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const portfolioURL = "https://www.ebinesh.in/json/"

type portfolioData map[string]any
type fileSystem map[string]string

func fetchPortfolio() (portfolioData, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(portfolioURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("portfolio endpoint returned %s", resp.Status)
	}
	var data portfolioData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func ensureHostKey(filename string) error {
	if _, err := os.Stat(filename); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	block, err := xssh.MarshalPrivateKey(privateKey, "Ebinesh portfolio SSH host key")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) { // Another server process created it first.
			return nil
		}
		return err
	}
	defer file.Close()
	return pem.Encode(file, block)
}

func pretty(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b) + "\n"
}

func makeFileSystem(data portfolioData) fileSystem {
	files := fileSystem{}
	put := func(name string, value any) { files["/"+name] = pretty(value) }
	put("about.json", map[string]any{"name": data["name"], "role": data["role"], "location": data["location"], "copy": nested(data, "pageCopy", "home")})
	put("contact.json", data["contact"])
	put("experience.json", data["workExperience"])
	put("skills.json", data["technologies"])
	put("projects.json", data["projectGroups"])
	put("work.json", data["work"])
	put("home.json", data["homepageHighlights"])
	put("now.json", nested(data, "pageCopy", "now"))
	put("uses.json", nested(data, "pageCopy", "uses"))

	if blog, ok := data["blog"].(map[string]any); ok {
		if articles, ok := blog["articles"].([]any); ok {
			for i, raw := range articles {
				article, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				title, _ := article["title"].(string)
				link, _ := article["url"].(string)
				if strings.HasPrefix(link, "/") {
					link = "https://www.ebinesh.in" + link
				}
				name, _ := article["slug"].(string)
				if name == "" {
					name = fmt.Sprintf("article-%02d", i+1)
				}
				files["/blogs/"+name+".md"] = fmt.Sprintf("# %s\n\n%s\n\n%s · %s\n\n%s\n\nOpen: %s\n", title, valueString(article["description"]), valueString(article["publication"]), valueString(article["readingTime"]), valueString(article["category"]), link)
			}
		}
	}
	return files
}

func nested(data map[string]any, parent, key string) any {
	if v, ok := data[parent].(map[string]any); ok {
		return v[key]
	}
	return nil
}
func valueString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func normalize(cwd, name string) string {
	if strings.HasPrefix(name, "/") {
		return path.Clean(name)
	}
	return path.Clean(path.Join(cwd, name))
}

func listDir(files fileSystem, dir string) []string {
	prefix := strings.TrimSuffix(dir, "/")
	if prefix == "." || prefix == "" {
		prefix = "/"
	}
	if prefix != "/" {
		prefix += "/"
	}
	entries := map[string]bool{}
	for file := range files {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		rest := strings.TrimPrefix(file, prefix)
		if rest == "" {
			continue
		}
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			entries[rest[:i]+"/"] = true
		} else {
			entries[rest] = false
		}
	}
	out := make([]string, 0, len(entries))
	for entry := range entries {
		out = append(out, entry)
	}
	sort.Strings(out)
	return out
}

func help() string {
	return "Commands: ls [path], cd [path], pwd, cat <file>, help, clear, exit\n" +
		"Try: ls / ; cd blogs ; ls ; cat <article>.md\n"
}

func respond(files fileSystem, cwd string, input string, out io.Writer) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return cwd, true
	}
	switch parts[0] {
	case "help", "/help", "?":
		fmt.Fprint(out, help())
	case "exit", "/exit", "quit":
		fmt.Fprintln(out, "Goodbye!")
		return cwd, false
	case "clear", "/clear":
		fmt.Fprint(out, "\033[2J\033[H")
	case "pwd":
		fmt.Fprintln(out, cwd)
	case "ls":
		dir := cwd
		if len(parts) > 1 {
			dir = normalize(cwd, parts[len(parts)-1])
		}
		if _, ok := files[dir]; ok {
			fmt.Fprintln(out, path.Base(dir))
			break
		}
		entries := listDir(files, dir)
		if len(entries) == 0 {
			fmt.Fprintf(out, "ls: %s: no such directory\n", dir)
			break
		}
		fmt.Fprintln(out, strings.Join(entries, "  "))
	case "cd":
		dest := "/"
		if len(parts) > 1 {
			dest = normalize(cwd, parts[1])
		}
		if dest == "/" || len(listDir(files, dest)) > 0 {
			cwd = dest
		} else {
			fmt.Fprintf(out, "cd: %s: no such directory\n", dest)
		}
	case "cat":
		if len(parts) < 2 {
			fmt.Fprintln(out, "usage: cat <file>")
			break
		}
		file := normalize(cwd, parts[1])
		if body, ok := files[file]; ok {
			fmt.Fprint(out, body)
		} else {
			fmt.Fprintf(out, "cat: %s: no such file\n", parts[1])
		}
	default:
		fmt.Fprintf(out, "%s: command not found (try help)\n", parts[0])
	}
	return cwd, true
}

func main() {
	hostKey := os.Getenv("SSH_HOST_KEY")
	if hostKey == "" {
		hostKey = "ssh_host_ed25519"
	}
	if err := ensureHostKey(hostKey); err != nil {
		log.Fatalf("Could not prepare SSH host key %q: %v", hostKey, err)
	}

	data, err := fetchPortfolio()
	if err != nil {
		log.Printf("Could not load %s: %v", portfolioURL, err)
		data = portfolioData{"name": "Ebinesh A", "role": "Fullstack developer", "location": "Chennai, India", "contact": map[string]any{"email": "ebinesh2511@gmail.com", "linkedin": "https://linkedin.com/in/ebinesh/", "github": "https://github.com/ebinesh25", "medium": "https://medium.com/@positron25"}}
	}
	files := makeFileSystem(data)

	ssh.Handle(func(s ssh.Session) {
		term := terminal.NewTerminal(s, "ebinesh.in@portfolio:")
		fmt.Fprintln(term, "Welcome to Ebinesh's portfolio. Type help to explore.")
		cwd := "/"
		for {
			line, err := term.ReadLine()
			if err != nil {
				return
			}
			cwd, ok := respond(files, cwd, strings.TrimSpace(line), term)
			if !ok {
				return
			}
			term.SetPrompt("ebinesh.in@portfolio:" + cwd + "$ ")
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "2222"
	}
	log.Printf("Starting portfolio SSH server on port %s", port)
	log.Fatal(ssh.ListenAndServe(":"+port, nil, ssh.HostKeyFile(hostKey)))
}
