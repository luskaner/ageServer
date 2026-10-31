package models

import (
	"encoding/json"
	"iter"
	"maps"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	i "github.com/luskaner/ageServer/server/internal"
)

type ItemCategoryRaw struct {
	Id            int32                 `json:"categoryID"`
	Name          string                `json:"name"`
	Group         string                `json:"categoryGroup"`
	Metadata      *ReadOnlyItemMetadata `json:"metadata"`
	LocalizedName string                `json:"localizedName,omitempty"`
}

type ItemDefinitionRaw struct {
	Id                   int32                 `json:"id"`
	Name                 string                `json:"name"`
	ImageData            string                `json:"imageData"`
	MetaData             *ReadOnlyItemMetadata `json:"metaData"`
	Available            int32                 `json:"available"`
	Version              int32                 `json:"version"`
	Level                int32                 `json:"level"`
	CategoryIDs          []int32               `json:"categoryIDs"`
	LocalizedName        string                `json:"localizedName,omitempty"`
	LocalizedDescription string                `json:"localizedDescription,omitempty"`
}

type ItemsRaw struct {
	Categories  []ItemCategoryRaw   `json:"itemCategories"`
	Definitions []ItemDefinitionRaw `json:"itemDefinitions"`
}

type ItemCategory interface {
	GetId() int32
	GetName() string
	GetGroup() string
	GetMetadata() *ReadOnlyItemMetadata
	GetLocalizedName() string
}

type MainItemCategory struct {
	id            int32
	name          string
	group         string
	metadata      *ReadOnlyItemMetadata
	localizedName string
}

func (c *MainItemCategory) GetId() int32 {
	return c.id
}

func (c *MainItemCategory) GetName() string {
	return c.name
}

func (c *MainItemCategory) GetGroup() string {
	return c.group
}

func (c *MainItemCategory) GetMetadata() *ReadOnlyItemMetadata {
	return c.metadata
}

func (c *MainItemCategory) GetLocalizedName() string {
	return c.localizedName
}

type Attribute struct {
	Value  string `json:"val"`
	Source int    `json:"source,omitempty,omitzero"`
	Mtype  int    `json:"mtype,omitempty,omitzero"`
	Vtype  int    `json:"vtype,omitempty,omitzero"`
}

// isNewAttributeKey is a client side transient flag. It never belongs to the
// persisted item metadata nor to the att representation sent to the client.
const isNewAttributeKey = "is_new"

type ReadOnlyItemMetadata struct {
	attributes map[string]Attribute
	other      map[string]json.RawMessage
}

func (r *ReadOnlyItemMetadata) GetAttribute(key string) (Attribute, bool) {
	if r.attributes == nil {
		return Attribute{}, false
	}
	attr, ok := r.attributes[key]
	return attr, ok
}

func (r *ReadOnlyItemMetadata) UnmarshalJSON(data []byte) error {
	var all map[string]json.RawMessage
	var allStr string
	var allData []byte
	if err := json.Unmarshal(data, &allStr); err != nil {
		return err
	} else if allStr == "" {
		return nil
	}
	allData = []byte(allStr)
	if err := json.Unmarshal(allData, &all); err != nil {
		return err
	}
	if attributes, ok := all["att"]; ok {
		if err := json.Unmarshal(attributes, &r.attributes); err != nil {
			return err
		}
		delete(all, "att")
	}
	if len(all) > 0 {
		r.other = maps.Clone(all)
	}
	return nil
}

type ItemMetadata struct {
	// attributes keeps the complete client side attribute shape so that the
	// source/mtype declared by the item definitions survives being saved and
	// loaded back. Encode() hands them to the client as the att object.
	attributes map[string]Attribute
	other      map[string]any
}

func NewItemMetadataRaw(metadata []byte) ItemMetadata {
	roMetadata := &ReadOnlyItemMetadata{}
	itemMetadata := ItemMetadata{}
	if err := json.Unmarshal(metadata, roMetadata); err != nil {
		return itemMetadata
	}
	if len(roMetadata.attributes) > 0 {
		itemMetadata.attributes = maps.Clone(roMetadata.attributes)
	}
	if len(roMetadata.other) > 0 {
		itemMetadata.other = make(map[string]any, len(roMetadata.other))
		for k, v := range roMetadata.other {
			var val any
			if err := json.Unmarshal(v, &val); err == nil {
				itemMetadata.other[k] = val
			}
		}
	}
	return itemMetadata
}

