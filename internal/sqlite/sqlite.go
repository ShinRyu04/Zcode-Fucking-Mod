// Package sqlite reads SQLite database files without a driver or cgo.
//
// It is deliberately small: it understands the file header, the WAL, table
// b-trees and record serialisation, which is everything needed to read the
// ZCode CLI usage database while the desktop application keeps writing to it.
// It never writes, never takes a lock, and never opens the file with O_RDWR.
package sqlite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Kind is the storage class of a record value.
type Kind uint8

const (
	KindNull Kind = iota
	KindInt
	KindFloat
	KindText
	KindBlob
)

// Value is one column of one row.
type Value struct {
	Kind  Kind
	Int   int64
	Float float64
	Text  string
	Blob  []byte
}

// String renders the value the way SQLite would print it.
func (v Value) String() string {
	switch v.Kind {
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case KindText:
		return v.Text
	case KindBlob:
		return string(v.Blob)
	default:
		return ""
	}
}

// Int64 returns the value as an integer when it is stored as one.
func (v Value) Int64() (int64, bool) {
	if v.Kind == KindInt {
		return v.Int, true
	}
	return 0, false
}

func (v Value) IsNull() bool { return v.Kind == KindNull }

// Column is one declared column of a table.
type Column struct {
	Name string
	Type string
}

// Table is a rowid table inside an open database.
type Table struct {
	Name         string
	Root         uint32
	SQL          string
	Columns      []Column
	WithoutRowID bool

	db *DB
}

// DB is a read-only SQLite database view.
type DB struct {
	path     string
	file     *os.File
	pageSize int
	usable   int
	encoding int

	mu       sync.Mutex
	wal      map[uint32][]byte
	cache    map[uint32][]byte
	cacheKey []uint32

	schema map[string]*Table
}

const (
	pageInteriorIndex = 2
	pageInteriorTable = 5
	pageLeafIndex     = 10
	pageLeafTable     = 13

	maxCachedPages = 4096
)

var (
	// ErrNoDatabase means the file does not start with the SQLite magic.
	ErrNoDatabase = errors.New("sqlite: not a database")
	// ErrWithoutRowID means the table has no rowid b-tree to scan.
	ErrWithoutRowID = errors.New("sqlite: WITHOUT ROWID tables are not supported")
)

