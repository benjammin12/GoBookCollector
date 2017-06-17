package main

import (
	"fmt"
	"net/http"
	"html/template"
	"database/sql"
	_"github.com/lib/pq"
	"encoding/json"
	"net/url"
	"io/ioutil"
	"encoding/xml"
)

type Page struct {
	Name string
	DBStatus bool
}

type SearchResult struct {
	Title string `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year string `xml:"hyr,attr"`
	ID string `xml:"owi,attr"`
}

const (
	host     = "localhost"
	port     = 5432
	user     = "benjaminxerri"
	password = "root"
	dbname   = "books"
)

var Db *sql.DB
var templates *template.Template

func init() {
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	Db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	templates = template.Must(template.ParseFiles("templates/index.html"))

}

func main() {

	http.HandleFunc("/", index)

	http.HandleFunc("/search", displayBooks)

	http.HandleFunc("/books/add", addBook)

	fmt.Println(http.ListenAndServe(":8080", nil))
}

type ClassifySearchResponse struct {
	Results []SearchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title string `xml:"title,attr"`
		Author string `xml:"author,attr"`
		ID string `xml:"owi,attr"`
	} `xml:"work"`
	Classification struct {
		MostPopular string `xml:"sfa,attr"`
	} `xml:"recommendations>ddc>mostPopular"`
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&owi=" + url.QueryEscape(id))

	if err != nil {
		return ClassifyBookResponse{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c, err
}

func index(w http.ResponseWriter, r *http.Request) {
	p := Page{Name: "Gopher"}
	if name := r.FormValue("name"); name != "" {
		p.Name = name
	}
	p.DBStatus = Db.Ping() == nil

	if err := templates.ExecuteTemplate(w, "index.html", p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func addBook (w http.ResponseWriter, r *http.Request) {
	var book ClassifyBookResponse
	var err error

	if book, err = find(r.FormValue("id")); err !=   nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if err = Db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	_, err = Db.Exec("insert into book (title, author, book_id, classification) values ($1,$2,$3,$4)",
		book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func search(query string) ([]SearchResult, error) {
	var c ClassifySearchResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&title=" + url.QueryEscape(query))

	if err != nil {
		return []SearchResult{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c.Results, err
}

func displayBooks(w http.ResponseWriter, r *http.Request) {
	var results []SearchResult
	var err error

	if results, err = search(r.FormValue("search")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
