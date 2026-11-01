package models

import (
	"context"
	"net/http"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
)

// testUser es un User mínimo para ejercitar modelos que solo necesitan GetId o
// EncodeProfileInfo.
type testUser struct {
	User
	id      int32
	profile i.A
}

func (t *testUser) GetId() int32                 { return t.id }
func (t *testUser) EncodeProfileInfo(uint16) i.A { return t.profile }

func TestMainSessionsCrud(t *testing.T) {
	i.InitializeRng(42)
	sessions := &MainSessions{}
	sessions.Initialize()

	sid := sessions.Create(100, 200)
	if sid == "" {
		t.Fatal("empty session id")
	}

	sess, ok := sessions.GetById(sid)
	if !ok {
		t.Fatal("session not found by id")
	}
	if sess.GetUserId() != 100 {
		t.Fatalf("userId = %d, want 100", sess.GetUserId())
	}
	if sess.GetClientLibVersion() != 200 {
		t.Fatalf("clientLibVersion = %d, want 200", sess.GetClientLibVersion())
	}
	if sess.Id() != sid {
		t.Fatalf("id mismatch")
	}

	if _, ok := sessions.GetByUserId(100); !ok {
		t.Fatal("session not found by userId")
	}
	if _, ok := sessions.GetByUserId(999); ok {
		t.Fatal("unexpected session for unknown userId")
	}

	sessions.ResetExpiry(sid)

	sessions.Delete(sid)
	if _, ok := sessions.GetById(sid); ok {
		t.Fatal("session not deleted")
	}
}

func TestChatChannel(t *testing.T) {
	channel := NewChatChannel(1, "lobby")
	if channel.GetId() != 1 {
		t.Fatalf("id = %d", channel.GetId())
	}
	if channel.GetName() != "lobby" {
		t.Fatalf("name = %q", channel.GetName())
	}

	enc := channel.Encode()
	if enc[0].(int32) != 1 || enc[1].(string) != "lobby" || enc[3].(int) != 0 {
		t.Fatalf("encode = %v", enc)
	}

	u := &testUser{id: 5, profile: i.A{0, i.A{5}}}
	if channel.HasUser(u) {
		t.Fatal("channel should not have user yet")
	}

	exists, _ := channel.AddUser(u, 200)
	if exists {
		t.Fatal("AddUser should not report existing on first add")
	}
	if !channel.HasUser(u) {
		t.Fatal("channel should have user after add")
	}

	if !channel.RemoveUser(u) {
		t.Fatal("RemoveUser should return true")
	}
	if channel.HasUser(u) {
		t.Fatal("channel should not have user after remove")
	}
}

func TestMainChatChannels(t *testing.T) {
	channels := &MainChatChannels{}
	src := map[string]*MainChatChannel{
		"10": NewChatChannel(10, "ten"),
		"20": NewChatChannel(20, "twenty"),
	}
	channels.Initialize(src)

	if c, ok := channels.GetById(10); !ok || c.GetName() != "ten" {
		t.Fatalf("GetById(10) = %v, %v", c, ok)
	}
	if _, ok := channels.GetById(99); ok {
		t.Fatal("unexpected channel 99")
	}
	if enc := channels.Encode(); len(enc) != 2 {
		t.Fatalf("Encode len = %d", len(enc))
	}
	count := 0
	for range channels.Iter() {
		count++
	}
	if count != 2 {
		t.Fatalf("iter count = %d", count)
	}
}

func TestPeer(t *testing.T) {
	peer := NewPeer(10, "1.2.3.4", 100, 5, -1, 1, 2)
	if peer.GetUserId() != 100 {
		t.Fatalf("userId = %d", peer.GetUserId())
	}
	if peer.GetParty() != -1 {
		t.Fatalf("party = %d", peer.GetParty())
	}

	enc := peer.Encode()
	if enc[0].(int32) != 10 || enc[1].(int32) != 100 || enc[4].(int32) != 1 || enc[5].(int32) != 2 {
		t.Fatalf("encode = %v", enc)
	}

	peer.UpdateMutable(3, 4)
	m := peer.GetMutable()
	if m.RaceID != 3 || m.TeamID != 4 {
		t.Fatalf("mutable = %v", m)
	}

	u := &testUser{id: 7}
	if !peer.Invite(u) {
		t.Fatal("first invite should succeed")
	}
	if peer.Invite(u) {
		t.Fatal("second invite should fail")
	}
	if !peer.Uninvite(u) {
		t.Fatal("uninvite should succeed")
	}
}

