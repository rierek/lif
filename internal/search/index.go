// Package search provides full-text search functionality for items using Bleve.
package search

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/rierek/lif/pkg/item"
)

// Index wraps a Bleve index for searching items. It may be a private
// in-memory index (persistent == false) or one backed by a persisted,
// locked on-disk index shared across invocations of this store
// (persistent == true); see Open.
type Index struct {
	index      bleve.Index
	persistent bool
	dir        string            // index directory; empty for in-memory indexes
	etags      map[string]string // item ID -> last-synced ETag; nil for in-memory indexes
	lock       *indexLock        // held only when persistent
	// readOnly is true exactly when this Index was obtained through
	// OpenRead (lockModeShared) AND a real persisted index was opened --
	// never for the in-memory fallback, which is always writable even
	// when OpenRead is the caller that triggered it. IndexItem, DeleteItem,
	// and Sync all hang forever on a readOnly index (see OpenRead's doc
	// comment); a caller that needs to write MUST Close this handle and
	// reopen the same dir with Open instead. See ReadOnly.
	readOnly bool
}

// ReadOnly reports whether this index was opened in shared, read-only mode
// (see OpenRead) and therefore must never be written to directly. The
// supported way to start writing is to Close this handle -- releasing the
// shared lock -- and reopen the same directory exclusively via Open.
func (idx *Index) ReadOnly() bool {
	return idx.readOnly
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

// Close closes the index and, for a persistent index, releases the
// cross-process lock acquired by Open.
func (idx *Index) Close() error {
	err := idx.index.Close()
	idx.lock.release()
	return err
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

// IndexItem adds or updates an item in the search index. For a persistent
// index this also records the item's current staleness key in the sidecar,
// so a later Sync in another process recognizes this item as already
// current.
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
	if idx.persistent {
		return saveETags(idx.dir, idx.etags)
	}
	return nil
}

// DeleteItem removes an item from the search index and, for a persistent
// index, its ETag sidecar entry.
func (idx *Index) DeleteItem(id string) error {
	if err := idx.index.Delete(id); err != nil {
		return err
	}
	delete(idx.etags, id)
	if idx.persistent {
		return saveETags(idx.dir, idx.etags)
	}
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
	// - Boolean operators: "user AND password"
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
// The diff runs the same way for a persistent and an in-memory index: only
// the persisted sidecar write at the end is conditional. An in-memory index
// starts with an empty idx.etags (see NewIndex), so its first Sync in a
// fresh process still reindexes everything; within one process, a later
// Sync on the same Index is incremental exactly like the persistent case,
// including retracting items no longer present -- swapping the underlying
// Bleve index instead would race Core.Search, which reads idx.index after
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
		if err := idx.index.Batch(batch); err != nil {
			return err
		}
	}
	if idx.persistent {
		return saveETags(idx.dir, idx.etags)
	}
	return nil
}

// diffAgainstETags compares items against etags (the ID -> staleKey map an
// Index carries) and reports which items are new or changed and which
// previously-seen IDs are no longer present. Sync and NeedsSync both go
// through this single comparison so they cannot drift: Sync acts on the
// result, NeedsSync only asks whether it is non-empty.
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

// NeedsSync reports whether calling Sync(items) on this index would change
// anything: an item added, removed, or edited (its ETag or Path changed)
// since this index's etags were last recorded. It runs the exact same
// comparison Sync itself uses (diffAgainstETags) without mutating the
// index or its etags, so a caller can safely ask this of a read-only
// index -- unlike Sync, which hangs forever on one (see OpenRead's doc
// comment) -- to decide whether it is actually behind before upgrading to
// a writable handle just to sync it.
func (idx *Index) NeedsSync(items []*item.Item) bool {
	changed, removed := diffAgainstETags(idx.etags, items)
	return len(changed) > 0 || len(removed) > 0
}
