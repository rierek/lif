// Package search provides full-text search functionality for items using Bleve.
package search

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/rierek/lif/pkg/item"
)

// Index wraps a private in-memory Bleve index for searching items. Every
// process builds its own index; nothing is persisted.
type Index struct {
	index bleve.Index
	etags map[string]string // item ID -> staleKey of the last indexed version
}

// itemDocument is the structure stored in the Bleve index.
type itemDocument struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// NewIndex creates a new in-memory Bleve index.
func NewIndex() (*Index, error) {
	indexMapping := buildIndexMapping()
	idx, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, err
	}

	return &Index{index: idx, etags: map[string]string{}}, nil
}

// buildIndexMapping creates the Bleve index mapping for item documents.
func buildIndexMapping() mapping.IndexMapping {
	// Create a text field mapping with the standard analyzer
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "standard"

	// Create a keyword field mapping for ID (stored but not analyzed)
	keywordFieldMapping := bleve.NewKeywordFieldMapping()

	// Create the document mapping
	itemMapping := bleve.NewDocumentMapping()
	itemMapping.AddFieldMappingsAt("id", keywordFieldMapping)
	itemMapping.AddFieldMappingsAt("slug", textFieldMapping)
	itemMapping.AddFieldMappingsAt("title", textFieldMapping)
	itemMapping.AddFieldMappingsAt("body", textFieldMapping)

	// Create the index mapping with BM25 scoring for better relevance ranking
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = itemMapping
	indexMapping.DefaultAnalyzer = "standard"
	indexMapping.IndexDynamic = false
	indexMapping.StoreDynamic = false

	// Use BM25 scoring algorithm (available in Bleve v2.5.0+)
	// BM25 provides better relevance ranking than TF-IDF, especially for:
	// - Handling term frequency saturation (repeated terms don't over-boost)
	// - Normalizing for document length (short docs aren't unfairly penalized)
	indexMapping.ScoringModel = "bm25"

	return indexMapping
}

// Close closes the index.
func (idx *Index) Close() error {
	return idx.index.Close()
}

// staleKey is the freshness signal Sync/IndexItem record and compare: the
// item's ETag alone is blind to a rename or move, because Slug/Path are
// excluded from the rendered front matter (`yaml:"-"`) and therefore never
// affect ETag. Folding Path into the key means a `lif rename` or a
// `git checkout` that only changes the filename is still detected as a
// change worth reindexing, even though the file's bytes (and thus the ETag
// component) did not move.
func staleKey(b *item.Item) string {
	return b.ETag() + "\x1f" + b.Path
}

// IndexItem adds or updates an item in the search index and records its
// current staleness key, so a later Sync sees the item as already current.
func (idx *Index) IndexItem(b *item.Item) error {
	doc := itemDocument{
		ID:    b.ID,
		Slug:  b.Slug,
		Title: b.Title,
		Body:  b.Body,
	}
	if err := idx.index.Index(b.ID, doc); err != nil {
		return err
	}
	idx.etags[b.ID] = staleKey(b)
	return nil
}

// DeleteItem removes an item from the search index and its staleness key.
func (idx *Index) DeleteItem(id string) error {
	if err := idx.index.Delete(id); err != nil {
		return err
	}
	delete(idx.etags, id)
	return nil
}

// DefaultSearchLimit is the default maximum number of search results.
const DefaultSearchLimit = 1000

// Search executes a search query and returns matching item IDs.
// The limit parameter controls the maximum number of results (0 uses DefaultSearchLimit).
func (idx *Index) Search(queryStr string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = DefaultSearchLimit
	}

	// Use query string syntax which supports:
	// - Simple terms: "authentication"
	// - Required/excluded terms: "+user -password" (no AND/OR/NOT, no grouping)
	// - Wildcards: "auth*"
	// - Phrases: "\"user login\""
	// - Field-specific: "title:login"
	query := bleve.NewQueryStringQuery(queryStr)

	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"id"} // Only return ID field

	result, err := idx.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ids = append(ids, hit.ID)
	}

	return ids, nil
}

// Sync brings the index up to date with the given items, indexing only what
// changed since the last Sync (AC-02): an item whose staleness key (ETag plus
// Path, see staleKey) differs from the recorded value -- new, edited by this
// process, or edited on disk by another process or an editor since this
// index was last synced -- is reindexed; an item no longer present is
// removed. ETag, not mtime, is the staleness signal (AC-03); Path is folded
// in because a rename alone never changes ETag (AC-02).
//
// The index starts with an empty idx.etags (see NewIndex), so its first Sync
// reindexes everything; a later Sync on the same Index is incremental and
// also retracts items no longer present -- swapping the underlying Bleve
// index instead would race Core.Search, which reads idx.index after
// releasing its lock.
func (idx *Index) Sync(items []*item.Item) error {
	changed, removed := diffAgainstETags(idx.etags, items)
	if len(changed) == 0 && len(removed) == 0 {
		return nil
	}

	batch := idx.index.NewBatch()
	for _, b := range changed {
		doc := itemDocument{ID: b.ID, Slug: b.Slug, Title: b.Title, Body: b.Body}
		if err := batch.Index(b.ID, doc); err != nil {
			return err
		}
		idx.etags[b.ID] = staleKey(b)
	}
	for _, id := range removed {
		batch.Delete(id)
		delete(idx.etags, id)
	}
	if batch.Size() > 0 {
		return idx.index.Batch(batch)
	}
	return nil
}

// diffAgainstETags compares items against etags (the ID -> staleKey map an
// Index carries) and reports which items are new or changed and which
// previously-seen IDs are no longer present.
func diffAgainstETags(etags map[string]string, items []*item.Item) (changed []*item.Item, removed []string) {
	seen := make(map[string]struct{}, len(items))
	for _, b := range items {
		seen[b.ID] = struct{}{}
		if etags[b.ID] != staleKey(b) {
			changed = append(changed, b)
		}
	}
	for id := range etags {
		if _, ok := seen[id]; !ok {
			removed = append(removed, id)
		}
	}
	return changed, removed
}