func TestMessage(t *testing.T) {
	sender := &testUser{id: 42}
	msg := &MainMessage{
		advertisementId: 9,
		time:            123456,
		broadcast:       true,
		content:         "hello",
		messageType:     1,
		sender:          sender,
		receivers:       []User{&testUser{id: 77}},
	}

	if msg.GetTime() != 123456 || !msg.GetBroadcast() || msg.GetContent() != "hello" ||
		msg.GetMessageType() != 1 || msg.GetAdvertisementId() != 9 {
		t.Fatalf("getters wrong")
	}
	if msg.GetSender().GetId() != 42 {
		t.Fatalf("sender id = %d", msg.GetSender().GetId())
	}
	if len(msg.GetReceivers()) != 1 {
		t.Fatalf("receivers len = %d", len(msg.GetReceivers()))
	}
	if msg.GetMetadata() != "" {
		t.Fatalf("metadata = %q", msg.GetMetadata())
	}

	enc := msg.Encode(193)
	if enc[0].(int32) != 42 || enc[3].(uint8) != 1 || len(enc) != 5 {
		t.Fatalf("encode(193) = %v", enc)
	}

	msgWithMeta := &MainMessage{metadata: "meta", sender: sender}
	encWith := msgWithMeta.Encode(194)
	if len(encWith) != 6 || encWith[5].(string) != "meta" {
		t.Fatalf("encode(194) = %v", encWith)
	}
}

func TestItemLoadout(t *testing.T) {
	i.InitializeRng(42)
	l := &MainItemLoadout{LoadoutID: 1, LoadoutName: "load", LoadoutType: 2}
	enc := l.Encode(50)
	if enc[0].(int32) != 1 || enc[1].(int32) != 50 || enc[2].(string) != "load" {
		t.Fatalf("encode = %v", enc)
	}

	l.Update("renamed", 3, mapset.NewSet[int32]())
	if l.LoadoutName != "renamed" || l.LoadoutType != 3 {
		t.Fatalf("update failed")
	}

	loadouts := &MainItemLoadouts{ItemLoadouts: make(map[int32]ItemLoadout)}
	if loadouts.Get(999) != nil {
		t.Fatal("Get should return nil for missing")
	}
	_ = loadouts.NewItemLoadout("new", 1, mapset.NewSet[int32](), 60)
	iterated := 0
	for range loadouts.Iter() {
		iterated++
	}
	if iterated != 1 {
		t.Fatalf("iter count = %d", iterated)
	}
}

func TestAvatarStatsRoundTrip(t *testing.T) {
	stat := NewAvatarStat(5, 10)
	if stat.StatID != 5 || stat.Value != 10 {
		t.Fatalf("stat = %v", stat)
	}
	stat.SetValue(20)
	if stat.Value != 20 {
		t.Fatalf("value = %d", stat.Value)
	}

	enc := stat.Encode(7)
	if enc[0].(int32) != 5 || enc[1].(int32) != 7 || enc[2].(int64) != 20 {
		t.Fatalf("encode = %v", enc)
	}

	stats := newAvatarStats(map[int32]int64{1: 100, 2: 200})
	data, err := stats.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored AvatarStats
	if err := restored.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s, ok := restored.GetStat(1); !ok || s.Value != 100 {
		t.Fatalf("round-trip stat = %v, %v", s, ok)
	}
	if _, ok := restored.GetStat(99); ok {
		t.Fatal("unexpected stat 99")
	}

	stats.AddStat(AvatarStat{StatID: 3, Value: 300})
	if len(stats.Encode(1)) != 3 {
		t.Fatalf("encode len = %d", len(stats.Encode(1)))
	}
}

func TestBattleServer(t *testing.T) {
	bs := &MainBattleServer{
		Base: battleServer.Base{
			Region: "test-region",
		},
	}
	bs.SetIPv4("10.0.0.1")
	bs.SetBsPort(1000)
	bs.SetWebSocketPort(2000)
	bs.SetOutOfBandPort(3000)
	bs.SetHasOobPort(true)
	bs.SetName("server-name")
	bs.SetBattleServerName("true")

	if bs.Region() != "test-region" {
		t.Fatalf("region = %q", bs.Region())
	}
	if bs.LAN() {
		t.Fatal("non-uuid region should be non-LAN")
	}
	bs.SetLAN(true)
	if !bs.LAN() {
		t.Fatal("SetLAN(true) failed")
	}

	ports := bs.EncodePorts()
	if ports[0].(int) != 1000 || ports[1].(int) != 2000 || ports[2].(int) != 3000 {
		t.Fatalf("ports = %v", ports)
	}

	// Sin OOB port no debe incluir el tercer puerto.
	bsNoOob := &MainBattleServer{Base: battleServer.Base{Region: "r"}}
	bsNoOob.SetBsPort(1)
	bsNoOob.SetWebSocketPort(2)
	bsNoOob.SetHasOobPort(false)
	if len(bsNoOob.EncodePorts()) != 2 {
		t.Fatalf("ports without oob = %v", bsNoOob.EncodePorts())
	}

	if ip := bs.ResolveIPv4(nil); ip != "10.0.0.1" {
		t.Fatalf("resolve ipv4 = %q", ip)
	}

	_ = bs.String()
}

