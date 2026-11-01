package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/battleServer"
)

// -------------------------------------------------------------------
// boolInt
// -------------------------------------------------------------------

func TestNumberToBool(t *testing.T) {
	if NumberToBool(0) {
		t.Fatal("0 should be false")
	}
	if !NumberToBool(1) {
		t.Fatal("1 should be true")
	}
	if !NumberToBool(2.5) {
		t.Fatal("2.5 should be true")
	}
	if NumberToBool(0.0) {
		t.Fatal("0.0 false")
	}
	// Test BoolMappedNumber
	b := NewBoolMappedNumber[int](5)
	if !b.Bool() {
		t.Fatal("5 should be true")
	}
	b0 := NewBoolMappedNumber[int](0)
	if b0.Bool() {
		t.Fatal("0 should be false")
	}
	// Marshal
	data, err := b.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil || v != 5 {
		t.Fatalf("marshal %v %v", v, err)
	}
	// Unmarshal
	b2 := &BoolMappedNumber[int]{}
	if err := b2.UnmarshalJSON([]byte("7")); err != nil || b2.Value != 7 {
		t.Fatalf("unmarshal %v %v", b2.Value, err)
	}
	if err := b2.UnmarshalJSON([]byte("bad")); err == nil {
		t.Fatal("should fail on bad json")
	}
	if got := NewBoolMappedNumberFromBool(true); got.Value != 1 {
		t.Fatalf("true should be 1 got %d", got.Value)
	}
	if got := NewBoolMappedNumberFromBool(false); got.Value != 0 {
		t.Fatalf("false should be 0")
	}
}

// -------------------------------------------------------------------
// config
// -------------------------------------------------------------------

func TestConfigurationGetGameHosts(t *testing.T) {
	cfg := &Configuration{
		Games: Games{
			Age1:   Game{Hosts: []string{"a1"}},
			Age2:   Game{Hosts: []string{"a2"}},
			Age3:   Game{Hosts: []string{"a3"}},
			Age4:   Game{Hosts: []string{"a4"}},
			Athens: Game{Hosts: []string{"ath"}},
		},
	}
	tests := []struct {
		id   string
		want string
	}{
		{"age1", "a1"}, {"age2", "a2"}, {"age3", "a3"}, {"age4", "a4"}, {"athens", "ath"}, {"unknown", ""},
	}
	for _, tt := range tests {
		hosts := cfg.GetGameHosts(tt.id)
		if tt.want == "" {
			if hosts != nil {
				t.Fatalf("expected nil for %s", tt.id)
			}
		} else {
			if len(hosts) != 1 || hosts[0] != tt.want {
				t.Fatalf("hosts for %s = %v want %s", tt.id, hosts, tt.want)
			}
		}
	}
}

func TestConfigurationGetGameBattleServers(t *testing.T) {
	bs := BattleServer{Base: battleServer.Base{Region: "r1", IPv4: "127.0.0.1", BsPort: 1000, WebSocketPort: 1001}}
	cfg := &Configuration{
		Games: Games{
			Age1: Game{BattleServers: []BattleServer{bs}},
			Age2: Game{BattleServers: []BattleServer{bs, bs}},
		},
	}
	if len(cfg.GetGameBattleServers("age1")) != 1 {
		t.Fatal("age1")
	}
	if len(cfg.GetGameBattleServers("age2")) != 2 {
		t.Fatal("age2")
	}
	if cfg.GetGameBattleServers("unknown") != nil {
		t.Fatal("unknown should be nil")
	}
	// Test other games nil
	if len(cfg.GetGameBattleServers("age3")) != 0 {
		t.Fatal("age3 should be empty")
	}
}

// -------------------------------------------------------------------
// domain
// -------------------------------------------------------------------

