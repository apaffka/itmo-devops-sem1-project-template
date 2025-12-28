package prices

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

type rowParsed struct {
	ID         int64
	Name       string
	Category   string
	PriceCents int64
	PriceStr   string
	CreateDate time.Time
}

func (s *Service) ImportArchive(ctx context.Context, tempFilePath string, archType string) (ImportResult, error) {
	switch archType {
	case "zip":
		return s.importZip(ctx, tempFilePath)
	case "tar":
		return s.importTar(ctx, tempFilePath)
	default:
		return ImportResult{}, fmt.Errorf("unsupported archive type %q", archType)
	}
}

func (s *Service) importZip(ctx context.Context, tempFilePath string) (ImportResult, error) {
	zr, err := zip.OpenReader(tempFilePath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	var csvFile *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			csvFile = f
			break
		}
	}
	if csvFile == nil {
		return ImportResult{}, errors.New("zip: no .csv file found")
	}

	rc, err := csvFile.Open()
	if err != nil {
		return ImportResult{}, fmt.Errorf("zip open csv: %w", err)
	}
	defer rc.Close()

	return s.importCSV(ctx, rc)
}

func (s *Service) importTar(ctx context.Context, tempFilePath string) (ImportResult, error) {
	f, err := os.Open(tempFilePath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open tar file: %w", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	peek, _ := br.Peek(2)

	var tr *tar.Reader
	if len(peek) == 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gzr, err := gzip.NewReader(br)
		if err != nil {
			return ImportResult{}, fmt.Errorf("gzip reader: %w", err)
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(br)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ImportResult{}, fmt.Errorf("tar read: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(hdr.Name), ".csv") {
			return s.importCSV(ctx, tr)
		}
	}
	return ImportResult{}, errors.New("tar: no .csv file found")
}

func (s *Service) importCSV(ctx context.Context, r io.Reader) (ImportResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ImportResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return ImportResult{}, fmt.Errorf("read csv header: %w", err)
	}

	colIndex := map[string]int{}
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"id", "name", "category", "price", "create_date"}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return ImportResult{}, fmt.Errorf("csv missing required column %q", col)
		}
	}

		seen := map[string]struct{}{}
	var totalCount, duplicatesCount, inserted int64

	const insertSQL = `
INSERT INTO prices(id, name, category, price, create_date)
VALUES ($1, $2, $3, $4::numeric, $5)
ON CONFLICT (name, category, price, create_date) DO NOTHING;
`

	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			totalCount++
			s.logger.Warn("csv read error, skipping line", "err", err)
			continue
		}

		totalCount++

		row, ok := parseRow(rec, colIndex)
		if !ok {
			continue
		}

		key := dedupeKey(row)
		if _, exists := seen[key]; exists {
			duplicatesCount++
			continue
		}
		seen[key] = struct{}{}

		tag, err := tx.Exec(ctx, insertSQL, row.ID, row.Name, row.Category, row.PriceStr, row.CreateDate)
		if err != nil {
			return ImportResult{}, fmt.Errorf("insert: %w", err)
		}
		if tag.RowsAffected() == 0 {
			duplicatesCount++
			continue
		}
		inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		return ImportResult{}, fmt.Errorf("commit: %w", err)
	}

	var cats int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT category) FROM prices`).Scan(&cats); err != nil {
		return ImportResult{}, fmt.Errorf("count categories: %w", err)
	}

	var sumTxt string
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(price),0)::text FROM prices`).Scan(&sumTxt); err != nil {
		return ImportResult{}, fmt.Errorf("sum price: %w", err)
	}
	totalPriceAny := parseNumericText(sumTxt)

	return ImportResult{
		TotalCount:      totalCount,
		DuplicatesCount: duplicatesCount,
		TotalItems:      inserted,
		TotalCategories: cats,
		TotalPrice:      totalPriceAny,
	}, nil
}


