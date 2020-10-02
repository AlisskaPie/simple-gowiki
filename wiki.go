package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/lib/pq"
	"github.com/subosito/gotenv"
)

type Page struct {
	Title string
	Body  []byte
}

var (
	errMultiple = errors.New("multiple response error")
	db          *sql.DB
	templates   = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))
	validPath   = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
)

func init() {
	if err := gotenv.Load(); err != nil {
		log.Fatalf("gotenv.Load: %v", err)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}
func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}
func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}
func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	var page Page
	var pages = []Page{}
	query := fmt.Sprintf("select * from Page where title = '%v'", title)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("Query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&page.Title, &page.Body)
		if err != nil {
			return nil, fmt.Errorf("rows.Scan: %v", err)
		}
		pages = append(pages, page)
	}
	if len(pages) > 1 {
		return nil, errMultiple
	}
	return &page, nil
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	pgURL, err := pq.ParseURL(os.Getenv("ELEPHANTSQL_URL"))
	if err != nil {
		log.Fatalf("pq.ParseURL: %v", err)
	}
	db, err = sql.Open("postgres", pgURL)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("db.Ping: %v", err)
	}
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