func TestSplitDomain(t *testing.T) {
	sub, mainDom, tld, err := SplitDomain("sub.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if sub != "sub" || mainDom != "example" || tld != "com" {
		t.Fatalf("got %q %q %q", sub, mainDom, tld)
	}
	// No subdomain
	sub, mainDom, tld, err = SplitDomain("example.co.uk")
	if err != nil {
		t.Fatal(err)
	}
	if mainDom != "example" || tld != "co.uk" {
		t.Fatalf("co.uk %q %q", mainDom, tld)
	}
	// Invalid
	_, _, _, err = SplitDomain("invalid")
	if err == nil {
		t.Fatal("should fail invalid")
	}
	// Case insensitive
	sub, mainDom, tld, err = SplitDomain("Sub.Example.COM")
	if err != nil || mainDom != "example" || tld != "com" {
		t.Fatalf("case %v %q %q", err, mainDom, tld)
	}
}

// -------------------------------------------------------------------
// writerPrefixer
// -------------------------------------------------------------------

type mockWriter struct {
	buf          bytes.Buffer
	failOnPrefix bool
	failOnWrite  bool
}

func (m *mockWriter) Write(p []byte) (int, error) {
	if m.failOnPrefix && bytes.Contains(p, []byte("[")) {
		return 0, io.ErrUnexpectedEOF
	}
	if m.failOnWrite {
		return 0, io.ErrUnexpectedEOF
	}
	return m.buf.Write(p)
}

func TestPrefixedWriter(t *testing.T) {
	mw := &mockWriter{}
	pw := NewPrefixedWriter(mw, "game", "test")
	n, err := pw.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("n=%d", n)
	}
	out := mw.buf.String()
	if !strings.Contains(out, "[test]") || !strings.Contains(out, "hello") {
		t.Fatalf("out=%q", out)
	}
	// Test with FileLogger nil vs not nil affects prefix
	// Already covered by NewPrefixedWriter branching
	// Test failure on prefix write
	mw2 := &mockWriter{failOnPrefix: true}
	pw2 := NewPrefixedWriter(mw2, "g", "n")
	if _, err := pw2.Write([]byte("x")); err == nil {
		t.Fatal("should fail prefix")
	}
	// Test failure on write
	mw3 := &mockWriter{}
	pw3 := &PrefixedWriter{writer: mw3, prefix: []byte("[pre] ")}
	mw3.failOnWrite = true
	if _, err := pw3.Write([]byte("x")); err == nil {
		t.Fatal("should fail write")
	}
}

// -------------------------------------------------------------------
// Map - SafeMap, SafeSet, etc
// -------------------------------------------------------------------

func TestSafeMapBasic(t *testing.T) {
	m := NewSafeMap[string, int]()
	if _, ok := m.Load("a"); ok {
		t.Fatal("should not exist")
	}
	m.Store("a", 1, nil)
	if v, ok := m.Load("a"); !ok || v != 1 {
		t.Fatal("load")
	}
	if m.Len() != 1 {
		t.Fatal("len")
	}
	// Store with replace false should not replace
	m.Store("a", 2, func(stored int) bool { return false })
	if v, _ := m.Load("a"); v != 1 {
		t.Fatal("should not replace")
	}
	// StoreAndDelete
	m.StoreAndDelete("b", 2, "a")
	if _, ok := m.Load("a"); ok {
		t.Fatal("a should be deleted")
	}
	if v, _ := m.Load("b"); v != 2 {
		t.Fatal("b")
	}
	// Delete
	m.Delete("b")
	if m.Len() != 0 {
		t.Fatal("len 0")
	}
	// CompareAndDelete
	m.Store("c", 3, nil)
	if !m.CompareAndDelete("c", func(v int) bool { return v == 3 }) {
		t.Fatal("should delete")
	}
	if m.CompareAndDelete("c", func(int) bool { return true }) {
		t.Fatal("should not delete missing")
	}
	// LoadOrStoreFn
	v, loaded := m.LoadOrStoreFn("d", func() int { return 4 })
	if loaded || v != 4 {
		t.Fatal("loadOrStore")
	}
	v, loaded = m.LoadOrStoreFn("d", func() int { return 5 })
	if !loaded || v != 4 {
		t.Fatal("should load")
	}
	// Values Iter
	m.Store("e", 5, nil)
	count := 0
	for range m.Values() {
		count++
	}
	if count == 0 {
		t.Fatal("values iter")
	}
	// Iter break early
	count = 0
	for k, _ := range m.Iter() {
		_ = k
		count++
		break
	}
	if count != 1 {
		t.Fatal("iter break")
	}
	// Marshal/Unmarshal
	data, err := m.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	m2 := NewSafeMap[string, int]()
	if err := m2.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if m2.Len() != m.Len() {
		t.Fatal("unmarshal len")
	}
	if err := m2.UnmarshalJSON([]byte("bad")); err == nil {
		t.Fatal("should fail bad json")
	}
}