func TestMainBattleServers(t *testing.T) {
	bs := &MainBattleServer{Base: battleServer.Base{Region: "r1"}}
	bs2 := &MainBattleServer{Base: battleServer.Base{Region: "r2"}}
	servers := &MainBattleServers{}
	servers.Initialize([]BattleServer{bs, bs2}, &BattleServerOpts{OobPort: false, Name: "null"})

	if _, ok := servers.Get("r1"); !ok {
		t.Fatal("r1 not found")
	}
	if _, ok := servers.Get("r9"); ok {
		t.Fatal("unexpected r9")
	}
	if enc := servers.Encode(nil); len(enc) != 2 {
		t.Fatalf("encode len = %d", len(enc))
	}

	lan := servers.NewLANBattleServer("r3")
	if !lan.LAN() {
		t.Fatal("LAN battle server should be LAN")
	}
	normal := servers.NewBattleServer("r4")
	if normal.LAN() {
		t.Fatal("normal battle server should not be LAN by default")
	}
}

func TestAuthAndDefaultData(t *testing.T) {
	if d := NewAuthUpgradableDefaultData().Default(); d == nil || !d.IsZero() {
		t.Fatal("auth default should be zero time")
	}
	if d := NewProfilePropertiesUpgradableDefaultData().Default(); d == nil {
		t.Fatal("profile properties default nil")
	}
	if d := NewAvatarMetadataUpgradableDefaultData(game.AoE3).Default(); d == nil || *d == "" {
		t.Fatal("AoE3 avatar metadata default empty")
	}
	if d := NewAvatarMetadataUpgradableDefaultData(game.AoE1).Default(); d == nil || *d != "" {
		t.Fatal("AoE1 avatar metadata default should be empty string")
	}
}

// fakeResources implementa Resources en memoria para ejercitar CreateMainGame sin
// depender del sistema de ficheros.
type fakeResources struct {
	arrayFiles   map[string]i.A
	signedAssets map[string][]byte
	chatChannels map[string]*MainChatChannel
	loginData    []i.A
}

func (f *fakeResources) Initialize(string, *ResourcesOpts) {
	f.arrayFiles = map[string]i.A{
		"leaderboards.json":  i.A{i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}, i.A{}},
		"itemLocations.json": i.A{},
		"presenceData.json":  i.A{},
	}
	f.signedAssets = map[string][]byte{
		"itemDefinitions.json": []byte(`{"itemCategories":[],"itemDefinitions":[]}`),
	}
	f.chatChannels = map[string]*MainChatChannel{"1": NewChatChannel(1, "lobby")}
	f.loginData = []i.A{}
}

func (f *fakeResources) ReturnSignedAsset(string, *http.ResponseWriter, *http.Request, bool) {}
func (f *fakeResources) LoginData() []i.A                                                    { return f.loginData }
func (f *fakeResources) ChatChannels() map[string]*MainChatChannel                           { return f.chatChannels }
func (f *fakeResources) News() []i.A                                                         { return nil }
func (f *fakeResources) ArrayFiles() map[string]i.A                                          { return f.arrayFiles }
func (f *fakeResources) SignedAssets() map[string][]byte                                     { return f.signedAssets }
func (f *fakeResources) CloudFiles() CloudFiles {
	return CloudFiles{Credentials: NewCredentials(), Value: map[string]CloudfilesIndex{}}
}

func TestCreateMainGame(t *testing.T) {
	i.InitializeRng(42)
	opts := &CreateMainGameOpts{
		Instances: &InstanceOpts{
			Resources: &fakeResources{},
		},
	}
	g := CreateMainGame("age2", opts)

	if g.Title() != "age2" {
		t.Fatalf("title = %q", g.Title())
	}
	if g.Resources() == nil {
		t.Fatal("resources nil")
	}
	if g.Users() == nil {
		t.Fatal("users nil")
	}
	if g.Advertisements() == nil {
		t.Fatal("advertisements nil")
	}
	if g.ChatChannels() == nil {
		t.Fatal("chatChannels nil")
	}
	if g.Sessions() == nil {
		t.Fatal("sessions nil")
	}
	if g.BattleServers() == nil {
		t.Fatal("battleServers nil")
	}
	if g.LeaderboardDefinitions() == nil {
		t.Fatal("leaderboards nil")
	}
	if g.Items() == nil {
		t.Fatal("items nil")
	}
	if g.PresenceDefinitions() == nil {
		t.Fatal("presence nil")
	}
}

func TestGameHelpers(t *testing.T) {
	i.InitializeRng(42)
	g := CreateMainGame("age2", &CreateMainGameOpts{
		Instances: &InstanceOpts{Resources: &fakeResources{}},
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), "game", g))
	if got := G(req); got == nil || got.Title() != "age2" {
		t.Fatalf("G() = %v", got)
	}
	if got := Gg[Game](req); got == nil {
		t.Fatal("Gg() nil")
	}
}
