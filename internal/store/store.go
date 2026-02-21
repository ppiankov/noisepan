package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Post struct {
	ID         int64
	Source     string
	Channel    string
	ExternalID string
	Text       string
	Snippet    string
	TextHash   string
	URL        string
	PostedAt   time.Time
	FetchedAt  time.Time
}

type PostInput struct {
	Source     string
	Channel    string
	ExternalID string
	Text       string
	Snippet    string
	URL        string
	PostedAt   time.Time
	FetchedAt  time.Time
}

type Score struct {
	PostID      int64
	Score       int
	Labels      []string
	Tier        string
	ScoredAt    time.Time
	Explanation json.RawMessage
}

type PostWithScore struct {
	Post  Post
	Score *Score
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path is required")
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) InsertPost(ctx context.Context, in PostInput) (Post, error) {
	if s == nil || s.db == nil {
		return Post{}, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if strings.TrimSpace(in.Source) == "" {
		return Post{}, errors.New("source is required")
	}
	if strings.TrimSpace(in.Channel) == "" {
		return Post{}, errors.New("channel is required")
	}
	if strings.TrimSpace(in.ExternalID) == "" {
		return Post{}, errors.New("external_id is required")
	}
	if in.PostedAt.IsZero() {
		return Post{}, errors.New("posted_at is required")
	}
	if in.FetchedAt.IsZero() {
		return Post{}, errors.New("fetched_at is required")
	}

	snippet := strings.TrimSpace(in.Snippet)
	if snippet == "" {
		if in.Text == "" {
			return Post{}, errors.New("snippet is required when text is empty")
		}
		snippet = firstNRunes(in.Text, 200)
	}

	hash := textHash(in.Text, snippet)

	var textVal sql.NullString
	if in.Text != "" {
		textVal = sql.NullString{String: in.Text, Valid: true}
	}

	var urlVal sql.NullString
	if strings.TrimSpace(in.URL) != "" {
		urlVal = sql.NullString{String: strings.TrimSpace(in.URL), Valid: true}
	}

	postedAt := formatTime(in.PostedAt)
	fetchedAt := formatTime(in.FetchedAt)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO posts (
			source, channel, external_id, text, snippet, text_hash, url, posted_at, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, channel, external_id) DO UPDATE SET
			text = excluded.text,
			snippet = excluded.snippet,
			text_hash = excluded.text_hash,
			url = excluded.url,
			posted_at = excluded.posted_at,
			fetched_at = excluded.fetched_at
	`,
		in.Source,
		in.Channel,
		in.ExternalID,
		textVal,
		snippet,
		hash,
		urlVal,
		postedAt,
		fetchedAt,
	)
	if err != nil {
		return Post{}, fmt.Errorf("insert post: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, source, channel, external_id, text, snippet, text_hash, url, posted_at, fetched_at
		FROM posts
		WHERE source = ? AND channel = ? AND external_id = ?
	`, in.Source, in.Channel, in.ExternalID)

	post, err := scanPost(row)
	if err != nil {
		return Post{}, err
	}

	return post, nil
}

func (s *Store) GetUnscored(ctx context.Context) ([]Post, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.source, p.channel, p.external_id, p.text, p.snippet, p.text_hash, p.url, p.posted_at, p.fetched_at
		FROM posts p
		LEFT JOIN scores s ON s.post_id = p.id
		WHERE s.post_id IS NULL
		ORDER BY p.posted_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get unscored: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var posts []Post
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unscored: %w", err)
	}

	return posts, nil
}

