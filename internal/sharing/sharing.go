// Package sharing is the bridge dashboard's local sharing board.
//
// Posts live in one JSON file (0600, path overridable via ZCODE_SHARING_FILE).
// The board is not a hosted service: it is shared by whoever opens the same
// bridge, which keeps the feature useful over a tunnel without adding an
// account system. Reactions are keyed by an opaque browser client id so a
// toggle is idempotent; only the creator's client may delete its post.
package sharing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	Version     = 1
	MaxPosts    = 80
	MaxComments = 50
	MaxTitle    = 120
	MaxBody     = 4000
	MaxLink     = 600
	MaxAuthor   = 40
	MaxImage    = 200000
	MaxComment  = 1000
	maxClient   = 80
)

var (
	ErrNotFound  = errors.New("post tidak ditemukan")
	ErrForbidden = errors.New("hanya pembuat post yang bisa menghapus")
)

// Post is the persisted record. Likes is client id -> true so clients can
// toggle their own reaction without inflating the count.
type Post struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Body      string          `json:"body,omitempty"`
	Link      string          `json:"link,omitempty"`
	Image     string          `json:"image,omitempty"`
	Author    string          `json:"author"`
	Owner     string          `json:"owner"`
	CreatedAt int64           `json:"createdAt"`
	Likes     map[string]bool `json:"likes,omitempty"`
	Comments  []Comment       `json:"comments,omitempty"`
}