// Open opens path read-only and loads its schema.
func Open(path string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	hdr := make([]byte, 100)
	if _, err := io.ReadFull(f, hdr); err != nil {
		f.Close()
		return nil, fmt.Errorf("sqlite: read header: %w", err)
	}
	if string(hdr[:16]) != "SQLite format 3\x00" {
		f.Close()
		return nil, ErrNoDatabase
	}
	pageSize := int(binary.BigEndian.Uint16(hdr[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize&(pageSize-1) != 0 {
		f.Close()
		return nil, fmt.Errorf("sqlite: invalid page size %d", pageSize)
	}
	db := &DB{
		path:     path,
		file:     f,
		pageSize: pageSize,
		usable:   pageSize - int(hdr[20]),
		encoding: int(binary.BigEndian.Uint32(hdr[56:60])),
		wal:      map[uint32][]byte{},
		cache:    map[uint32][]byte{},
		schema:   map[string]*Table{},
	}
	if db.usable < 480 {
		f.Close()
		return nil, fmt.Errorf("sqlite: unusable page geometry (usable %d)", db.usable)
	}
	db.loadWAL(path + "-wal")
	if err := db.loadSchema(); err != nil {
		f.Close()
		return nil, err
	}
	return db, nil
}

// Close releases the file handle.
func (db *DB) Close() error {
	db.mu.Lock()
	db.cache = nil
	db.cacheKey = nil
	db.mu.Unlock()
	return db.file.Close()
}

// Path returns the database path this view was opened from.
func (db *DB) Path() string { return db.path }

// Table returns the named table, or nil when it does not exist.
func (db *DB) Table(name string) *Table {
	return db.schema[strings.ToLower(name)]
}

// Tables lists every rowid table in the database.
func (db *DB) Tables() []*Table {
	out := make([]*Table, 0, len(db.schema))
	for _, t := range db.schema {
		out = append(out, t)
	}
	return out
}

// ColumnIndex returns the index of name, or -1.
func (t *Table) ColumnIndex(name string) int {
	for i, c := range t.Columns {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

// Scan walks every row of the table. Iteration stops at the first error
// returned by fn. Rows are returned in b-tree order, not sorted.
func (t *Table) Scan(fn func(rowid int64, values []Value) error) error {
	if t.WithoutRowID {
		return ErrWithoutRowID
	}
	return t.db.scanTable(t.Root, func(rowid int64, payload []byte) error {
		values, err := decodeRecord(payload)
		if err != nil {
			return err
		}
		return fn(rowid, values)
	})
}

// page returns one page image, preferring the committed WAL overlay.
func (db *DB) page(n uint32) ([]byte, error) {
	db.mu.Lock()
	if p, ok := db.wal[n]; ok {
		db.mu.Unlock()
		return p, nil
	}
	if p, ok := db.cache[n]; ok {
		db.mu.Unlock()
		return p, nil
	}
	db.mu.Unlock()

	buf := make([]byte, db.pageSize)
	if _, err := db.file.ReadAt(buf, int64(n-1)*int64(db.pageSize)); err != nil {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return nil, fmt.Errorf("sqlite: page %d is past the end of the file", n)
		}
		return nil, fmt.Errorf("sqlite: read page %d: %w", n, err)
	}

	db.mu.Lock()
	if len(db.cacheKey) >= maxCachedPages {
		evict := db.cacheKey[0]
		db.cacheKey = db.cacheKey[1:]
		delete(db.cache, evict)
	}
	db.cache[n] = buf
	db.cacheKey = append(db.cacheKey, n)
	db.mu.Unlock()
	return buf, nil
}

func (db *DB) scanTable(root uint32, fn func(rowid int64, payload []byte) error) error {
	return db.walk(root, func(n uint32, page []byte, hdrOff int) error {
		cellType := page[hdrOff]
		switch cellType {
		case pageLeafTable:
			count := int(binary.BigEndian.Uint16(page[hdrOff+3 : hdrOff+5]))
			ptrOff := hdrOff + 8
			for i := 0; i < count; i++ {
				cell := int(binary.BigEndian.Uint16(page[ptrOff+2*i:]))
				if cell < 0 || cell >= len(page) {
					return fmt.Errorf("sqlite: cell %d of page %d is out of range", i, n)
				}
				payloadLen, size := varint(page[cell:])
				if size == 0 {
					return fmt.Errorf("sqlite: bad payload length on page %d", n)
				}
				rowid, rsize := varint(page[cell+size:])
				if rsize == 0 {
					return fmt.Errorf("sqlite: bad rowid on page %d", n)
				}
				body := cell + size + rsize
				payload, err := db.payload(n, page, body, payloadLen)
				if err != nil {
					return err
				}
				if err := fn(rowid, payload); err != nil {
					return err
				}
			}
			return nil
		case pageInteriorTable:
			count := int(binary.BigEndian.Uint16(page[hdrOff+3 : hdrOff+5]))
			ptrOff := hdrOff + 12
			for i := 0; i < count; i++ {
				cell := int(binary.BigEndian.Uint16(page[ptrOff+2*i:]))
				if cell+4 > len(page) {
					return fmt.Errorf("sqlite: interior cell %d of page %d is out of range", i, n)
				}
				child := binary.BigEndian.Uint32(page[cell:])
				if err := db.scanTable(child, fn); err != nil {
					return err
				}
			}
			right := binary.BigEndian.Uint32(page[hdrOff+8 : hdrOff+12])
			if right != 0 {
				return db.scanTable(right, fn)
			}
			return nil
		default:
			return fmt.Errorf("sqlite: page %d is not a table b-tree page (type %d)", n, cellType)
		}
	})
}

// walk calls fn for one b-tree page after loading it.
func (db *DB) walk(n uint32, fn func(n uint32, page []byte, hdrOff int) error) error {
	page, err := db.page(n)
	if err != nil {
		return err
	}
	hdrOff := 0
	if n == 1 {
		hdrOff = 100
	}
	if hdrOff >= len(page) {
		return fmt.Errorf("sqlite: page %d is too small", n)
	}
	if page[hdrOff] != pageLeafTable && page[hdrOff] != pageInteriorTable {
		return fmt.Errorf("sqlite: page %d is not a table b-tree page", n)
	}
	return fn(n, page, hdrOff)
}

// payload assembles a record payload, following the overflow chain.
func (db *DB) payload(pageNo uint32, page []byte, start int, size int64) ([]byte, error) {
	if size < 0 || size > 1<<30 {
		return nil, fmt.Errorf("sqlite: implausible payload size %d", size)
	}
	usable := db.usable
	maxLocal := usable - 35
	minLocal := (usable-12)*32/255 - 23
	local := int(size)
	if size > int64(maxLocal) {
		local = minLocal + int(int(size-int64(minLocal))%(usable-4))
		if local > maxLocal {
			local = minLocal
		}
	}
	if local > usable-4 || start+local > len(page) {
		return nil, fmt.Errorf("sqlite: payload overruns page %d", pageNo)
	}
	out := make([]byte, 0, size)
	out = append(out, page[start:start+local]...)
	remaining := int(size) - local
	next := uint32(0)
	if remaining > 0 {
		nextOff := start + local
		if nextOff+4 > len(page) {
			return nil, fmt.Errorf("sqlite: overflow pointer overruns page %d", pageNo)
		}
		next = binary.BigEndian.Uint32(page[nextOff:])
	}
	for remaining > 0 {
		if next == 0 {
			return nil, fmt.Errorf("sqlite: truncated overflow chain at page %d", pageNo)
		}
		ov, err := db.page(next)
		if err != nil {
			return nil, err
		}
		var take int
		if remaining < usable-4 {
			take = remaining
		} else {
			take = usable - 4
		}
		if 4+take > len(ov) {
			return nil, fmt.Errorf("sqlite: short overflow page %d", next)
		}
		out = append(out, ov[4:4+take]...)
		remaining -= take
		next = binary.BigEndian.Uint32(ov[:4])
	}
	return out, nil
}

// varint decodes a SQLite variable-length integer.
func varint(buf []byte) (int64, int) {
	var v uint64
	for i := 0; i < 9; i++ {
		if i >= len(buf) {
			return 0, 0
		}
		b := buf[i]
		if i == 8 {
			v = (v << 8) | uint64(b)
			return int64(v), 9
		}
		v = (v << 7) | uint64(b&0x7f)
		if b&0x80 == 0 {
			return int64(v), i + 1
		}
	}
	return 0, 0
}

// decodeRecord turns a record payload into values.
func decodeRecord(payload []byte) ([]Value, error) {
	headerLen, n := varint(payload)
	if n == 0 || headerLen < 0 || int(headerLen) > len(payload) {
		return nil, errors.New("sqlite: corrupt record header")
	}
	types := make([]int64, 0, 8)
	for off := n; off < int(headerLen); {
		st, sn := varint(payload[off:])
		if sn == 0 {
			return nil, errors.New("sqlite: corrupt serial type")
		}
		types = append(types, st)
		off += sn
	}
	values := make([]Value, len(types))
	body := int(headerLen)
	for i, st := range types {
		v, size, err := decodeSerial(payload, body, st)
		if err != nil {
			return nil, err
		}
		values[i] = v
		body += size
	}
	return values, nil
}

func decodeSerial(payload []byte, off int, st int64) (Value, int, error) {
	need := func(n int) error {
		if off+n > len(payload) {
			return errors.New("sqlite: record body overruns payload")
		}
		return nil
	}
	switch {
	case st == 0:
		return Value{Kind: KindNull}, 0, nil
	case st >= 1 && st <= 6:
		sizes := [...]int{0, 1, 2, 3, 4, 6, 8}
		n := sizes[st]
		if err := need(n); err != nil {
			return Value{}, 0, err
		}
		var v int64
		for i := 0; i < n; i++ {
			v = v<<8 | int64(payload[off+i])
		}
		shift := uint(64 - 8*n)
		v = v << shift >> shift // sign extend
		return Value{Kind: KindInt, Int: v}, n, nil
	case st == 7:
		if err := need(8); err != nil {
			return Value{}, 0, err
		}
		bits := binary.BigEndian.Uint64(payload[off:])
		return Value{Kind: KindFloat, Float: math.Float64frombits(bits)}, 8, nil
	case st == 8:
		return Value{Kind: KindInt, Int: 0}, 0, nil
	case st == 9:
		return Value{Kind: KindInt, Int: 1}, 0, nil
	case st == 10 || st == 11:
		return Value{}, 0, errors.New("sqlite: reserved serial type in record")
	case st >= 12:
		var n int
		kind := KindBlob
		if st%2 == 0 {
			n = int((st - 12) / 2)
		} else {
			n = int((st - 13) / 2)
			kind = KindText
		}
		if err := need(n); err != nil {
			return Value{}, 0, err
		}
		if kind == KindText {
			return Value{Kind: KindText, Text: string(payload[off : off+n])}, n, nil
		}
		blob := make([]byte, n)
		copy(blob, payload[off:off+n])
		return Value{Kind: KindBlob, Blob: blob}, n, nil
	default:
		return Value{}, 0, fmt.Errorf("sqlite: unknown serial type %d", st)
	}
}

// loadSchema reads sqlite_master (page 1) into db.schema.
func (db *DB) loadSchema() error {
	type entry struct {
		typ, name, sql string
		root           uint32
	}
	var entries []entry
	err := db.scanTable(1, func(_ int64, payload []byte) error {
		vals, err := decodeRecord(payload)
		if err != nil {
			return err
		}
		if len(vals) < 5 {
			return nil
		}
		e := entry{
			typ:  vals[0].String(),
			name: vals[1].String(),
			sql:  vals[4].String(),
		}
		if r, ok := vals[3].Int64(); ok {
			e.root = uint32(r)
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return fmt.Errorf("sqlite: read schema: %w", err)
	}
	for _, e := range entries {
		if e.typ != "table" || e.name == "" || strings.HasPrefix(e.name, "sqlite_") {
			continue
		}
		cols, withoutRowID := parseColumns(e.sql)
		db.schema[strings.ToLower(e.name)] = &Table{
			Name:         e.name,
			Root:         e.root,
			SQL:          e.sql,
			Columns:      cols,
			WithoutRowID: withoutRowID,
			db:           db,
		}
	}
	return nil
}

// parseColumns extracts declared column names from a CREATE TABLE statement.
// Table constraints are skipped; quoted identifiers are unquoted.
func parseColumns(sql string) ([]Column, bool) {
	open := strings.IndexByte(sql, '(')
	if open < 0 {
		return nil, false
	}
	withoutRowID := strings.Contains(strings.ToUpper(sql), "WITHOUT ROWID")
	var cols []Column
	depth := 0
	start := open + 1
	flush := func(part string) {
		part = strings.TrimSpace(part)
		if part == "" {
			return
		}
		upper := strings.ToUpper(part)
		for _, kw := range []string{"PRIMARY ", "UNIQUE ", "CHECK ", "FOREIGN ", "CONSTRAINT "} {
			if strings.HasPrefix(upper, kw) {
				return
			}
		}
		name, rest := leadingIdentifier(part)
		if name == "" {
			return
		}
		typ := ""
		if f := strings.Fields(rest); len(f) > 0 && !strings.HasPrefix(f[0], "(") {
			typ = strings.ToUpper(f[0])
		}
		cols = append(cols, Column{Name: name, Type: strings.ToUpper(typ)})
	}
	var quote byte
	for i := open; i < len(sql); i++ {
		c := sql[i]
		switch {
		case quote != 0:
			if quote == ']' {
				if c == ']' {
					quote = 0
				}
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '[':
			quote = ']'
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				flush(sql[start:i])
				return cols, withoutRowID
			}
		case c == ',' && depth == 1:
			flush(sql[start:i])
			start = i + 1
		}
	}
	flush(sql[start:])
	return cols, withoutRowID
}

func leadingIdentifier(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	switch s[0] {
	case '"', '`':
		q := s[0]
		for i := 1; i < len(s); i++ {
			if s[i] == q {
				return strings.ReplaceAll(s[1:i], string(q)+string(q), string(q)), s[i+1:]
			}
		}
		return s[1:], ""
	case '[':
		if i := strings.IndexByte(s, ']'); i > 0 {
			return s[1:i], s[i+1:]
		}
		return s[1:], ""
	default:
		i := 0
		for i < len(s) && (isIdentByte(s[i])) {
			i++
		}
		return s[:i], s[i:]
	}
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// loadWAL applies committed frames from the sibling -wal file.
func (db *DB) loadWAL(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	hdr := make([]byte, 32)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return
	}
	magic := binary.BigEndian.Uint32(hdr[0:4])
	if magic != 0x377f0682 && magic != 0x377f0683 {
		return
	}
	walPageSize := int(binary.BigEndian.Uint32(hdr[8:12]))
	if walPageSize != db.pageSize {
		return
	}
	salt1 := binary.BigEndian.Uint32(hdr[16:20])
	salt2 := binary.BigEndian.Uint32(hdr[20:24])
	stored0 := binary.BigEndian.Uint32(hdr[24:28])
	stored1 := binary.BigEndian.Uint32(hdr[28:32])
	// The magic number documents the checksum byte order, but builds exist in
	// both orders; the header checksum is the authoritative test.
	bigEndian := magic == 0x377f0682
	if c0, c1, ok := walChecksum(bigEndian, hdr[:24], 0, 0, 0, 0); !ok || c0 != stored0 || c1 != stored1 {
		alt := !bigEndian
		if c0, c1, ok := walChecksum(alt, hdr[:24], 0, 0, 0, 0); ok && c0 == stored0 && c1 == stored1 {
			bigEndian = alt
		} else {
			return
		}
	}
	s0, s1 := stored0, stored1
	frameSize := 24 + walPageSize
	frame := make([]byte, frameSize)
	var frames []struct {
		page uint32
		data []byte
	}
	committed := 0
	for {
		if _, err := io.ReadFull(f, frame); err != nil {
			break
		}
		pageNo := binary.BigEndian.Uint32(frame[0:4])
		dbSize := binary.BigEndian.Uint32(frame[4:8])
		if binary.BigEndian.Uint32(frame[8:12]) != salt1 || binary.BigEndian.Uint32(frame[12:16]) != salt2 {
			break
		}
		want0 := binary.BigEndian.Uint32(frame[16:20])
		want1 := binary.BigEndian.Uint32(frame[20:24])
		c0, c1, ok := walChecksum(bigEndian, frame[0:8], s0, s1, 0, 0)
		if !ok {
			break
		}
		c0, c1, ok = walChecksum(bigEndian, frame[24:], c0, c1, 0, 0)
		if !ok || c0 != want0 || c1 != want1 {
			break
		}
		s0, s1 = c0, c1
		if pageNo == 0 {
			break
		}
		data := make([]byte, walPageSize)
		copy(data, frame[24:])
		frames = append(frames, struct {
			page uint32
			data []byte
		}{pageNo, data})
		if dbSize != 0 {
			committed = len(frames)
		}
	}
	for _, fr := range frames[:committed] {
		db.wal[fr.page] = fr.data
	}
}

// walChecksum implements the SQLite WAL checksum over 8-byte words.
func walChecksum(bigEndian bool, buf []byte, s0, s1 uint32, _, _ int) (uint32, uint32, bool) {
	if len(buf)%8 != 0 {
		return 0, 0, false
	}
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	for i := 0; i < len(buf); i += 8 {
		x0 := order.Uint32(buf[i : i+4])
		x1 := order.Uint32(buf[i+4 : i+8])
		s0 += x0 + s1
		s1 += x1 + s0
	}
	return s0, s1, true
}