func (i *ItemMetadata) MarshalJSON() ([]byte, error) {
	all := make(map[string]any)
	if len(i.attributes) > 0 {
		all["attributes"] = i.attributes
	}
	if len(i.other) > 0 {
		all["other"] = i.other
	}
	return json.Marshal(all)
}

func (i *ItemMetadata) UnmarshalJSON(data []byte) error {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	if attributes, ok := all["attributes"]; ok {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(attributes, &raw); err != nil {
			return err
		}
		i.attributes = make(map[string]Attribute, len(raw))
		for k, v := range raw {
			if k == isNewAttributeKey {
				continue
			}
			attr, err := unmarshalAttribute(v)
			if err != nil {
				return err
			}
			i.attributes[k] = attr
		}
	}
	if other, ok := all["other"]; ok {
		if err := json.Unmarshal(other, &i.other); err != nil {
			return err
		}
	}
	return nil
}

// unmarshalAttribute reads an attribute saved as an att object. Documents
// written before the attributes kept their source/mtype store the value alone,
// those are read back with the values AoE4's item definitions declare for
// persistent attributes.
func unmarshalAttribute(data json.RawMessage) (Attribute, error) {
	var attr Attribute
	if err := json.Unmarshal(data, &attr); err == nil {
		return attr, nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return Attribute{}, err
	}
	return Attribute{Value: value, Source: 1, Mtype: 1}, nil
}

func (i *ItemMetadata) Encode() (result string) {
	all := make(map[string]any, len(i.other)+1)
	if len(i.attributes) > 0 {
		all["att"] = i.attributes
	}
	maps.Copy(all, i.other)
	if res, err := json.Marshal(all); err == nil {
		result = string(res)
	}
	return
}

func (i *ItemMetadata) GetAttribute(key string) (string, bool) {
	attr, ok := i.attributes[key]
	if !ok {
		return "", false
	}
	return attr.Value, true
}

// UpdateAttribute stores value under key and reports whether it was accepted.
// Keys that are not valid item metadata, such as the transient is_new flag,
// are rejected instead of being silently dropped.
func (i *ItemMetadata) UpdateAttribute(key string, value string) bool {
	if key == isNewAttributeKey {
		return false
	}
	if i.attributes == nil {
		i.attributes = make(map[string]Attribute)
	}
	attr := i.attributes[key]
	if attr.Source == 0 {
		attr.Source = 1
	}
	if attr.Mtype == 0 {
		attr.Mtype = 1
	}
	attr.Value = value
	i.attributes[key] = attr
	return true
}

func (i *ItemMetadata) UpdateOther(key string, value any) {
	if i.other == nil {
		i.other = make(map[string]any)
	}
	i.other[key] = value
}

func (i *ItemMetadata) RemoveOther(key string) {
	if i.other == nil {
		return
	}
	delete(i.other, key)
}

type ItemDefinition interface {
	GetId() int32
	GetName() string
	GetMetadata() *ReadOnlyItemMetadata
	GetAvailable() int32
	GetVersion() int32
	GetLevel() int32
	GetCategories() mapset.Set[ItemCategory]
	GetLocalizedName() string
	GetLocalizedDescription() string
}

type MainItemDefinition struct {
	id                   int32
	name                 string
	metadata             *ReadOnlyItemMetadata
	available            int32
	version              int32
	level                int32
	categoryIDs          mapset.Set[ItemCategory]
	localizedName        string
	localizedDescription string
}

func (d *MainItemDefinition) GetId() int32 {
	return d.id
}

func (d *MainItemDefinition) GetName() string {
	return d.name
}

func (d *MainItemDefinition) GetMetadata() *ReadOnlyItemMetadata {
	return d.metadata
}