func TestSafeSet(t *testing.T) {
	s := NewSafeSet[string]()
	if !s.Store("a") {
		t.Fatal("store a should be new")
	}
	if s.Store("a") {
		t.Fatal("store a again should be false")
	}
	if s.Len() != 1 {
		t.Fatal("len")
	}
	var nilSet *SafeSet[string]
	if nilSet.Len() != 0 {
		t.Fatal("nil len")
	}
	if !s.Delete("a") {
		t.Fatal("delete")
	}
	if s.Delete("a") {
		t.Fatal("delete again")
	}
}

func TestReadOnlyOrderedMap(t *testing.T) {
	m := NewReadOnlyOrderedMap([]string{"b", "a"}, map[string]int{"a": 1, "b": 2})
	if v, _ := m.Load("a"); v != 1 {
		t.Fatal("load")
	}
	if m.Len() != 2 {
		t.Fatal("len")
	}
	// Iter order should be b, a
	var order []string
	for k, _ := range m.Iter() {
		order = append(order, k)
	}
	if order[0] != "b" || order[1] != "a" {
		t.Fatalf("order %v", order)
	}
	// Values
	var vals []int
	for v := range m.Values() {
		vals = append(vals, v)
	}
	if len(vals) != 2 {
		t.Fatal("values")
	}
}

func TestSafeOrderedMap(t *testing.T) {
	m := NewSafeOrderedMap[string, int]()
	m.Store("a", 1, nil)
	m.Store("b", 2, nil)
	if v, _ := m.Load("a"); v != 1 {
		t.Fatal("load a")
	}
	if m.Len() != 2 {
		t.Fatal("len")
	}
	// Store with replace false
	m.Store("a", 10, func(s int) bool { return false })
	if v, _ := m.Load("a"); v != 1 {
		t.Fatal("should not replace")
	}
	// Store with replace true
	m.Store("a", 10, func(s int) bool { return true })
	if v, _ := m.Load("a"); v != 10 {
		t.Fatal("should replace")
	}
	// Delete
	if !m.Delete("a") {
		t.Fatal("delete")
	}
	if m.Delete("a") {
		t.Fatal("delete again")
	}
	// Keys, Values, Iter
	m.Store("c", 3, nil)
	m.Store("d", 4, nil)
	_, seq := m.Keys()
	count := 0
	for range seq {
		count++
	}
	if count != 3 { // b, c, d (a deleted)
		t.Fatalf("keys %d", count)
	}
	_, vseq := m.Values()
	count = 0
	for range vseq {
		count++
	}
	// Iter
	_, iseq := m.Iter()
	count = 0
	for range iseq {
		count++
	}
	// First
	k, v, ok := m.First()
	if !ok || k == "" {
		t.Fatal("first")
	}
	_ = v
	// IterAndStore
	m.IterAndStore("e", 5, nil, func(l int, seq2 iter.Seq2[string, int]) {
		for range seq2 {
		}
		if l != 3 {
			t.Fatalf("len %d", l)
		}
	})
	if m.Len() != 4 {
		t.Fatal("len after IterAndStore")
	}
	// Test with nil replace
	m.Store("f", 6, nil)
	if v, _ := m.Load("f"); v != 6 {
		t.Fatal("f")
	}
}