func (s *Store) SaveScore(ctx context.Context, in Score) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if in.PostID == 0 {
		return errors.New("post_id is required")
	}
	if in.Tier == "" {
		return errors.New("tier is required")
	}
	if in.ScoredAt.IsZero() {
		return errors.New("scored_at is required")
	}

	labels := in.Labels
	if labels == nil {
		labels = []string{}
	}
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("encode labels: %w", err)
	}

	var explanationVal sql.NullString
	if len(in.Explanation) > 0 {
		explanationVal = sql.NullString{String: string(in.Explanation), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO scores (post_id, score, labels, tier, scored_at, explanation)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(post_id) DO UPDATE SET
			score = excluded.score,
			labels = excluded.labels,
			tier = excluded.tier,
			scored_at = excluded.scored_at,
			explanation = excluded.explanation
	`,
		in.PostID,
		in.Score,
		string(labelsJSON),
		in.Tier,
		formatTime(in.ScoredAt),
		explanationVal,
	)
	if err != nil {
		return fmt.Errorf("save score: %w", err)
	}

	return nil
}

// PostFilter holds optional filters for GetPosts.
type PostFilter struct {
	Source  string // filter by source (e.g. "rss", "telegram")
	Channel string // filter by channel name
}

func (s *Store) GetPosts(ctx context.Context, since time.Time, tier string, filters ...PostFilter) ([]PostWithScore, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	sinceValue := formatTime(since)

	join := "LEFT JOIN"
	if tier != "" {
		join = "JOIN"
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.source, p.channel, p.external_id, p.text, p.snippet, p.text_hash, p.url, p.posted_at, p.fetched_at,
			s.score, s.labels, s.tier, s.scored_at, s.explanation
		FROM posts p
		%s scores s ON s.post_id = p.id
		WHERE p.posted_at >= ?`, join)
	args := []any{sinceValue}

	if tier != "" {
		query += " AND s.tier = ?"
		args = append(args, tier)
	}

	var filter PostFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if filter.Source != "" {
		query += " AND p.source = ?"
		args = append(args, filter.Source)
	}
	if filter.Channel != "" {
		query += " AND p.channel = ?"
		args = append(args, filter.Channel)
	}

	query += " ORDER BY p.posted_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get posts: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var posts []PostWithScore
	for rows.Next() {
		post, score, err := scanPostWithScore(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, PostWithScore{Post: post, Score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}

	return posts, nil
}

func (s *Store) Deduplicate(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, source, channel, text_hash, posted_at
		FROM posts
		ORDER BY text_hash, posted_at, id
	`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("query duplicates: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	type dupEntry struct {
		dupID    int64
		keeperID int64
		source   string
		channel  string
	}

	var (
		lastHash string
		keeperID int64
		toDelete []dupEntry
	)

	for rows.Next() {
		var (
			id             int64
			src, ch        string
			hash, postedAt string
		)
		if err := rows.Scan(&id, &src, &ch, &hash, &postedAt); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("scan duplicate: %w", err)
		}
		if hash == lastHash {
			toDelete = append(toDelete, dupEntry{
				dupID: id, keeperID: keeperID, source: src, channel: ch,
			})
			continue
		}
		lastHash = hash
		keeperID = id
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("iterate duplicates: %w", err)
	}

	deleted := 0
	for _, dup := range toDelete {
		_, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO post_also_in(post_id, source, channel) VALUES(?, ?, ?)",
			dup.keeperID, dup.source, dup.channel,
		)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert also_in: %w", err)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM scores WHERE post_id = ?", dup.dupID); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("delete duplicate score: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE id = ?", dup.dupID); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("delete duplicate post: %w", err)
		}
		deleted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit deduplicate: %w", err)
	}

	return deleted, nil
}

// PruneOld deletes posts older than retainDays and their associated scores.
// post_also_in rows are cascade-deleted. Returns the number of posts removed.
func (s *Store) PruneOld(ctx context.Context, retainDays int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if retainDays <= 0 {
		return 0, nil
	}

	cutoff := formatTime(time.Now().AddDate(0, 0, -retainDays))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune transaction: %w", err)
	}

	// Delete scores for old posts (no CASCADE on scores FK)
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM scores WHERE post_id IN (SELECT id FROM posts WHERE posted_at < ?)", cutoff,
	); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prune old scores: %w", err)
	}

	// Delete old posts (post_also_in cascades)
	res, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE posted_at < ?", cutoff)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prune old posts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune: %w", err)
	}

	n, _ := res.RowsAffected()
	return n, nil
}

// GetAlsoIn returns "also seen in" channels for the given post IDs.
// Returns a map of postID → ["source/channel", ...].
func (s *Store) GetAlsoIn(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(postIDs))
	args := make([]any, len(postIDs))
	for i, id := range postIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT post_id, source, channel FROM post_also_in WHERE post_id IN (%s)",
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query also_in: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]string)
	for rows.Next() {
		var postID int64
		var src, ch string
		if err := rows.Scan(&postID, &src, &ch); err != nil {
			return nil, fmt.Errorf("scan also_in: %w", err)
		}
		result[postID] = append(result[postID], src+"/"+ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate also_in: %w", err)
	}

	return result, nil
}

// ChannelStats holds aggregated scoring stats for one channel.
type ChannelStats struct {
	Source   string
	Channel  string
	Total    int
	ReadNow  int
	Skim     int
	Ignored  int
	LastSeen time.Time
}

// GetChannelStats returns per-channel scoring aggregates for posts since the given time.
func (s *Store) GetChannelStats(ctx context.Context, since time.Time) ([]ChannelStats, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.source, p.channel,
			COUNT(*) AS total,
			SUM(CASE WHEN s.tier = 'read_now' THEN 1 ELSE 0 END) AS read_now,
			SUM(CASE WHEN s.tier = 'skim' THEN 1 ELSE 0 END) AS skim,
			SUM(CASE WHEN s.tier = 'ignore' OR s.tier IS NULL THEN 1 ELSE 0 END) AS ignored,
			MAX(p.posted_at) AS last_seen
		FROM posts p
		LEFT JOIN scores s ON s.post_id = p.id
		WHERE p.posted_at >= ?
		GROUP BY p.source, p.channel
		ORDER BY p.source, p.channel
	`, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("get channel stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stats []ChannelStats
	for rows.Next() {
		var cs ChannelStats
		var lastSeen string
		if err := rows.Scan(&cs.Source, &cs.Channel, &cs.Total, &cs.ReadNow, &cs.Skim, &cs.Ignored, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan channel stats: %w", err)
		}
		cs.LastSeen, err = parseTime(lastSeen)
		if err != nil {
			return nil, fmt.Errorf("parse last_seen: %w", err)
		}
		stats = append(stats, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel stats: %w", err)
	}

	return stats, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(scanner rowScanner) (Post, error) {
	var (
		post                Post
		textVal, urlVal     sql.NullString
		postedAt, fetchedAt string
	)

	if err := scanner.Scan(
		&post.ID,
		&post.Source,
		&post.Channel,
		&post.ExternalID,
		&textVal,
		&post.Snippet,
		&post.TextHash,
		&urlVal,
		&postedAt,
		&fetchedAt,
	); err != nil {
		return Post{}, fmt.Errorf("scan post: %w", err)
	}

	if textVal.Valid {
		post.Text = textVal.String
	}
	if urlVal.Valid {
		post.URL = urlVal.String
	}

	var err error
	post.PostedAt, err = parseTime(postedAt)
	if err != nil {
		return Post{}, fmt.Errorf("parse posted_at: %w", err)
	}
	post.FetchedAt, err = parseTime(fetchedAt)
	if err != nil {
		return Post{}, fmt.Errorf("parse fetched_at: %w", err)
	}

	return post, nil
}

func scanPostWithScore(scanner rowScanner) (Post, *Score, error) {
	var (
		post                        Post
		textVal, urlVal             sql.NullString
		postedAt, fetchedAt         string
		scoreVal                    sql.NullInt64
		labelsVal, tierVal          sql.NullString
		scoredAtVal, explanationVal sql.NullString
	)

	if err := scanner.Scan(
		&post.ID,
		&post.Source,
		&post.Channel,
		&post.ExternalID,
		&textVal,
		&post.Snippet,
		&post.TextHash,
		&urlVal,
		&postedAt,
		&fetchedAt,
		&scoreVal,
		&labelsVal,
		&tierVal,
		&scoredAtVal,
		&explanationVal,
	); err != nil {
		return Post{}, nil, fmt.Errorf("scan post with score: %w", err)
	}

	if textVal.Valid {
		post.Text = textVal.String
	}
	if urlVal.Valid {
		post.URL = urlVal.String
	}

	var err error
	post.PostedAt, err = parseTime(postedAt)
	if err != nil {
		return Post{}, nil, fmt.Errorf("parse posted_at: %w", err)
	}
	post.FetchedAt, err = parseTime(fetchedAt)
	if err != nil {
		return Post{}, nil, fmt.Errorf("parse fetched_at: %w", err)
	}

	if !scoreVal.Valid {
		return post, nil, nil
	}

	labels := []string{}
	if labelsVal.Valid && labelsVal.String != "" {
		if err := json.Unmarshal([]byte(labelsVal.String), &labels); err != nil {
			return Post{}, nil, fmt.Errorf("decode labels: %w", err)
		}
	}

	if !tierVal.Valid {
		return Post{}, nil, errors.New("score tier missing")
	}

	scoredAt, err := parseTime(scoredAtVal.String)
	if err != nil {
		return Post{}, nil, fmt.Errorf("parse scored_at: %w", err)
	}

	var explanation json.RawMessage
	if explanationVal.Valid {
		explanation = json.RawMessage(explanationVal.String)
	}

	score := &Score{
		PostID:      post.ID,
		Score:       int(scoreVal.Int64),
		Labels:      labels,
		Tier:        tierVal.String,
		ScoredAt:    scoredAt,
		Explanation: explanation,
	}

	return post, score, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, value)
}

func textHash(text, snippet string) string {
	if text == "" {
		text = snippet
	}
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func firstNRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