func (d *MainItemDefinition) GetAvailable() int32 {
	return d.available
}

func (d *MainItemDefinition) GetVersion() int32 {
	return d.version
}

func (d *MainItemDefinition) GetLevel() int32 {
	return d.level
}

func (d *MainItemDefinition) GetCategories() mapset.Set[ItemCategory] {
	return d.categoryIDs
}

func (d *MainItemDefinition) GetLocalizedName() string {
	return d.localizedName
}

func (d *MainItemDefinition) GetLocalizedDescription() string {
	return d.localizedDescription
}

type Item interface {
	Encode(profileID int32) i.A
	GetId() int32
	SetLocationID(locationID int32)
	GetMetadata() *ItemMetadata
	IncrementVersion()
	SetDurability(durability int32)
}

type itemStorage struct {
	Id              int32
	Metadata        ItemMetadata
	DefinitionId    int32
	DurabilityId    int32 `json:",omitempty"`
	DurabilityCount int32 `json:",omitempty"`
	CreationDate    time.Time
	LocationId      int32  `json:",omitempty"`
	TradeId         int32  `json:",omitempty"`
	PermissionFlags uint32 `json:",omitempty"`
	MaxCharges      int32  `json:",omitempty"`
}

type MainItem struct {
	id                int32
	metadata          ItemMetadata
	entityVersion     int32
	itemDefinitionID  int32
	durabilityType    int32
	durability        int32
	creationDate      time.Time
	locationID        int32
	tradeID           int32
	permission        uint32
	maxChargesPerItem int32
}

func newMainItemFromStorage(storage *itemStorage) *MainItem {
	return &MainItem{
		id:                storage.Id,
		metadata:          storage.Metadata,
		itemDefinitionID:  storage.DefinitionId,
		durabilityType:    storage.DurabilityId,
		durability:        storage.DurabilityCount,
		creationDate:      storage.CreationDate,
		locationID:        storage.LocationId,
		tradeID:           storage.TradeId,
		permission:        storage.PermissionFlags,
		maxChargesPerItem: storage.MaxCharges,
	}
}

func NewMainItemFromRaw(itemRaw i.A) *MainItem {
	return &MainItem{
		id:                int32(itemRaw[0].(float64)),
		metadata:          NewItemMetadataRaw([]byte(itemRaw[6].(string))),
		entityVersion:     int32(itemRaw[1].(float64)),
		itemDefinitionID:  int32(itemRaw[2].(float64)),
		durabilityType:    int32(itemRaw[5].(float64)),
		durability:        int32(itemRaw[4].(float64)),
		creationDate:      time.Unix(int64(itemRaw[7].(float64)), 0),
		locationID:        int32(itemRaw[8].(float64)),
		tradeID:           int32(itemRaw[9].(float64)),
		permission:        uint32(itemRaw[10].(float64)),
		maxChargesPerItem: int32(itemRaw[11].(float64)),
	}
}

func (item *MainItem) MarshalJSON() ([]byte, error) {
	storage := &itemStorage{
		Id:              item.id,
		Metadata:        item.metadata,
		DefinitionId:    item.itemDefinitionID,
		DurabilityId:    item.durabilityType,
		DurabilityCount: item.durability,
		CreationDate:    item.creationDate,
		LocationId:      item.locationID,
		TradeId:         item.tradeID,
		PermissionFlags: item.permission,
		MaxCharges:      item.maxChargesPerItem,
	}
	return json.Marshal(storage)
}

func (item *MainItem) UnmarshalJSON(data []byte) error {
	var storage itemStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return err
	}
	*item = *newMainItemFromStorage(&storage)
	return nil
}

func (item *MainItem) GetId() int32 {
	return item.id
}

func (item *MainItem) SetLocationID(locationID int32) {
	item.locationID = locationID
}

func (item *MainItem) Encode(profileID int32) i.A {
	return i.A{
		item.id,
		item.entityVersion,
		item.itemDefinitionID,
		profileID,
		item.durability,
		item.durabilityType,
		item.metadata.Encode(),
		item.creationDate.Unix(),
		item.locationID,
		item.tradeID,           // -1 = no trade
		item.permission,        // bit flags
		item.maxChargesPerItem, // -1 = no charges
	}
}

