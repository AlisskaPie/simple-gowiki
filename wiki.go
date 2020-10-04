package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/lib/pq"
	"github.com/subosito/gotenv"
	"google.golang.org/api/iterator"
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
		title, err := getTitle(w, r)
		if err != nil {
			fmt.Errorf("getTitle: %v", err)
		}
		fn(w, r, title)
	}
}
func (p *Page) save() error {
	row := db.QueryRow("insert into Page (title, body) values ($1, $2)", p.Title, p.Body)
	err := row.Err()
	if err != nil {
		return fmt.Errorf("QueryRow: %v", err)
	}
	return nil
}

func loadPage(title string) (*Page, error) {
	var page Page
	var pages []Page
	rows, err := db.Query("select * from Page where title = $1", title)
	if err != nil {
		return nil, fmt.Errorf("Query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&page.Title, &page.Body)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("rows.Scan: %v", err)
		}
		pages = append(pages, page)
	}
	if pages == nil {
		return nil, sql.ErrNoRows
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