// Comment is one reply on a post.
type Comment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"createdAt"`
}

// View is the API-safe projection: the likes map never leaves the process.
type View struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Link      string    `json:"link,omitempty"`
	Image     string    `json:"image,omitempty"`
	Author    string    `json:"author"`
	CreatedAt int64     `json:"createdAt"`
	Likes     int       `json:"likes"`
	Liked     bool      `json:"liked"`
	Mine      bool      `json:"mine"`
	Comments  []Comment `json:"comments"`
}

// Input is the create payload after JSON decoding.
type Input struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Link   string `json:"link"`
	Image  string `json:"image"`
	Author string `json:"author"`
	Client string `json:"client"`
}

type document struct {
	Version int    `json:"version"`
	Posts   []Post `json:"posts"`
}

var mu sync.Mutex

// File returns the board store path.
func File() string {
	if f := os.Getenv("ZCODE_SHARING_FILE"); f != "" {
		return f
	}
	return filepath.Join("data", "sharing.json")
}

func load() document {
	var doc document
	raw, err := os.ReadFile(File())
	if err != nil || json.Unmarshal(raw, &doc) != nil || doc.Posts == nil {
		return document{Version: Version, Posts: []Post{}}
	}
	return doc
}

func save(doc document) error {
	if err := os.MkdirAll(filepath.Dir(File()), 0o700); err != nil {
		return err
	}
	doc.Version = Version
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(File(), raw, 0o600)
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}

func cleanAuthor(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "anon"
	}
	return truncate(v, MaxAuthor)
}

func cleanClient(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "anon"
	}
	return truncate(v, maxClient)
}

func truncate(v string, n int) string {
	if len(v) <= n {
		return v
	}
	cut := n
	for cut > 0 && (v[cut]&0xC0) == 0x80 {
		cut--
	}
	return v[:cut]
}

func normalizeLink(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if len(v) > MaxLink {
		return "", errors.New("link terlalu panjang")
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("link harus http/https")
	}
	return u.String(), nil
}

func validImage(v string) bool {
	if v == "" {
		return true
	}
	if len(v) > MaxImage {
		return false
	}
	for _, prefix := range []string{
		"data:image/png;base64,",
		"data:image/jpeg;base64,",
		"data:image/gif;base64,",
		"data:image/webp;base64,",
	} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func (p Post) view(client string) View {
	likes := make([]string, 0, len(p.Likes))
	for id := range p.Likes {
		likes = append(likes, id)
	}
	comments := p.Comments
	if comments == nil {
		comments = []Comment{}
	}
	return View{
		ID:        p.ID,
		Title:     p.Title,
		Body:      p.Body,
		Link:      p.Link,
		Image:     p.Image,
		Author:    p.Author,
		CreatedAt: p.CreatedAt,
		Likes:     len(likes),
		Liked:     p.Likes[client],
		Mine:      client != "" && p.Owner == client,
		Comments:  comments,
	}
}

// Create validates and stores one post, pruning the oldest when the board is
// full. The returned view is already scoped to the submitting client.
func Create(in Input) (View, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	author := cleanAuthor(in.Author)
	client := cleanClient(in.Client)
	if title == "" {
		return View{}, errors.New("judul wajib diisi")
	}
	if len(title) > MaxTitle {
		return View{}, errors.New("judul terlalu panjang")
	}
	if len(body) > MaxBody {
		return View{}, errors.New("isi post terlalu panjang")
	}
	link, err := normalizeLink(in.Link)
	if err != nil {
		return View{}, err
	}
	if in.Image != "" && !validImage(in.Image) {
		return View{}, errors.New("gambar tidak didukung atau terlalu besar")
	}
	if body == "" && link == "" && in.Image == "" {
		return View{}, errors.New("isi, link, atau gambar wajib ada")
	}
	p := Post{
		ID:        newID(),
		Title:     title,
		Body:      body,
		Link:      link,
		Image:     in.Image,
		Author:    author,
		Owner:     client,
		CreatedAt: time.Now().UnixMilli(),
		Likes:     map[string]bool{},
	}
	mu.Lock()
	defer mu.Unlock()
	doc := load()
	doc.Posts = append(doc.Posts, p)
	if len(doc.Posts) > MaxPosts {
		doc.Posts = doc.Posts[len(doc.Posts)-MaxPosts:]
	}
	if err := save(doc); err != nil {
		return View{}, err
	}
	return p.view(client), nil
}

// List returns posts newest-first, or most-liked-first when sort is "top".
func List(client, order string) ([]View, error) {
	mu.Lock()
	defer mu.Unlock()
	doc := load()
	sort.SliceStable(doc.Posts, func(i, j int) bool {
		if order == "top" {
			li, lj := len(doc.Posts[i].Likes), len(doc.Posts[j].Likes)
			if li != lj {
				return li > lj
			}
		}
		return doc.Posts[i].CreatedAt > doc.Posts[j].CreatedAt
	})
	out := make([]View, 0, len(doc.Posts))
	for _, p := range doc.Posts {
		out = append(out, p.view(client))
	}
	return out, nil
}

// ToggleLike flips one client's reaction on a post.
func ToggleLike(id, client string) (View, error) {
	client = cleanClient(client)
	mu.Lock()
	defer mu.Unlock()
	doc := load()
	for i := range doc.Posts {
		if doc.Posts[i].ID != id {
			continue
		}
		if doc.Posts[i].Likes == nil {
			doc.Posts[i].Likes = map[string]bool{}
		}
		if doc.Posts[i].Likes[client] {
			delete(doc.Posts[i].Likes, client)
		} else {
			doc.Posts[i].Likes[client] = true
		}
		if err := save(doc); err != nil {
			return View{}, err
		}
		return doc.Posts[i].view(client), nil
	}
	return View{}, ErrNotFound
}

// AddComment appends a reply and keeps only the newest MaxComments.
func AddComment(id, author, body string) (View, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return View{}, errors.New("komentar kosong")
	}
	if len(body) > MaxComment {
		return View{}, errors.New("komentar terlalu panjang")
	}
	c := Comment{ID: newID(), Author: cleanAuthor(author), Body: body, CreatedAt: time.Now().UnixMilli()}
	mu.Lock()
	defer mu.Unlock()
	doc := load()
	for i := range doc.Posts {
		if doc.Posts[i].ID != id {
			continue
		}
		doc.Posts[i].Comments = append(doc.Posts[i].Comments, c)
		if len(doc.Posts[i].Comments) > MaxComments {
			doc.Posts[i].Comments = doc.Posts[i].Comments[len(doc.Posts[i].Comments)-MaxComments:]
		}
		if err := save(doc); err != nil {
			return View{}, err
		}
		return doc.Posts[i].view(""), nil
	}
	return View{}, ErrNotFound
}

// Delete removes one post when client matches its owner.
func Delete(id, client string) error {
	client = cleanClient(client)
	mu.Lock()
	defer mu.Unlock()
	doc := load()
	for i := range doc.Posts {
		if doc.Posts[i].ID != id {
			continue
		}
		if doc.Posts[i].Owner != client {
			return ErrForbidden
		}
		doc.Posts = append(doc.Posts[:i], doc.Posts[i+1:]...)
		return save(doc)
	}
	return ErrNotFound
}