func dedupeKey(r rowParsed) string {
	return fmt.Sprintf("%s|%s|%d|%s",
		strings.ToLower(strings.TrimSpace(r.Name)),
		strings.ToLower(strings.TrimSpace(r.Category)),
		r.PriceCents,
		r.CreateDate.Format("2006-01-02"),
	)
}

func parseRow(rec []string, idx map[string]int) (rowParsed, bool) {
	get := func(col string) string {
		i := idx[col]
		if i < 0 || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	id, _ := parseInt64(get("id"))
	name := get("name")
	category := get("category")
	priceStr := get("price")
	dateStr := get("create_date")

	if name == "" || category == "" || priceStr == "" || dateStr == "" {
		return rowParsed{}, false
	}

	cents, canon, err := parsePriceToCents(priceStr)
	if err != nil || cents <= 0 {
		return rowParsed{}, false
	}

	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return rowParsed{}, false
	}

	return rowParsed{
		ID:         id,
		Name:       name,
		Category:   category,
		PriceCents: cents,
		PriceStr:   canon,
		CreateDate: dt,
	}, true
}

func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parsePriceToCents(s string) (int64, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", errors.New("empty price")
	}
	if strings.Contains(s, ",") {
		return 0, "", errors.New("price must use '.' as decimal separator")
	}

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	if intPart == "" {
		intPart = "0"
	}
	var whole int64
	if _, err := fmt.Sscanf(intPart, "%d", &whole); err != nil {
		return 0, "", err
	}
	if whole < 0 {
		return 0, "", errors.New("negative price")
	}

	var frac int64
	if len(parts) == 2 {
		fp := parts[1]
		if len(fp) > 2 {
			return 0, "", errors.New("too many fractional digits")
		}
		if len(fp) == 1 {
			fp += "0"
		}
		if fp == "" {
			fp = "00"
		}
		if _, err := fmt.Sscanf(fp, "%d", &frac); err != nil {
			return 0, "", err
		}
	}

	if frac < 0 || frac > 99 {
		return 0, "", errors.New("bad fraction")
	}

	cents := whole*100 + frac
	canon := fmt.Sprintf("%d.%02d", whole, frac)
	return cents, canon, nil
}

func parseNumericText(s string) any {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, ".00") {
		var n int64
		_, err := fmt.Sscanf(strings.TrimSuffix(s, ".00"), "%d", &n)
		if err == nil {
			return n
		}
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err == nil {
		return f
	}
	return 0
}

func (s *Service) ExportZip(ctx context.Context, f ExportFilters) ([]byte, error) {
	q := `SELECT id, name, category, price::text, create_date FROM prices WHERE 1=1`
	args := []any{}
	n := 1

	if f.Start != nil {
		q += fmt.Sprintf(" AND create_date >= $%d", n)
		args = append(args, *f.Start)
		n++
	}
	if f.End != nil {
		q += fmt.Sprintf(" AND create_date <= $%d", n)
		args = append(args, *f.End)
		n++
	}
	if f.Min != nil {
		q += fmt.Sprintf(" AND price >= $%d::numeric", n)
		args = append(args, fmt.Sprintf("%d.00", *f.Min))
		n++
	}
	if f.Max != nil {
		q += fmt.Sprintf(" AND price <= $%d::numeric", n)
		args = append(args, fmt.Sprintf("%d.00", *f.Max))
		n++
	}
	q += " ORDER BY create_date, category, name"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query export: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	fw, err := zw.Create("data.csv")
	if err != nil {
		return nil, fmt.Errorf("zip create data.csv: %w", err)
	}

	cw := csv.NewWriter(fw)
	_ = cw.Write([]string{"id", "name", "category", "price", "create_date"})

	for rows.Next() {
		var id int64
		var name, category string
		var priceTxt string
		var createDate time.Time

		if err := rows.Scan(&id, &name, &category, &priceTxt, &createDate); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		_ = cw.Write([]string{
			fmt.Sprintf("%d", id),
			name,
			category,
			priceTxt,
			createDate.Format("2006-01-02"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return nil, fmt.Errorf("csv write: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}

	return buf.Bytes(), nil
}
