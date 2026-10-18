package salmon

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

//go:embed knowledge/corpus.json
var reviewedCorpus []byte

const RetrievalModel = "tfidf-cosine-v1"

type corpus struct {
	Sources []struct {
		ID        string `json:"source_id"`
		Title     string `json:"title"`
		Publisher string `json:"publisher"`
		URL       string `json:"url"`
		Chunks    []struct {
			ID   string `json:"chunk_id"`
			Text string `json:"text"`
		} `json:"chunks"`
	} `json:"sources"`
}

// Retriever is a local sparse-vector index. It does not claim dense semantic embeddings.
type Retriever struct {
	chunks  []Chunk
	vectors []map[string]float64
	idf     map[string]float64
}

var word = regexp.MustCompile(`[a-z0-9]+`)
var stopWords = map[string]bool{"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "by": true, "for": true, "from": true, "in": true, "is": true, "it": true, "of": true, "on": true, "or": true, "the": true, "to": true, "with": true}

func terms(text string) map[string]float64 {
	out := map[string]float64{}
	for _, t := range word.FindAllString(strings.ToLower(text), -1) {
		if !stopWords[t] {
			out[t]++
		}
	}
	return out
}
func NewRetriever() (*Retriever, error) { return newRetriever(reviewedCorpus) }
func newRetriever(data []byte) (*Retriever, error) {
	var c corpus
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	r := &Retriever{idf: map[string]float64{}}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		for _, c := range s.Chunks {
			if c.ID == "" || seen[c.ID] || strings.TrimSpace(c.Text) == "" {
				return nil, fmt.Errorf("invalid corpus chunk %q", c.ID)
			}
			seen[c.ID] = true
			r.chunks = append(r.chunks, Chunk{ID: c.ID, SourceID: s.ID, Title: s.Title, Publisher: s.Publisher, URL: s.URL, Text: c.Text, SHA256: digest([]byte(c.Text))})
			v := terms(c.Text)
			r.vectors = append(r.vectors, v)
			for t := range v {
				r.idf[t]++
			}
		}
	}
	if len(r.chunks) == 0 {
		return nil, fmt.Errorf("empty reviewed corpus")
	}
	for t, df := range r.idf {
		r.idf[t] = math.Log((1+float64(len(r.chunks)))/(1+df)) + 1
	}
	for _, v := range r.vectors {
		r.weight(v)
	}
	return r, nil
}
func (r *Retriever) weight(v map[string]float64) {
	for t, n := range v {
		v[t] = (1 + math.Log(n)) * r.idf[t]
	}
}
func cosine(a, b map[string]float64) float64 {
	var dot, na, nb float64
	for t, v := range a {
		dot += v * b[t]
		na += v * v
	}
	for _, v := range b {
		nb += v * v
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}
func (r *Retriever) Search(query string, k int) ([]Chunk, error) {
	if strings.TrimSpace(query) == "" || len(query) > 16000 || k < 1 || k > 5 {
		return nil, fmt.Errorf("nonempty query (max 16000 bytes) and top_k between 1 and 5 required")
	}
	v := terms(query)
	r.weight(v)
	ranked := make([]Chunk, 0, len(r.chunks))
	for i, c := range r.chunks {
		c.Score = math.Round(cosine(v, r.vectors[i])*1e6) / 1e6
		if c.Score > 0 {
			ranked = append(ranked, c)
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].ID < ranked[j].ID
		}
		return ranked[i].Score > ranked[j].Score
	})
	if len(ranked) > k {
		ranked = ranked[:k]
	}
	return ranked, nil
}

type Source struct {
	ID        string `json:"source_id"`
	Title     string `json:"title"`
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
	SHA256    string `json:"content_sha256"`
}

func (r *Retriever) GetSource(id string) (Source, error) {
	var hashes []string
	var source Source
	for _, c := range r.chunks {
		if c.SourceID == id {
			hashes = append(hashes, c.SHA256)
			source = Source{ID: id, Title: c.Title, Publisher: c.Publisher, URL: c.URL}
		}
	}
	if len(hashes) == 0 {
		return source, fmt.Errorf("unknown source_id: %s", id)
	}
	sort.Strings(hashes)
	source.SHA256 = digest([]byte(strings.Join(hashes, "")))
	return source, nil
}
