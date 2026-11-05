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
	figure "github.com/common-nighthawk/go-figure"
	"golang.org/x/crypto/ssh/terminal"
)

const portfolioURL = "https://www.ebinesh.in/json/"

type portfolioData map[string]any
type fileSystem map[string]string

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	blue   = "\033[34m"
	green  = "\033[32m"
	yellow = "\033[33m"
	muted  = "\033[90m"
)

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

func printBanner(out io.Writer, text string) {
	art := figure.NewFigure(strings.ToUpper(text), "small", true)
	fmt.Fprintln(out, cyan+bold+art.String()+reset)
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

func renderBlog(body string) string {
	var rendered strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			fmt.Fprintf(&rendered, "%s%s%s\n", yellow+bold, line, reset)
		} else if strings.HasPrefix(line, "Open: ") {
			url := strings.TrimPrefix(line, "Open: ")
			fmt.Fprintf(&rendered, "Open: \033]8;;%s\033\\%s%s%s\033]8;;\033\\\n", url, cyan+bold, url, reset)
		} else {
			rendered.WriteString(line + "\n")
		}
	}
	return rendered.String()
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

func isDir(files fileSystem, dir string) bool {
	if dir == "/" || dir == "." {
		return true
	}
	prefix := strings.TrimSuffix(dir, "/") + "/"
	for file := range files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

func colorEntry(files fileSystem, dir, entry string) string {
	fullPath := normalize(dir, strings.TrimSuffix(entry, "/"))
	if strings.HasSuffix(entry, "/") || isDir(files, fullPath) {
		return cyan + bold + "▸ " + entry + reset
	}
	return green + "· " + entry + reset
}

func printTree(files fileSystem, dir, prefix string, out io.Writer, depth int) {
	entries := listDir(files, dir)
	for i, entry := range entries {
		last := i == len(entries)-1
		branch := "├── "
		childPrefix := prefix + "│   "
		if last {
			branch = "└── "
			childPrefix = prefix + "    "
		}
		fmt.Fprintf(out, "%s%s%s\n", prefix, branch, colorEntry(files, dir, entry))
		if strings.HasSuffix(entry, "/") && depth > 0 {
			printTree(files, normalize(dir, strings.TrimSuffix(entry, "/")), childPrefix, out, depth-1)
		}
	}
}

func banner(out io.Writer) {
	printBanner(out, "Ebinesh A")
	fmt.Fprintf(out, "\n%sChennai, India%s  %s·%s  Portfolio, served from the terminal\n\n", muted, reset, muted, reset)
	fmt.Fprintf(out, "%sTry:%s %ssections%s  %stree%s  %shelp%s\n\n", bold, reset, green, reset, green, reset, green, reset)
}

func help() string {
	return cyan + bold + "Explore\n" + reset +
		"  " + green + "sections" + reset + "             Show the portfolio sections\n" +
		"  " + green + "tree [path]" + reset + "         Browse the portfolio as a tree\n" +
		"  " + green + "ls [path]" + reset + "           List files and directories\n" +
		"  " + green + "cd [path]" + reset + "           Move around (try cd blogs, cd .., cd /)\n" +
		"  " + green + "cat <file>" + reset + "          Read a section or blog card\n" +
		"  " + green + "open <blog.md>" + reset + "      Print a clickable blog link\n" +
		"  " + green + "pwd" + reset + "                  Show your current directory\n\n" +
		"  " + yellow + "clear" + reset + "  clear screen     " + yellow + "exit" + reset + "  leave the portfolio\n\n" +
		muted + "Paths work like a regular shell. Tab completion and host commands are intentionally disabled." + reset + "\n"
}

func respond(files fileSystem, cwd string, input string, out io.Writer) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return cwd, true
	}
	switch parts[0] {
	case "help", "/help", "?":
		fmt.Fprint(out, help())
	case "banner":
		banner(out)
	case "sections":
		fmt.Fprintln(out, cyan+bold+"  Portfolio sections"+reset)
		for _, entry := range listDir(files, "/") {
			fmt.Fprintf(out, "  %s\n", colorEntry(files, "/", entry))
		}
	case "tree":
		dir := cwd
		if len(parts) > 1 {
			dir = normalize(cwd, parts[1])
		}
		if !isDir(files, dir) {
			fmt.Fprintf(out, "tree: %s: no such directory\n", dir)
			break
		}
		fmt.Fprintf(out, "%s%s%s\n", cyan+bold, dir, reset)
		printTree(files, dir, "", out, 12)
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
			fmt.Fprintf(out, "%s\n", green+path.Base(dir)+reset)
			break
		}
		entries := listDir(files, dir)
		if len(entries) == 0 {
			fmt.Fprintf(out, "ls: %s: no such directory\n", dir)
			break
		}
		for _, entry := range entries {
			fmt.Fprintf(out, "%s  ", colorEntry(files, dir, entry))
		}
		fmt.Fprintln(out)
	case "cd":
		dest := "/" // No argument returns to the portfolio root.
		if len(parts) > 1 {
			target := parts[1]
			if target == "~" {
				target = "/"
			}
			dest = normalize(cwd, target)
		}
		if isDir(files, dest) {
			cwd = dest
			fmt.Fprintf(out, "%s%s%s\n", muted, cwd, reset)
		} else {
			if _, isFile := files[dest]; isFile {
				fmt.Fprintf(out, "cd: %s: is a file; try cat %s\n", dest, path.Base(dest))
			} else {
				fmt.Fprintf(out, "cd: %s: no such directory\n", dest)
			}
		}
	case "cat":
		if len(parts) < 2 {
			fmt.Fprintln(out, "usage: cat <file>")
			break
		}
		file := normalize(cwd, parts[1])
		if body, ok := files[file]; ok {
			if strings.HasSuffix(file, ".md") {
				fmt.Fprint(out, renderBlog(body))
			} else {
				fmt.Fprint(out, body)
			}
		} else {
			fmt.Fprintf(out, "cat: %s: no such file\n", parts[1])
		}
	case "open":
		if len(parts) < 2 {
			fmt.Fprintln(out, "usage: open <blog.md>")
			break
		}
		file := normalize(cwd, parts[1])
		body, ok := files[file]
		if !ok || !strings.HasSuffix(file, ".md") {
			fmt.Fprintf(out, "open: %s: no blog link found\n", parts[1])
			break
		}
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "Open: ") {
				url := strings.TrimPrefix(line, "Open: ")
				fmt.Fprintf(out, "\033]8;;%s\033\\%s%s%s\033]8;;\033\\\n", url, cyan+bold, url, reset)
				break
			}
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
		fmt.Fprint(term, "\033[2J\033[H")
		banner(term)
		fmt.Fprintf(term, "%sType %shelp%s to see all commands. Use your terminal's scrollback to browse.%s\n\n", muted, green, muted, reset)
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
			term.SetPrompt("ebinesh.in:" + cwd + "$ ")
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "2222"
	}
	log.Printf("Starting portfolio SSH server on port %s", port)
	log.Fatal(ssh.ListenAndServe(":"+port, nil, ssh.HostKeyFile(hostKey)))
}