// -------------------------------------------------------------------
// http
// -------------------------------------------------------------------

type testData struct {
	Name string `schema:"name" json:"name"`
	Age  int    `schema:"age" json:"age"`
}

func TestHttpBindGet(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/?name=foo&age=30", nil)
	var d testData
	if err := Bind(req, &d); err != nil {
		t.Fatal(err)
	}
	if d.Name != "foo" || d.Age != 30 {
		t.Fatalf("%v", d)
	}
	// Unknown key should be ignored (not error)
	req2, _ := http.NewRequest(http.MethodGet, "/?name=foo&unknown=1", nil)
	var d2 testData
	if err := Bind(req2, &d2); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestHttpBindJson(t *testing.T) {
	body := `{"name":"bar","age":25}`
	req, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	var d testData
	if err := Bind(req, &d); err != nil {
		t.Fatal(err)
	}
	if d.Name != "bar" {
		t.Fatal(d.Name)
	}
	// Invalid json
	req2, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader("bad"))
	req2.Header.Set("Content-Type", "application/json")
	var d3 testData
	if err := Bind(req2, &d3); err == nil {
		t.Fatal("should fail bad json")
	}
}

func TestHttpBindForm(t *testing.T) {
	form := "name=foo&age=20"
	req, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var d testData
	if err := Bind(req, &d); err != nil {
		t.Fatal(err)
	}
	if d.Name != "foo" {
		t.Fatal(d.Name)
	}
}

func TestHttpJsonHelpers(t *testing.T) {
	// Test writeJSONHeader via JSON
	// Use a mock ResponseWriter
	mw := &mockResponseWriter{header: make(http.Header)}
	var w http.ResponseWriter = mw
	JSON(&w, H{"a": 1})
	if mw.header.Get("Content-Type") == "" {
		t.Fatal("header not set")
	}
	if mw.body.Len() == 0 {
		t.Fatal("body empty")
	}
	mw2 := &mockResponseWriter{header: make(http.Header)}
	var w2 http.ResponseWriter = mw2
	RawJSON(&w2, []byte(`{"x":1}`))
	if mw2.body.Len() == 0 {
		t.Fatal("rawjson")
	}
	// Test Json UnmarshalText
	j := Json[testData]{}
	if err := j.UnmarshalText([]byte(`{"name":"a","age":1}`)); err != nil {
		t.Fatal(err)
	}
	if j.Data.Name != "a" {
		t.Fatal(j.Data.Name)
	}
	if err := j.UnmarshalText([]byte("bad")); err == nil {
		t.Fatal("should fail")
	}
}

type mockResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (m *mockResponseWriter) Header() http.Header         { return m.header }
func (m *mockResponseWriter) Write(p []byte) (int, error) { return m.body.Write(p) }
func (m *mockResponseWriter) WriteHeader(statusCode int)  { m.status = statusCode }

// -------------------------------------------------------------------
// errors
// -------------------------------------------------------------------

func TestErrorConstants(t *testing.T) {
	// Just verify they are non-zero and distinct
	seen := map[int]bool{}
	for _, v := range []int{ErrCertDirectory, ErrResolveHost, ErrCreateLogFile, ErrStartServer, ErrMulticastGroup, ErrGames, ErrGame, ErrAnnounce, ErrInvalidId, ErrInvalidAuthentication} {
		if v == 0 {
			t.Fatal("zero")
		}
		if seen[v] {
			t.Fatal("duplicate")
		}
		seen[v] = true
	}
}

func TestKeyRWMutexRLock(t *testing.T) {
	kl := NewKeyRWMutex[string]()
	kl.RLock("a")
	kl.RUnlock("a")
	kl.RLock("b")
	kl.RUnlock("b")
	// Unlock non-existing should not panic
	kl.Unlock("nonexistent")
	kl.RUnlock("nonexistent")
}

func TestCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := &CustomWriter{OriginalWriter: &buf}
	n, err := cw.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("write %d %v", n, err)
	}
	if buf.String() != "hello" {
		t.Fatalf("buf %q", buf.String())
	}
	// TLS handshake error should be swallowed
	n, err = cw.Write([]byte("TLS handshake error from 1.2.3.4"))
	if err != nil || n != len("TLS handshake error from 1.2.3.4") {
		t.Fatalf("tls %d %v", n, err)
	}
	// Original writer should not have that content
	if buf.String() != "hello" {
		t.Fatalf("tls should not write, got %q", buf.String())
	}
}

func TestRngAndRootSignal(t *testing.T) {
	InitializeRng(42)
	var b [10]byte
	n, err := rng.Read(b[:])
	if err != nil || n != 10 {
		t.Fatalf("rng read %d %v", n, err)
	}
	// WithRng
	WithRng(func(r *RandReader) {
		_ = r.N(10)
	})
	rng.WithRng(func(r *RandReader) {
		_ = r.N(10)
	})
	InitializeStopSignal()
	if StopSignal == nil {
		t.Fatal("StopSignal nil")
	}
	// Calling again should replace channel (no panic)
	InitializeStopSignal()
}

func TestGetGameBattleServersAllGames(t *testing.T) {
	cfg := &Configuration{
		Games: Games{
			Age3:   Game{BattleServers: []BattleServer{{Base: battleServer.Base{Region: "r3"}}}},
			Age4:   Game{BattleServers: []BattleServer{{Base: battleServer.Base{Region: "r4"}}}},
			Athens: Game{BattleServers: []BattleServer{{Base: battleServer.Base{Region: "ath"}}}},
		},
	}
	if len(cfg.GetGameBattleServers("age3")) != 1 {
		t.Fatal("age3")
	}
	if len(cfg.GetGameBattleServers("age4")) != 1 {
		t.Fatal("age4")
	}
	if len(cfg.GetGameBattleServers("athens")) != 1 {
		t.Fatal("athens")
	}
}

func TestSafeMapEdgeCases(t *testing.T) {
	m := NewSafeMap[string, int]()
	// CompareAndDelete with nil func (should not delete)
	m.Store("x", 1, nil)
	if m.CompareAndDelete("x", nil) {
		t.Fatal("nil compare should be false")
	}
	// LoadOrStore with existing
	m.Store("y", 2, nil)
	if v, loaded := m.LoadOrStoreFn("y", func() int { return 99 }); !loaded || v != 2 {
		t.Fatal("loadOrStore existing")
	}
	// Values with break early
	m.Store("z", 3, nil)
	count := 0
	for v := range m.Values() {
		_ = v
		count++
		if count == 1 {
			break
		}
	}
	// Iter with break
	count = 0
	for range m.Iter() {
		count++
		break
	}
	// ReadOnlyOrderedMap Values with break
	ro := NewReadOnlyOrderedMap([]string{"a", "b"}, map[string]int{"a": 1, "b": 2})
	count = 0
	for range ro.Values() {
		count++
		break
	}
	// SafeOrderedMap Delete non-existing
	som := NewSafeOrderedMap[string, int]()
	if som.Delete("nonexistent") {
		t.Fatal("should be false")
	}
	som.Store("a", 1, nil)
	som.Store("b", 2, nil)
	// Test Keys with break
	_, seq := som.Keys()
	count = 0
	for range seq {
		count++
		break
	}
	// Test Values with break
	_, vseq := som.Values()
	count = 0
	for range vseq {
		count++
		break
	}
	// Test iter with break
	_, iseq := som.Iter()
	count = 0
	for range iseq {
		count++
		break
	}
	// Test First on empty
	empty := NewSafeOrderedMap[string, int]()
	if _, _, ok := empty.First(); ok {
		t.Fatal("should be not ok")
	}
}