func (item *MainItem) GetMetadata() *ItemMetadata {
	return &item.metadata
}

func (item *MainItem) SetDurability(durability int32) {
	item.durability = durability
}

func (item *MainItem) IncrementVersion() {
	item.entityVersion++
}

type Items interface {
	GetLocation(locationID int32) (ItemLocation, bool)
	EncodeLocations() i.A
	GetCategory(id int32) (ItemCategory, bool)
	IterCategories() iter.Seq[ItemCategory]
	GetDefinition(id int32) (ItemDefinition, bool)
	IterDefinitions() iter.Seq[ItemDefinition]
	Initialize(itemDefinitions []byte, itemLocations i.A)
}

// ItemLocation mirrors the client's item location object. Encode writes the
// fields in the client's positional order: id, entityVersion, locationID,
// categoryID, maxItems, locked, isPublic, isForMatch, tracksMoves.
type ItemLocation interface {
	GetId() int32
	GetEntityVersion() int32
	GetLocationID() int32
	GetCategoryID() int32
	GetMaxItems() int32
	IsLocked() bool
	IsPublic() bool
	IsForMatch() bool
	TracksMoves() bool
	Encode() i.A
}

type MainItemLocation struct {
	id            int32
	entityVersion int32
	locationID    int32
	categoryID    int32
	maxItems      int32
	locked        *i.BoolMappedNumber[int32]
	isPublic      *i.BoolMappedNumber[int32]
	isForMatch    *i.BoolMappedNumber[int32]
	tracksMoves   *i.BoolMappedNumber[int32]
}

func (loc *MainItemLocation) GetId() int32 {
	return loc.id
}

func (loc *MainItemLocation) GetEntityVersion() int32 {
	return loc.entityVersion
}

func (loc *MainItemLocation) GetLocationID() int32 {
	return loc.locationID
}

func (loc *MainItemLocation) GetCategoryID() int32 {
	return loc.categoryID
}

func (loc *MainItemLocation) GetMaxItems() int32 {
	return loc.maxItems
}

func (loc *MainItemLocation) IsLocked() bool {
	return loc.locked.Bool()
}

func (loc *MainItemLocation) IsPublic() bool {
	return loc.isPublic.Bool()
}

func (loc *MainItemLocation) IsForMatch() bool {
	return loc.isForMatch.Bool()
}

func (loc *MainItemLocation) TracksMoves() bool {
	return loc.tracksMoves.Bool()
}

func (loc *MainItemLocation) Encode() i.A {
	return i.A{
		loc.id,
		loc.entityVersion,
		loc.locationID,
		loc.categoryID,
		loc.maxItems,
		loc.locked,      // 0/1
		loc.isPublic,    // 0/1
		loc.isForMatch,  // 0/1
		loc.tracksMoves, // 0/1
	}
}

type MainItems struct {
	categories  map[int32]ItemCategory
	definitions map[int32]ItemDefinition
	locations   map[int32]ItemLocation
}

func (m *MainItems) IterDefinitions() iter.Seq[ItemDefinition] {
	return maps.Values(m.definitions)
}

func (m *MainItems) IterCategories() iter.Seq[ItemCategory] {
	return maps.Values(m.categories)
}

func (m *MainItems) EncodeLocations() i.A {
	result := make(i.A, len(m.locations))
	for j, location := range m.locations {
		result[j] = location.Encode()
	}
	return result
}

func (m *MainItems) GetCategory(id int32) (ItemCategory, bool) {
	val, ok := m.categories[id]
	return val, ok
}

func (m *MainItems) GetDefinition(id int32) (ItemDefinition, bool) {
	val, ok := m.definitions[id]
	return val, ok
}

func (m *MainItems) GetLocation(locationID int32) (ItemLocation, bool) {
	val, ok := m.locations[locationID]
	return val, ok
}

