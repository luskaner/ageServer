package playfab

import (
	"fmt"
	"strings"
	"time"

	"github.com/luskaner/ageServer/server/internal/models/playfab"
)

func addItem(name string, catalogItems map[string]playfab.CatalogItem, inventoryItems *[]playfab.InventoryItem) {
	*inventoryItems = append(
		*inventoryItems,
		playfab.InventoryItem{
			Id:      name,
			StackId: "default",
			Amount:  1,
			Type:    "catalogItem",
		},
	)
	dateFormatted := time.Date(2024, 5, 2, 3, 34, 0, 0, time.UTC).Format(playfab.Iso8601Layout)
	catalogItems[name] =
		playfab.CatalogItem{
			Id:   name,
			Type: "catalogItem",
			AlternateIds: []playfab.CatalogItemAlternativeId{
				{
					"FriendlyId",
					name,
				},
			},
			FriendlyId:       name,
			Title:            playfab.CatalogItemTitle{NEUTRAL: name},
			CreatorEntity:    playfab.CatalogItemCreatorEntity{Id: "C15F9", Type: "title", TypeString: "title"},
			Platforms:        []any{},
			Tags:             []any{},
			CreationDate:     dateFormatted,
			LastModifiedDate: dateFormatted,
			StartDate:        dateFormatted,
			Contents:         []any{},
			Images:           []any{},
			ItemReferences:   []any{},
			DeepLinks:        []any{},
		}
}

func ItemName(category string, effectName string, rarity int) string {
	return fmt.Sprintf("Item_%s_%s_%d", category, effectName, rarity)
}

func Items(blessings []Blessings) (catalogItems map[string]playfab.CatalogItem, inventoryItems []playfab.InventoryItem) {
	catalogItems = make(map[string]playfab.CatalogItem)
	for _, b := range blessings {
		for _, r := range b.KnownRarities {
			if r > -1 {
				addItem(ItemName("Season0", b.EffectName, r), catalogItems, &inventoryItems)
				if strings.HasPrefix(b.EffectName, "GrantLegend") {
					addItem(ItemName("", b.EffectName, r), catalogItems, &inventoryItems)
				}
			}
		}
	}
	return
}
