package emby

import "testing"

func TestSortBaseItemsAdultDefaultNewestFirst(t *testing.T) {
	items := []baseItem{
		{ID: "old", Name: "Alpha", DateCreated: "2024-01-01T00:00:00.0000000Z"},
		{ID: "new", Name: "Zulu", DateCreated: "2026-09-17T10:00:00.0000000Z"},
		{ID: "mid", Name: "Mike", DateCreated: "2025-06-01T00:00:00.0000000Z"},
	}
	sortBaseItems(items, "", true)
	if items[0].ID != "new" || items[1].ID != "mid" || items[2].ID != "old" {
		t.Fatalf("added desc=%v", idsOf(items))
	}
	sortBaseItems(items, "name-asc", true)
	if items[0].ID != "old" || items[1].ID != "mid" || items[2].ID != "new" {
		t.Fatalf("name asc=%v", idsOf(items))
	}
}

func TestPaginateBaseItems(t *testing.T) {
	items := []baseItem{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	page := paginateBaseItems(items, 1, 2)
	if len(page) != 2 || page[0].ID != "b" || page[1].ID != "c" {
		t.Fatalf("page=%#v", page)
	}
}

func idsOf(items []baseItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