func (m *MainItems) Initialize(itemDefinitions []byte, itemLocations i.A) {
	var definitions ItemsRaw
	if err := json.Unmarshal(itemDefinitions, &definitions); err != nil {
		return
	}
	m.categories = make(map[int32]ItemCategory)
	for _, categoryRaw := range definitions.Categories {
		metadata := categoryRaw.Metadata
		if metadata != nil && len(metadata.attributes) == 0 && len(metadata.other) == 0 {
			metadata = nil
		}
		category := &MainItemCategory{
			id:            categoryRaw.Id,
			name:          categoryRaw.Name,
			group:         categoryRaw.Group,
			metadata:      metadata,
			localizedName: categoryRaw.LocalizedName,
		}
		m.categories[category.id] = category
	}
	m.definitions = make(map[int32]ItemDefinition)
	for _, definitionRaw := range definitions.Definitions {
		categoryIDs := mapset.NewSet[ItemCategory]()
		for _, categoryID := range definitionRaw.CategoryIDs {
			if category, ok := m.categories[categoryID]; ok {
				categoryIDs.Add(category)
			}
		}
		metadata := definitionRaw.MetaData
		if metadata != nil && len(metadata.attributes) == 0 && len(metadata.other) == 0 {
			metadata = nil
		}
		definition := &MainItemDefinition{
			id:                   definitionRaw.Id,
			name:                 definitionRaw.Name,
			metadata:             metadata,
			available:            definitionRaw.Available,
			version:              definitionRaw.Version,
			level:                definitionRaw.Level,
			categoryIDs:          categoryIDs,
			localizedName:        definitionRaw.LocalizedName,
			localizedDescription: definitionRaw.LocalizedDescription,
		}
		m.definitions[definition.id] = definition
	}
	m.locations = make(map[int32]ItemLocation)
	for _, locationRawAny := range itemLocations {
		locationRawArr := locationRawAny.(i.A)
		location := &MainItemLocation{
			id:            int32(locationRawArr[0].(float64)),
			entityVersion: int32(locationRawArr[1].(float64)),
			locationID:    int32(locationRawArr[2].(float64)),
			categoryID:    int32(locationRawArr[3].(float64)),
			maxItems:      int32(locationRawArr[4].(float64)),
			locked:        i.NewBoolMappedNumber(int32(locationRawArr[5].(float64))),
			isPublic:      i.NewBoolMappedNumber(int32(locationRawArr[6].(float64))),
			isForMatch:    i.NewBoolMappedNumber(int32(locationRawArr[7].(float64))),
			tracksMoves:   i.NewBoolMappedNumber(int32(locationRawArr[8].(float64))),
		}
		m.locations[location.locationID] = location
	}
}

type ReadOnlyCategories struct {
	fromId map[int]ItemCategory
}

func (c *ReadOnlyCategories) GetById(id int) (ItemCategory, bool) {
	itemCategory, ok := c.fromId[id]
	return itemCategory, ok
}

type ReadOnlyItemDefinitions struct {
	fromId   map[int]ItemDefinition
	fromName map[string]ItemDefinition
}

func (d *ReadOnlyItemDefinitions) GetById(id int) (ItemDefinition, bool) {
	itemDefinition, ok := d.fromId[id]
	return itemDefinition, ok
}

func (d *ReadOnlyItemDefinitions) GetByName(name string) (ItemDefinition, bool) {
	itemDefinition, ok := d.fromName[name]
	return itemDefinition, ok
}

// AoE IV items that the client places outside of the regular equipment slots,
// mirroring the static location and eslots data the game ships with.
const (
	coatOfArmsDefinitionId int32 = 451923
	monumentDefinitionId   int32 = 451960
	portraitDefinitionId   int32 = 452595
)

const (
	portraitLocationId   int32 = 3
	coatOfArmsLocationId int32 = 4
	monumentLocationId   int32 = 5
)

// sigilPosAttributeKey holds the sigil position the client reports through
// updateItemAttributes, so it must never be seeded from the definition.
const sigilPosAttributeKey = "sigil_pos"

const defaultSigilPos = "001_centre_large_single"

