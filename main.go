package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/webdav"
)

const (
	fn_cert_pem = "cert.pem"
	fn_key_pem  = "key.pem"
)

var dir string

func main() {

	dirFlag := flag.String("d", "./", "Directory to serve from. Default is CWD")
	httpPort := flag.Int("p", 80, "Port to serve on HTTP/HTTPS")
	serveSecure := flag.Bool("s", false, "Serve HTTPS. Default false")
	flagUserName := flag.String("user", "", "user name")
	flagPassword := flag.String("password", "", "user password")
	flagReadonly := flag.Bool("r", false, "read only")

	flag.Parse()

	dir = *dirFlag

	srv := &webdav.Handler{
		FileSystem: webdav.Dir(dir),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("[WEBDAV] [%s]: %s, ERROR: %s\n", r.Method, r.URL, err)
			} else {
				log.Printf("[WEBDAV] [%s]: %s \n", r.Method, r.URL)
			}
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if *flagUserName != "" && *flagPassword != "" {
			username, password, ok := req.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if username != *flagUserName || password != *flagPassword {
				http.Error(w, "WebDAV: need authorized!", http.StatusUnauthorized)
				return
			}
		}
		if req.Method == "GET" && handleDirList(srv.FileSystem, w, req) {
			return
		}
		if *flagReadonly {
			switch req.Method {
			case "PUT", "DELETE", "PROPPATCH", "MKCOL", "COPY", "MOVE":
				http.Error(w, "WebDAV: Read Only!!!", http.StatusForbidden)
				return
			}
		}
		srv.ServeHTTP(w, req)
	})

	var err error
	if *serveSecure {
		if _, err = os.Stat(fn_cert_pem); err != nil {
			log.Fatalf("[INIT] No %v in current directory. Please provide a valid cert", fn_cert_pem)
			return
		}
		if _, err = os.Stat(fn_key_pem); err != nil {
			log.Fatalf("[INIT] No %v in current directory. Please provide a valid cert", fn_key_pem)
			return
		}

		err = http.ListenAndServeTLS(fmt.Sprintf(":%d", *httpPort), fn_cert_pem, fn_key_pem, nil)
	} else {
		err = http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil)
	}
	if err != nil {
		log.Fatalf("Error with WebDAV server: %v", err)
	}

}

func handleDirList(fs webdav.FileSystem, w http.ResponseWriter, req *http.Request) bool {
	ctx := context.Background()
	f, err := fs.OpenFile(ctx, req.URL.Path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	if fi, _ := f.Stat(); fi != nil && !fi.IsDir() {
		return false
	}
	dirs, err := f.Readdir(-1)
	if err != nil {
		log.Print(w, "Error reading directory", http.StatusInternalServerError)
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	for _, d := range dirs {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", name, name)
	}
	fmt.Fprintf(w, "</pre>\n")
	return true
}
