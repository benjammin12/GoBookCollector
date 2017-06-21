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
	"github.com/urfave/negroni"
)

type Page struct {
	Books []Book
}

type Book struct {
	Id int
	Title string
	Author string
	Classification string
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
	mux := http.NewServeMux()

	mux.HandleFunc("/", index)

	mux.HandleFunc("/search", displayBooks)

	mux.HandleFunc("/books/add", addBook)

	n := negroni.Classic() //use negroni as a web middleware to handle actions before your route is called, negroni logs all response and requests
				//to the console

	n.Use(negroni.HandlerFunc(verifyConnection))
	n.UseHandler(mux)

	n.Run(":8080")
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
	p := Page{Books: []Book{}}

	rows, _ := Db.Query("SELECT id, title, author, classification from books")

	for rows.Next() {
		var b Book
		rows.Scan(&b.Id, &b.Title, &b.Author, &b.Classification)
		p.Books = append(p.Books, b)
	}


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

	result , err := Db.Exec("insert into books (title, author, book_id, classification) values ($1,$2,$3,$4)",
		book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	pk, _ := result.LastInsertId()

	b := Book{
		Id:int(pk),
		Title:book.BookData.Title,
		Author:string(book.BookData.Author),
		Classification:book.Classification.MostPopular,
	}

	if errJson := json.NewEncoder(w).Encode(b); errJson != nil {
		http.Error(w, errJson.Error(),http.StatusInternalServerError)
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

func verifyConnection(w http.ResponseWriter, r *http.Request, next http.HandlerFunc){
	err := Db.Ping()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, r)
}