// categoryDefaultEslots maps an AoE IV item category to the eslot value the
// client expects in the item metadata. It applies to every definition holding
// one of these categories, not only to the Coat of Arms.
var categoryDefaultEslots = []struct {
	categoryID int32
	slot       string
}{
	{1405716, "0"},
	{1405715, "1"},
	{1450297, "2"},
	{1405717, "3"},
	{1450298, "4"},
	{1405720, "5"},
	{1450299, "6"},
	{1405718, "7"},
	{1407415, "8"},
	{1405719, "9"},
	{1450300, "10"},
}

// setDefaultEslot seeds eslot from the categories of an item definition.
func (i *ItemMetadata) setDefaultEslot(categories mapset.Set[ItemCategory]) {
	for _, category := range categories.ToSlice() {
		for _, defaultEslot := range categoryDefaultEslots {
			if category.GetId() == defaultEslot.categoryID {
				i.UpdateOther("eslot", defaultEslot.slot)
				return
			}
		}
	}
}

// defaultItemLocationId returns the location new profiles start an item at.
func defaultItemLocationId(itemDefinitionID int32) int32 {
	switch itemDefinitionID {
	case portraitDefinitionId:
		return portraitLocationId
	case coatOfArmsDefinitionId:
		return coatOfArmsLocationId
	case monumentDefinitionId:
		return monumentLocationId
	}
	return 0
}

type ItemsUpgradableDefaultData struct {
	InitialUpgradableDefaultData[*map[int32]*MainItem]
	gameId      string
	definitions Items
}

func NewItemsUpgradableDefaultData(gameId string, definitions Items) *ItemsUpgradableDefaultData {
	return &ItemsUpgradableDefaultData{
		InitialUpgradableDefaultData: InitialUpgradableDefaultData[*map[int32]*MainItem]{},
		gameId:                       gameId,
		definitions:                  definitions,
	}
}

func (is *ItemsUpgradableDefaultData) Default() *map[int32]*MainItem {
	var items []*MainItem
	var itemPackCategory ItemCategory
	for category := range is.definitions.IterCategories() {
		if category.GetName() == "ItemPack" {
			itemPackCategory = category
			break
		}
	}
	for itemDefinition := range is.definitions.IterDefinitions() {
		if itemPackCategory != nil && itemDefinition.GetCategories().ContainsOne(itemPackCategory) {
			continue
		}
		var itemId int32
		i.WithRng(func(rand *i.RandReader) {
			for itemId = rand.Int32(); itemId < 100 || slices.ContainsFunc(items, func(item *MainItem) bool {
				return item.GetId() == itemId
			}); {
			}
		})
		metadata := ItemMetadata{}
		if md := itemDefinition.GetMetadata(); md != nil {
			if attrs := md.attributes; len(attrs) > 0 {
				metadata.attributes = make(map[string]Attribute, len(attrs))
				for k, v := range attrs {
					// sigil_pos is written by the client through updateItemAttributes,
					// so the position declared by the definition is not a valid
					// starting value. is_new is a transient client flag.
					if k == sigilPosAttributeKey || k == isNewAttributeKey {
						continue
					}
					metadata.attributes[k] = v
				}
			}
		}

		// Starting sigil position of the Coat of Arms. updateItemAttributes
		// overwrites it whenever the client changes the position.
		if itemDefinition.GetId() == coatOfArmsDefinitionId {
			metadata.UpdateAttribute(sigilPosAttributeKey, defaultSigilPos)
		}
		metadata.setDefaultEslot(itemDefinition.GetCategories())

		items = append(items, &MainItem{
			id:               itemId,
			metadata:         metadata,
			itemDefinitionID: itemDefinition.GetId(),
			locationID:       defaultItemLocationId(itemDefinition.GetId()),
			durability:       1,
			creationDate:     time.Now().UTC(),
			// Either another item id or location id as defined in statically per game
			tradeID: -1,
			// Max value seen given it's a binary flag
			permission:        63,
			maxChargesPerItem: -1,
		})
	}
	itemsMap := make(map[int32]*MainItem)
	for _, item := range items {
		itemsMap[item.GetId()] = item
	}
	return &itemsMap
}
